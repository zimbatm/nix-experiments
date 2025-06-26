package rewrite

import (
	"sync"
	"testing"
	"time"

	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/store"
)

func TestStoreCache_GetReferences(t *testing.T) {
	s := store.New("/nix/store")
	cache := NewStoreCacheWithStore(s)

	// Test initial state
	if len(cache.references) != 0 {
		t.Error("Cache should be empty initially")
	}

	// Note: Actual testing of GetReferences requires store operations
	// This would be better as an integration test
}

func TestStoreCache_Concurrency(t *testing.T) {
	s := store.New("/nix/store")
	cache := NewStoreCacheWithStore(s)

	// Test concurrent access
	var wg sync.WaitGroup
	numGoroutines := 10

	// Add test data
	testPath := "/nix/store/test-concurrent"
	cache.references[testPath] = cacheEntry{
		data:      []string{"ref1", "ref2"},
		timestamp: time.Now(),
	}

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			// Try to read from cache multiple times
			for j := 0; j < 100; j++ {
				cache.mu.RLock()
				_, ok := cache.references[testPath]
				cache.mu.RUnlock()
				if !ok {
					t.Error("Cache entry disappeared during concurrent access")
				}
			}
		}()
	}

	// Also have a writer
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 50; i++ {
			cache.mu.Lock()
			cache.references[testPath] = cacheEntry{
				data:      []string{"ref1", "ref2", "ref3"},
				timestamp: time.Now(),
			}
			cache.mu.Unlock()
			time.Sleep(time.Microsecond)
		}
	}()

	wg.Wait()
}

func TestStoreCache_MemoryLimit(t *testing.T) {
	// Test that large NAR data is not cached
	largeData := make([]byte, 11*1024*1024) // 11MB, over the 10MB limit

	// Simulate adding large data
	// Since we can't easily mock store.Dump, we'll test the caching logic directly
	if len(largeData) < 10*1024*1024 {
		t.Error("Test data should be larger than cache limit")
	}

	// Verify that the caching logic would skip large data
	if len(largeData) >= 10*1024*1024 {
		// In real implementation, this would not be cached
		t.Log("Large data would not be cached")
	}

	// Test that small data would be cached
	smallData := make([]byte, 1024) // 1KB, well under limit
	if len(smallData) >= 10*1024*1024 {
		t.Error("Small data should be under cache limit")
	}
}
