package webtools

import "sync"

// ThreeValuePair Value pair
type ThreeValuePair[A any, B any, C any] struct {
	A A
	B B
	C C
}

// FiveValuePair Value pair
type FiveValuePair[A any, B any, C any, D any, E any] struct {
	A A
	B B
	C C
	D D
	E E
}

// FourValuePair Value pair
type FourValuePair[A any, B any, C any, D any] struct {
	A A
	B B
	C C
	D D
}

// KeyValuePair Value pair
type KeyValuePair[K comparable, V any] struct {
	Key   K
	Value V
}

// SafeMap provides safe locking map for Go Routines
type SafeMap[K comparable, V any] struct {
	m     map[K]V
	mutex *sync.RWMutex
}

// Has checks if value is in map
func (m SafeMap[K, V]) Has(key K) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	_, ok := m.m[key]
	return ok
}

// MakeSafeMap creates new Safe Map
func MakeSafeMap[K comparable, V any]() SafeMap[K, V] {
	return SafeMap[K, V]{m: map[K]V{}, mutex: &sync.RWMutex{}}
}

// Get gets safely value from map
func (m *SafeMap[K, V]) Get(key K) V {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.m[key]
}

// GetHas gets safely value from map and returns if value is in map
func (m *SafeMap[K, V]) GetHas(key K) (V, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	v, has := m.m[key]
	return v, has
}

// Set sets safely value to map
func (m *SafeMap[K, V]) Set(key K, value V) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.m[key] = value
}

// Delete deletes safely value to map
func (m *SafeMap[K, V]) Delete(key K) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.m, key)
}

// GetKeys gets keys safely value to map
func (m *SafeMap[K, V]) GetKeys() []K {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result := make([]K, 0)
	for k := range m.m {
		result = append(result, k)
	}
	return result
}

// GetValues gets values safely value to map
func (m *SafeMap[K, V]) GetValues() []V {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result := make([]V, 0)
	for _, v := range m.m {
		result = append(result, v)
	}
	return result
}

// Len retuns lenght of map
func (m *SafeMap[K, V]) Len() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.m)
}

// Clear clears map
func (m *SafeMap[K, V]) Clear() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for k := range m.m {
		delete(m.m, k)
	}
}

// GetData gets keys and values safely value to map
func (m *SafeMap[K, V]) GetData() []KeyValuePair[K, V] {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result := make([]KeyValuePair[K, V], 0)
	for k, v := range m.m {
		result = append(result, KeyValuePair[K, V]{Key: k, Value: v})
	}
	return result
}

// SetMutex sets new Mutex
func (m *SafeMap[K, V]) SetMutex(mutex *sync.RWMutex) bool {
	if mutex == nil {
		return false
	}
	m.mutex = mutex
	return true
}

// GetMutex gets Mutex
func (m *SafeMap[K, V]) GetMutex() *sync.RWMutex {
	return m.mutex
}
