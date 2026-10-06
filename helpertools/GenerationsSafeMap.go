package helpertools

import "sync"

// GenerationsSafeMapPreserveLevel is level of automatic preservence in map (sets what triggers presistence in map)
type GenerationsSafeMapPreserveLevel uint8

// NoPreserveGenerationsSafeMapPreserveLevel does not do any automatic preservation
const NoPreserveGenerationsSafeMapPreserveLevel GenerationsSafeMapPreserveLevel = 0

// GetPreserveGenerationsSafeMapPreserveLevel preserves value on Get
const GetPreserveGenerationsSafeMapPreserveLevel GenerationsSafeMapPreserveLevel = 1

// SetPreserveGenerationsSafeMapPreserveLevel preserves value on Set
const SetPreserveGenerationsSafeMapPreserveLevel GenerationsSafeMapPreserveLevel = 2

// GetSetPreserveGenerationsSafeMapPreserveLevel preserves value on Get and on Set
const GetSetPreserveGenerationsSafeMapPreserveLevel GenerationsSafeMapPreserveLevel = 3

// CopyOnSwapPreserveGenerationsSafeMapPreserveLevel copies all values to new generation before removing the old one (overwrites all settings)
//
// Warning: Can cause performance slowdowns
const CopyOnSwapPreserveGenerationsSafeMapPreserveLevel GenerationsSafeMapPreserveLevel = 4

// GenerationsSafeMap provides safe locking map for Go Routines with generation swapping
//
// Warning: Map can delete values if not configured correctly!
type GenerationsSafeMap[K comparable, V any] struct {
	capacity      int
	current       map[K]V
	next          map[K]V
	mutex         *sync.RWMutex
	PresenceLevel GenerationsSafeMapPreserveLevel
}

// MakeGenerationsSafeMap creates new Safe Map with generations (minimum is 2) for better RAM usage. Total usage is: 2 * size (or current len()) * sizePerObject - it is recommended to use pointers
//
// Warning: Map can delete values if not configured correctly!
func MakeGenerationsSafeMap[K comparable, V any](presenceLevel GenerationsSafeMapPreserveLevel, size ...int) GenerationsSafeMap[K, V] {
	//Create map
	result := GenerationsSafeMap[K, V]{mutex: &sync.RWMutex{}, PresenceLevel: presenceLevel}

	//Create capacity
	result.capacity = 0
	if len(size) > 0 {
		result.capacity = size[0]
	}

	//Create sub maps
	result.current = make(map[K]V, result.capacity)
	result.next = make(map[K]V, result.capacity)
	return result
}

// IsNill checks if map is nil
func (m *GenerationsSafeMap[K, V]) IsNil() bool {
	return m == nil || m.current == nil || m.next == nil
}

// Get gets safely value from map
func (m *GenerationsSafeMap[K, V]) Get(key K) V {
	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Check if preserve on GET
	if CheckBinaryContains(m.PresenceLevel, GetPreserveGenerationsSafeMapPreserveLevel) {
		m.next[key] = m.current[key]
	}

	//Return
	return m.current[key]
}

// Has checks if value is in map
func (m *GenerationsSafeMap[K, V]) Has(key K) bool {
	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Check if preserve on GET
	if CheckBinaryContains(m.PresenceLevel, GetPreserveGenerationsSafeMapPreserveLevel) {
		m.next[key] = m.current[key]
	}

	//Return
	_, ok := m.current[key]
	return ok
}

// GetHas gets safely value from map and returns if value is in map
func (m *GenerationsSafeMap[K, V]) GetHas(key K) (V, bool) {
	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Check if preserve on GET
	if CheckBinaryContains(m.PresenceLevel, GetPreserveGenerationsSafeMapPreserveLevel) {
		m.next[key] = m.current[key]
	}

	//Return
	v, has := m.current[key]
	return v, has
}

// Set sets safely value to map
func (m *GenerationsSafeMap[K, V]) Set(key K, value V) {
	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Check if preserve on SET
	if CheckBinaryContains(m.PresenceLevel, SetPreserveGenerationsSafeMapPreserveLevel) {
		m.next[key] = value
	}

	//Set
	m.current[key] = value
}

// Delete deletes safely value to map
func (m *GenerationsSafeMap[K, V]) Delete(key K) {
	//Lock
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	//Check if preserve on SET
	if CheckBinaryContains(m.PresenceLevel, SetPreserveGenerationsSafeMapPreserveLevel) {
		//Should be useless, but for safety
		delete(m.next, key)
	}

	//Delete
	delete(m.current, key)
}

// GetKeys gets keys safely value to map
func (m *GenerationsSafeMap[K, V]) GetKeys(deleteListedKeys bool) []K {
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
	m.rangeLocal(func(key K) (doBreak bool, delete bool) {
		result = append(result, KeyValuePair[K, V]{Key: key, Value: m.m[key]})
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
