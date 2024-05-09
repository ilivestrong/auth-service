package internal

import "sync"

type (
	Cache interface {
		Get(key string) bool
		Set(key string)
		Remove(key string)
	}

	inMemoryCache struct {
		mu   sync.Mutex
		data map[string]struct{}
	}
)

func (memCache *inMemoryCache) Get(key string) bool {
	memCache.mu.Lock()
	defer memCache.mu.Unlock()

	_, exists := memCache.data[key]
	return exists
}

func (memCache *inMemoryCache) Set(key string) {
	memCache.mu.Lock()
	defer memCache.mu.Unlock()

	memCache.data[key] = struct{}{}
}

func (memCache *inMemoryCache) Remove(key string) {
	memCache.mu.Lock()
	defer memCache.mu.Unlock()

	delete(memCache.data, key)
}

func NewInMemoryCache() Cache {
	return &inMemoryCache{data: make(map[string]struct{})}
}
