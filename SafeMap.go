package webtools

import "sync"

// Safe locking map for Go Routines
type SafeMap[K comparable, V any] struct {
	m     map[K]V
	mutex sync.RWMutex
}

// Creates new Safe Map
func MakeSafeMap[K comparable, V any]() SafeMap[K, V] {
	return SafeMap[K, V]{m: map[K]V{}, mutex: sync.RWMutex{}}
}

// Gets safely value from map
func (m *SafeMap[K, V]) Get(key K) V {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.m[key]
}

// Sets safely value to map
func (m *SafeMap[K, V]) Set(key K, value V) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.m[key] = value
}

// Deletes safely value to map
func (m *SafeMap[K, V]) Delete(key K) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.m, key)
}

// Gets keys safely value to map
func (m *SafeMap[K, V]) GetKeys() []K {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result := make([]K, 0)
	for k, _ := range m.m {
		result = append(result, k)
	}
	return result
}

// Gets values safely value to map
func (m *SafeMap[K, V]) GetValues() []V {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result := make([]V, 0)
	for _, v := range m.m {
		result = append(result, v)
	}
	return result
}
