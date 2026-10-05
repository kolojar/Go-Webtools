package helpertools

import (
	"math/rand/v2"
	"sync"
)

// SafeMapWithIndexSlice provides safe locking map for Go Routines with index slice as helper
type SafeMapWithIndexSlice[K comparable, V any] struct {
	m     map[K]V
	s     []K
	mutex *sync.RWMutex
}

// MakeSafeMapWithIndexSlice creates new Safe Map with index slice
func MakeSafeMapWithIndexSlice[K comparable, V any](size ...int) SafeMapWithIndexSlice[K, V] {
	capacity := 0
	if len(size) > 0 {
		capacity = size[0]
	}
	return SafeMapWithIndexSlice[K, V]{m: make(map[K]V, capacity), s: make([]K, capacity), mutex: &sync.RWMutex{}}
}

// IsNill checks if map is nil
func (m *SafeMapWithIndexSlice[K, V]) IsNil() bool {
	return m == nil || m.m == nil
}

// Get gets safely value from map
func (m *SafeMapWithIndexSlice[K, V]) Get(key K) V {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.m[key]
}

// Has checks if value is in map
func (m *SafeMapWithIndexSlice[K, V]) Has(key K) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	_, ok := m.m[key]
	return ok
}

// GetHas gets safely value from map and returns if value is in map
func (m *SafeMapWithIndexSlice[K, V]) GetHas(key K) (V, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	v, has := m.m[key]
	return v, has
}

// Set sets safely value to map
func (m *SafeMapWithIndexSlice[K, V]) Set(key K, value V) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.m[key] = value
}

// Delete deletes safely value to map
func (m *SafeMapWithIndexSlice[K, V]) Delete(key K) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.m, key)
}

// GetKeys gets keys safely value to map
func (m *SafeMapWithIndexSlice[K, V]) GetKeys() []K {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result := make([]K, 0)
	for k := range m.m {
		result = append(result, k)
	}
	return result
}

// GetValues gets values safely value to map
func (m *SafeMapWithIndexSlice[K, V]) GetValues() []V {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result := make([]V, 0)
	for _, v := range m.m {
		result = append(result, v)
	}
	return result
}

// Len retuns lenght of map
func (m *SafeMapWithIndexSlice[K, V]) Len() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.m)
}

// Clear clears map
func (m *SafeMapWithIndexSlice[K, V]) Clear() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for k := range m.m {
		delete(m.m, k)
	}
}

// GetData gets keys and values safely value to map
func (m *SafeMapWithIndexSlice[K, V]) GetData() []KeyValuePair[K, V] {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result := make([]KeyValuePair[K, V], 0)
	for k, v := range m.m {
		result = append(result, KeyValuePair[K, V]{Key: k, Value: v})
	}
	return result
}

// SetMutex sets new Mutex
func (m *SafeMapWithIndexSlice[K, V]) SetMutex(mutex *sync.RWMutex) bool {
	if mutex == nil {
		return false
	}
	m.mutex = mutex
	return true
}

// GetMutex gets Mutex
func (m *SafeMapWithIndexSlice[K, V]) GetMutex() *sync.RWMutex {
	return m.mutex
}

// Range iterates trought map keys and values without creating slice
//
// Map is locked, so no write operations involving the map should be called in rangeFunc
func (m *SafeMapWithIndexSlice[K, V]) Range(rangeFunc func(key K, value V) (doBreak bool)) {
	if rangeFunc == nil {
		return
	}
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	for k, v := range m.m {
		if rangeFunc(k, v) {
			break
		}
	}
}

// RangeWithEmpty iterates trought map keys and values without creating slice but it removes values (goes from end).
//
// Map is locked, so no operations involving the map should be called in rangeFunc
func (m *SafeMapWithIndexSlice[K, V]) RangeWithEmpty(rangeFunc func(key K, value V) (doBreak bool)) {
	if rangeFunc == nil {
		return
	}
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	for k, v := range m.m {
		delete(m.m, k)
		if rangeFunc(k, v) {
			break
		}
	}
}

// getRandomKeysLocal should not be called externally, helper function, gets random keys based on limit
func (m SafeMapWithIndexSlice[K, V]) getRandomKeysLocal(limit uint, fallback func(key K)) {
	//Copy keys
	keysLocal := make([]K, len(m.s))
	copy(keysLocal, m.s)

	//Get random keys
	for i := uint(0); i < limit; i++ {
		//Check if can choose
		if len(keysLocal) == 0 {
			break
		}

		//Get random
		index := rand.IntN(len(keysLocal))
		fallback(keysLocal[index])
		keysLocal = RemoveElementAtIndex(keysLocal, index)
	}
}

// GetRandomKeys gets random list of keys
func (m SafeMapWithIndexSlice[K, V]) GetRandomKeys(limit uint) []K {
	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Get keys
	result := make([]K, limit)
	m.getRandomKeysLocal(limit, func(key K) {
		result = append(result, key)
	})
	return result
}

// GetRandomValues gets random list of values
func (m SafeMapWithIndexSlice[K, V]) GetRandomValues(limit uint) []V {
	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Get values
	result := make([]V, limit)
	m.getRandomKeysLocal(limit, func(key K) {
		result = append(result, m.m[key])
	})
	return result
}

// GetRandomData gets random list of key-values
func (m SafeMapWithIndexSlice[K, V]) GetRandomData(limit uint) []KeyValuePair[K, V] {
	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Get values
	result := make([]KeyValuePair[K, V], limit)
	m.getRandomKeysLocal(limit, func(key K) {
		result = append(result, KeyValuePair[K, V]{Key: key, Value: m.m[key]})
	})
	return result
}

// GetRandomKeys gets random list of keys
//
// Map is locked, so no operations involving the map should be called in rangeFunc
func (m SafeMapWithIndexSlice[K, V]) RangeRandomKeys(limit uint, rangeFunc func(key K)) {
	//Check if rangeFunc valid
	if rangeFunc == nil {
		return
	}

	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Get keys
	m.getRandomKeysLocal(limit, rangeFunc)
}

// GetRandomValues gets random list of values
//
// Map is locked, so no operations involving the map should be called in rangeFunc
func (m SafeMapWithIndexSlice[K, V]) RangeRandomValues(limit uint, rangeFunc func(value V)) {
	//Check if rangeFunc valid
	if rangeFunc == nil {
		return
	}

	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Get values
	m.getRandomKeysLocal(limit, func(key K) {
		rangeFunc(m.m[key])
	})
}

// GetRandomData gets random list of key-values
//
// Map is locked, so no operations involving the map should be called in rangeFunc
func (m SafeMapWithIndexSlice[K, V]) RangeRandomData(limit uint, rangeFunc func(key K, value V)) {
	//Check if rangeFunc valid
	if rangeFunc == nil {
		return
	}

	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Get values
	m.getRandomKeysLocal(limit, func(key K) {
		rangeFunc(key, m.m[key])
	})
}
