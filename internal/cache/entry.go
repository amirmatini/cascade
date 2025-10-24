package cache

import (
	"encoding/json"
	"os"
	"time"
)

type CacheEntry struct {
	Key         string            `json:"key"`
	URL         string            `json:"url"`
	FilePath    string            `json:"file_path"`
	Size        int64             `json:"size"`
	ContentType string            `json:"content_type"`
	Headers     map[string]string `json:"headers"`
	CreatedAt   time.Time         `json:"created_at"`
	AccessedAt  time.Time         `json:"accessed_at"`
	ExpiresAt   time.Time         `json:"expires_at"`
}

func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

func (e *CacheEntry) Save(metaPath string) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, data, 0644)
}

func LoadCacheEntry(metaPath string) (*CacheEntry, error) {
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}
