package fulfillment

import "sync"

type syncMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

func newSyncMap[K comparable, V any]() *syncMap[K, V] {
	return &syncMap[K, V]{m: make(map[K]V)}
}

func (sm *syncMap[K, V]) Store(key K, value V) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m[key] = value
}

func (sm *syncMap[K, V]) Delete(key K) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.m, key)
}

func (sm *syncMap[K, V]) Values() []V {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	values := make([]V, 0, len(sm.m))
	for _, v := range sm.m {
		values = append(values, v)
	}
	return values
}

func chunk[S ~[]E, E any](slice S, size int) []S {
	if len(slice) == 0 {
		return nil
	}
	chunks := make([]S, 0, (len(slice)+size-1)/size)
	for i := 0; i < len(slice); i += size {
		end := min(i+size, len(slice))
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}
