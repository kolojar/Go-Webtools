package webtools

import "sync"

// Value pair
type ThreeValuePair[A any, B any, C any] struct {
	A A
	B B
	C C
}

// Value pair
type FiveValuePair[A any, B any, C any, D any, E any] struct {
	A A
	B B
	C C
	D D
	E E
}

// Value pair
type KeyValuePair[K comparable, V any] struct {
	Key   K
	Value V
}

// Safe locking map for Go Routines
type SafeMap[K comparable, V any] struct {
	m     map[K]V
	mutex *sync.RWMutex
}

// Check if value is in map
func (m SafeMap[K, V]) Has(key K) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	_, ok := m.m[key]
	return ok
}

// Creates new Safe Map
func MakeSafeMap[K comparable, V any]() SafeMap[K, V] {
	return SafeMap[K, V]{m: map[K]V{}, mutex: &sync.RWMutex{}}
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
	for k := range m.m {
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

// Retuns lenght of map
func (m *SafeMap[K, V]) Len() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.m)
}

// Gets keys and values safely value to map
func (m *SafeMap[K, V]) GetData() []KeyValuePair[K, V] {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result := make([]KeyValuePair[K, V], 0)
	for k, v := range m.m {
		result = append(result, KeyValuePair[K, V]{Key: k, Value: v})
	}
	return result
}

// Sets new Mutex
func (m *SafeMap[K, V]) SetMutex(mutex *sync.RWMutex) bool {
	if mutex == nil {
		return false
	} else {
		m.mutex = mutex
		return true
	}
}

// Gets Mutex
func (m *SafeMap[K, V]) GetMutex(mutex *sync.RWMutex) *sync.RWMutex {
	return m.mutex
}
