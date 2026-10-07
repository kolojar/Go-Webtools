package helpertools

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
type KeyValuePair[K any, V any] struct {
	Key   K
	Value V
}

// SafeMap provides safe locking map for Go Routines
type SafeMap[K comparable, V any] struct {
	m     map[K]V
	mutex *sync.RWMutex
}

// MakeSafeMap creates new Safe Map
func MakeSafeMap[K comparable, V any](size ...int) SafeMap[K, V] {
	capacity := 0
	if len(size) > 0 {
		capacity = size[0]
	}
	return SafeMap[K, V]{m: make(map[K]V, capacity), mutex: &sync.RWMutex{}}
}

// IsNill checks if map is nil
func (m *SafeMap[K, V]) IsNil() bool {
	return m == nil || m.m == nil
}

// Get gets safely value from map
func (m *SafeMap[K, V]) Get(key K) V {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.m[key]
}

// Has checks if value is in map
func (m *SafeMap[K, V]) Has(key K) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	_, ok := m.m[key]
	return ok
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
func (m *SafeMap[K, V]) GetKeys(deleteListedKeys bool) []K {
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
func (m *SafeMap[K, V]) GetValues(deleteListedValues bool) []V {
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
func (m *SafeMap[K, V]) GetData(deleteListedKeyValues bool) []KeyValuePair[K, V] {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Range map
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

// rangeLocal is local function for range over map. It should not be used externally
func (m *SafeMap[K, V]) rangeLocal(rangeFunc func(key K) (doBreak bool, delete bool)) {
	//Range map
	for k, _ := range m.m {
		//Call rangeFunc
		doBreak, del := rangeFunc(k)

		//Delete if needed
		if del {
			delete(m.m, k)
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
func (m *SafeMap[K, V]) RangeKeys(rangeFunc func(key K) (doBreak bool, delete bool)) {
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
func (m *SafeMap[K, V]) RangeValues(rangeFunc func(value V) (doBreak bool, delete bool)) {
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
func (m *SafeMap[K, V]) RangeData(rangeFunc func(key K, value V) (doBreak bool, delete bool)) {
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
