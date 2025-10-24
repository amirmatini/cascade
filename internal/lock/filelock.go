package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

type FileLock struct {
	mu    sync.Mutex
	locks map[string]*lockEntry
}

type lockEntry struct {
	mu     sync.Mutex
	file   *os.File
	refcnt int
}

func NewFileLock() *FileLock {
	return &FileLock{
		locks: make(map[string]*lockEntry),
	}
}

func (fl *FileLock) Lock(path string) (func(), error) {
	fl.mu.Lock()
	entry, exists := fl.locks[path]
	if !exists {
		entry = &lockEntry{}
		fl.locks[path] = entry
	}
	entry.refcnt++
	fl.mu.Unlock()

	entry.mu.Lock()

	if entry.file == nil {
		lockPath := path + ".lock"
		if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
			entry.mu.Unlock()
			return nil, fmt.Errorf("failed to create lock directory: %w", err)
		}

		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			entry.mu.Unlock()
			return nil, fmt.Errorf("failed to create lock file: %w", err)
		}

		if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
			file.Close()
			entry.mu.Unlock()
			return nil, fmt.Errorf("failed to acquire file lock: %w", err)
		}

		entry.file = file
	}

	unlockFn := func() {
		if entry.file != nil {
			syscall.Flock(int(entry.file.Fd()), syscall.LOCK_UN)
			entry.file.Close()
			os.Remove(entry.file.Name())
			entry.file = nil
		}
		entry.mu.Unlock()

		fl.mu.Lock()
		entry.refcnt--
		if entry.refcnt == 0 {
			delete(fl.locks, path)
		}
		fl.mu.Unlock()
	}

	return unlockFn, nil
}
