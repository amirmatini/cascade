package cache

import (
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cascade/internal/lock"
)

type Storage struct {
	baseDir     string
	lru         *LRU
	fileLock    *lock.FileLock
	mu          sync.RWMutex
	bufferSize  int
	minFileSize int64
	maxFileSize int64
}

func NewStorage(baseDir string, maxSizeBytes int64, bufferSizeKB int, minFileSizeKB, maxFileSizeMB int64) (*Storage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	s := &Storage{
		baseDir:     baseDir,
		lru:         NewLRU(maxSizeBytes),
		fileLock:    lock.NewFileLock(),
		bufferSize:  bufferSizeKB * 1024,
		minFileSize: minFileSizeKB * 1024,
		maxFileSize: maxFileSizeMB * 1024 * 1024,
	}

	if err := s.loadExistingCache(); err != nil {
		return nil, fmt.Errorf("failed to load existing cache: %w", err)
	}

	return s, nil
}

func (s *Storage) loadExistingCache() error {
	return filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".meta") {
			entry, err := LoadCacheEntry(path)
			if err != nil {
				return nil
			}

			if entry.IsExpired() {
				s.deleteEntry(entry)
				return nil
			}

			s.lru.Add(entry.Key, entry.Size)
		}

		return nil
	})
}

func (s *Storage) generateKey(url string) string {
	h := fnv.New128a()
	h.Write([]byte(url))
	sum := h.Sum(nil)
	result := make([]byte, hex.EncodedLen(len(sum)))
	hex.Encode(result, sum)
	return string(result)
}

func (s *Storage) getFilePath(key string) (string, string) {
	prefix := key[:2]
	dir := filepath.Join(s.baseDir, prefix)
	dataPath := filepath.Join(dir, key+".data")
	metaPath := filepath.Join(dir, key+".meta")
	return dataPath, metaPath
}

func (s *Storage) Get(url string) (*CacheEntry, io.ReadCloser, error) {
	key := s.generateKey(url)
	dataPath, metaPath := s.getFilePath(key)

	unlock, err := s.fileLock.Lock(dataPath)
	if err != nil {
		return nil, nil, err
	}

	entry, err := LoadCacheEntry(metaPath)
	if err != nil {
		unlock()
		return nil, nil, err
	}

	if entry.IsExpired() {
		unlock()
		s.Delete(url)
		return nil, nil, fmt.Errorf("cache entry expired")
	}

	file, err := os.Open(dataPath)
	if err != nil {
		unlock()
		return nil, nil, err
	}

	s.lru.Get(key)
	entry.AccessedAt = time.Now()
	entry.Save(metaPath)

	reader := &lockedReader{
		ReadCloser: file,
		unlock:     unlock,
	}

	return entry, reader, nil
}

type lockedReader struct {
	io.ReadCloser
	unlock func()
}

func (lr *lockedReader) Close() error {
	err := lr.ReadCloser.Close()
	lr.unlock()
	return err
}

func (s *Storage) Put(url string, contentType string, headers map[string]string, ttl time.Duration, reader io.Reader, expectedSize int64) error {
	key := s.generateKey(url)
	dataPath, metaPath := s.getFilePath(key)

	if err := os.MkdirAll(filepath.Dir(dataPath), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	unlock, err := s.fileLock.Lock(dataPath)
	if err != nil {
		return err
	}
	defer unlock()

	tempPath := dataPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempPath)

	buffer := make([]byte, s.bufferSize)
	written, err := io.CopyBuffer(tempFile, reader, buffer)
	if err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write cache data: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if written == 0 {
		return fmt.Errorf("refusing to cache empty file (0 bytes)")
	}

	if expectedSize > 0 && written != expectedSize {
		return fmt.Errorf("incomplete download: got %d bytes, expected %d bytes", written, expectedSize)
	}

	if written < s.minFileSize {
		return fmt.Errorf("file too small to cache: %d bytes (min: %d bytes)", written, s.minFileSize)
	}

	if written > s.maxFileSize {
		return fmt.Errorf("file too large to cache: %d bytes (max: %d bytes)", written, s.maxFileSize)
	}

	s.evictIfNeeded(written)

	if err := os.Rename(tempPath, dataPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	entry := &CacheEntry{
		Key:         key,
		URL:         url,
		FilePath:    dataPath,
		Size:        written,
		ContentType: contentType,
		Headers:     headers,
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		ExpiresAt:   time.Now().Add(ttl),
	}

	if err := entry.Save(metaPath); err != nil {
		os.Remove(dataPath)
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	s.lru.Add(key, written)

	return nil
}

func (s *Storage) evictIfNeeded(newSize int64) {
	for s.lru.Size()+newSize > s.lru.Capacity() {
		key, _, ok := s.lru.GetOldest()
		if !ok {
			break
		}

		_, metaPath := s.getFilePath(key)
		entry, err := LoadCacheEntry(metaPath)
		if err == nil {
			s.deleteEntry(entry)
		}
		s.lru.Remove(key)
	}
}

func (s *Storage) Delete(url string) error {
	key := s.generateKey(url)
	dataPath, metaPath := s.getFilePath(key)

	unlock, err := s.fileLock.Lock(dataPath)
	if err != nil {
		return err
	}
	defer unlock()

	os.Remove(dataPath)
	os.Remove(metaPath)
	s.lru.Remove(key)

	return nil
}

func (s *Storage) deleteEntry(entry *CacheEntry) {
	os.Remove(entry.FilePath)
	metaPath := strings.TrimSuffix(entry.FilePath, ".data") + ".meta"
	os.Remove(metaPath)
}

func (s *Storage) GetStats() (int64, int64, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lru.Size(), s.lru.Capacity(), len(s.lru.items)
}
