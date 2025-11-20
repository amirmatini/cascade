package cache

import (
	"container/list"
	"sync"
)

type LRU struct {
	mu        sync.RWMutex
	capacity  int64
	size      int64
	items     map[string]*list.Element
	evictList *list.List
}

type lruEntry struct {
	key  string
	size int64
}

func NewLRU(capacity int64) *LRU {
	return &LRU{
		capacity:  capacity,
		items:     make(map[string]*list.Element, 10000),
		evictList: list.New(),
	}
}

func (l *LRU) Add(key string, size int64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if elem, exists := l.items[key]; exists {
		l.evictList.MoveToFront(elem)
		oldSize := elem.Value.(*lruEntry).size
		l.size = l.size - oldSize + size
		elem.Value.(*lruEntry).size = size
		return
	}

	entry := &lruEntry{key: key, size: size}
	elem := l.evictList.PushFront(entry)
	l.items[key] = elem
	l.size += size
}

func (l *LRU) Get(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if elem, exists := l.items[key]; exists {
		l.evictList.MoveToFront(elem)
		return true
	}
	return false
}

func (l *LRU) Remove(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if elem, exists := l.items[key]; exists {
		l.removeElement(elem)
	}
}

func (l *LRU) removeElement(elem *list.Element) {
	l.evictList.Remove(elem)
	entry := elem.Value.(*lruEntry)
	delete(l.items, entry.key)
	l.size -= entry.size
}

func (l *LRU) GetOldest() (string, int64, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.evictList.Len() == 0 {
		return "", 0, false
	}

	elem := l.evictList.Back()
	entry := elem.Value.(*lruEntry)
	return entry.key, entry.size, true
}

func (l *LRU) Size() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.size
}

func (l *LRU) Capacity() int64 {
	return l.capacity
}

func (l *LRU) NeedsEviction() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.size > l.capacity
}
