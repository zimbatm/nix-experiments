// Package rewrite - cache.go implements caching for store operations
package rewrite

import (
	"sync"
	"time"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/constants"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/store"
)

// StoreCache caches expensive store operations
type StoreCache struct {
	references map[string]cacheEntry
	narData    map[string]cacheEntry
	mu         sync.RWMutex
	store      *store.Store
}

type cacheEntry struct {
	data      any
	timestamp time.Time
}

// NewStoreCacheWithStore creates a new cache instance with a specific store
func NewStoreCacheWithStore(s *store.Store) *StoreCache {
	return &StoreCache{
		references: make(map[string]cacheEntry),
		narData:    make(map[string]cacheEntry),
		store:      s,
	}
}

// GetReferences returns cached references or queries the store
func (c *StoreCache) GetReferences(path string) ([]string, error) {
	c.mu.RLock()
	if entry, ok := c.references[path]; ok {
		c.mu.RUnlock()
		return entry.data.([]string), nil
	}
	c.mu.RUnlock()

	// Not in cache, query store
	refs, err := c.store.QueryReferences(path)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.mu.Lock()
	c.references[path] = cacheEntry{
		data:      refs,
		timestamp: time.Now(),
	}
	c.mu.Unlock()

	return refs, nil
}

// GetNARData returns cached NAR data or generates it
func (c *StoreCache) GetNARData(path string) ([]byte, error) {
	c.mu.RLock()
	if entry, ok := c.narData[path]; ok {
		c.mu.RUnlock()
		return entry.data.([]byte), nil
	}
	c.mu.RUnlock()

	// Not in cache, generate NAR
	data, err := c.store.Dump(path)
	if err != nil {
		return nil, err
	}

	// Cache the result (be careful with memory usage)
	if len(data) < constants.MaxCacheSize { // Only cache if < MaxCacheSize
		c.mu.Lock()
		c.narData[path] = cacheEntry{
			data:      data,
			timestamp: time.Now(),
		}
		c.mu.Unlock()
	}

	return data, nil
}

// Clear removes all cached data
func (c *StoreCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.references = make(map[string]cacheEntry)
	c.narData = make(map[string]cacheEntry)
}
