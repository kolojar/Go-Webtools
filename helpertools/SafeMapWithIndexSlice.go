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
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Check if exist
	_, ok := m.m[key]
	if !ok {
		m.s = append(m.s, key)
	}

	//Set value
	m.m[key] = value
}

// Delete deletes safely value to map
func (m *SafeMapWithIndexSlice[K, V]) Delete(key K) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.m, key)
	m.s = RemoveElement(m.s, key)
}

// GetKeys gets keys safely value to map
func (m *SafeMapWithIndexSlice[K, V]) GetKeys(deleteListedKeys bool) []K {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Range map
	result := make([]K, len(m.m))
	m.rangeLocal(func(key K) (doBreak bool, delete bool) {
		result = append(result, key)
		return false, deleteListedKeys
	})
	return result
}

// GetValues gets values safely value to map
func (m *SafeMapWithIndexSlice[K, V]) GetValues(deleteListedValues bool) []V {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Range map
	result := make([]V, len(m.m))
	m.rangeLocal(func(key K) (doBreak bool, delete bool) {
		result = append(result, m.m[key])
		return false, deleteListedValues
	})
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
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Clear
	clear(m.m)
	clear(m.s)
}

// GetData gets keys and values safely value to map
func (m *SafeMapWithIndexSlice[K, V]) GetData(deleteListedKeyValues bool) []KeyValuePair[K, V] {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result := make([]KeyValuePair[K, V], len(m.m))
	i := 0
	m.rangeLocal(func(key K) (doBreak bool, delete bool) {
		result[i] = KeyValuePair[K, V]{Key: key, Value: m.m[key]}
		i++
		return false, deleteListedKeyValues
	})
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

// rangeLocal is local function for range over map. It should not be used externally
func (m *SafeMapWithIndexSlice[K, V]) rangeLocal(rangeFunc func(key K) (doBreak bool, delete bool)) {
	//Range map
	for k, _ := range m.m {
		//Call rangeFunc
		doBreak, del := rangeFunc(k)

		//Delete if needed
		if del {
			delete(m.m, k)
			m.s = RemoveElement(m.s, k)
		}

		//Break if needed
		if doBreak {
			break
		}
	}
}

// RangeKeys iterates trought map keys without creating slice
//
// Map is locked, so no write operations involving the map should be called in rangeFunc
func (m *SafeMapWithIndexSlice[K, V]) RangeKeys(rangeFunc func(key K) (doBreak bool, delete bool)) {
	//Check if can run
	if rangeFunc == nil {
		return
	}

	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Range
	m.rangeLocal(rangeFunc)
}

// RangeValues iterates trought map values without creating slice
//
// Map is locked, so no write operations involving the map should be called in rangeFunc
func (m *SafeMapWithIndexSlice[K, V]) RangeValues(rangeFunc func(value V) (doBreak bool, delete bool)) {
	//Check if can run
	if rangeFunc == nil {
		return
	}

	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Range
	m.rangeLocal(func(key K) (doBreak bool, delete bool) {
		return rangeFunc(m.m[key])
	})
}

// RangeData iterates trought map keys and values without creating slice
//
// Map is locked, so no write operations involving the map should be called in rangeFunc
func (m *SafeMapWithIndexSlice[K, V]) RangeData(rangeFunc func(key K, value V) (doBreak bool, delete bool)) {
	//Check if can run
	if rangeFunc == nil {
		return
	}

	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Range
	m.rangeLocal(func(key K) (doBreak bool, delete bool) {
		return rangeFunc(key, m.m[key])
	})
}

// getRandomKeysLocal should not be called externally, helper function, gets random keys based on limit
func (m SafeMapWithIndexSlice[K, V]) getRandomKeysLocal(limit uint, callback func(key K) (doBreak bool, delete bool)) {
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
		key := keysLocal[index]

		//Call callback
		doBreak, del := callback(key)

		//Delete if needed
		if del {
			delete(m.m, key)
			m.s = RemoveElement(m.s, key)
		}

		//Break if needed
		if doBreak {
			break
		}

		//Remove for loop
		keysLocal = RemoveElementAtIndex(keysLocal, index)
	}
}

// GetRandomKeys gets random list of keys. Set delete to true when you want to delete key after usage
func (m SafeMapWithIndexSlice[K, V]) GetRandomKeys(limit uint, delete bool) []K {
	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Get keys
	result := make([]K, limit)
	m.getRandomKeysLocal(limit, func(key K) (doBreak bool, _ bool) {
		result = append(result, key)
		return false, delete
	})
	return result
}

// GetRandomValues gets random list of values. Set delete to true when you want to delete value after usage
func (m SafeMapWithIndexSlice[K, V]) GetRandomValues(limit uint, delete bool) []V {
	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Get values
	result := make([]V, limit)
	m.getRandomKeysLocal(limit, func(key K) (doBreak bool, _ bool) {
		result = append(result, m.m[key])
		return false, delete
	})
	return result
}

// GetRandomData gets random list of key-values. Set delete to true when you want to delete key-value after usage
func (m SafeMapWithIndexSlice[K, V]) GetRandomData(limit uint, delete bool) []KeyValuePair[K, V] {
	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Get values
	result := make([]KeyValuePair[K, V], limit)
	m.getRandomKeysLocal(limit, func(key K) (doBreak bool, _ bool) {
		result = append(result, KeyValuePair[K, V]{Key: key, Value: m.m[key]})
		return false, delete
	})
	return result
}

// GetRandomKeys gets random list of keys
//
// Map is locked, so no operations involving the map should be called in rangeFunc
func (m SafeMapWithIndexSlice[K, V]) RangeRandomKeys(limit uint, rangeFunc func(key K) (doBreak bool, delete bool)) {
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
func (m SafeMapWithIndexSlice[K, V]) RangeRandomValues(limit uint, rangeFunc func(value V) (doBreak bool, delete bool)) {
	//Check if rangeFunc valid
	if rangeFunc == nil {
		return
	}

	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Get values
	m.getRandomKeysLocal(limit, func(key K) (doBreak bool, delete bool) {
		return rangeFunc(m.m[key])
	})
}

// GetRandomData gets random list of key-values
//
// Map is locked, so no operations involving the map should be called in rangeFunc
func (m SafeMapWithIndexSlice[K, V]) RangeRandomData(limit uint, rangeFunc func(key K, value V) (doBreak bool, delete bool)) {
	//Check if rangeFunc valid
	if rangeFunc == nil {
		return
	}

	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Get values
	m.getRandomKeysLocal(limit, func(key K) (doBreak bool, delete bool) {
		return rangeFunc(key, m.m[key])
	})
}
