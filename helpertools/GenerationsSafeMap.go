package helpertools

import (
	"context"
	"maps"
	"sync"
	"time"
)

// GenerationsSafeMapPreserveLevel is level of automatic preservence in map (sets what triggers presistence in map)
type GenerationsSafeMapPreserveLevel uint8

// NoPreserveGenerationsSafeMapPreserveLevel does not do any automatic preservation
//
// Warning: On generation swap it removes all data
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
	capacity          int
	current           map[K]V
	next              map[K]V
	mutex             *sync.Mutex
	PresenceLevel     GenerationsSafeMapPreserveLevel
	ticker            *time.Ticker
	contextCancelFunc context.CancelFunc
}

// MakeGenerationsSafeMap creates new Safe Map with generations (minimum is 2) for better RAM usage. Total usage is: 2 * size (or current len()) * sizePerObject - it is recommended to use pointers
//
// Warning: Map can delete values if not configured correctly!
func MakeGenerationsSafeMap[K comparable, V any](presenceLevel GenerationsSafeMapPreserveLevel, size ...int) GenerationsSafeMap[K, V] {
	//Create map
	result := GenerationsSafeMap[K, V]{mutex: &sync.Mutex{}, PresenceLevel: presenceLevel, ticker: nil}

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
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Check if preserve on GET
	if CheckBinaryContains(m.PresenceLevel, GetPreserveGenerationsSafeMapPreserveLevel, false) {
		m.next[key] = m.current[key]
	}

	//Return
	return m.current[key]
}

// Has checks if value is in map
func (m *GenerationsSafeMap[K, V]) Has(key K) bool {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Check if preserve on GET
	if CheckBinaryContains(m.PresenceLevel, GetPreserveGenerationsSafeMapPreserveLevel, false) {
		m.next[key] = m.current[key]
	}

	//Return
	_, ok := m.current[key]
	return ok
}

// GetHas gets safely value from map and returns if value is in map
func (m *GenerationsSafeMap[K, V]) GetHas(key K) (V, bool) {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Check if preserve on GET
	if CheckBinaryContains(m.PresenceLevel, GetPreserveGenerationsSafeMapPreserveLevel, false) {
		m.next[key] = m.current[key]
	}

	//Return
	v, has := m.current[key]
	return v, has
}

// Set sets safely value to map
func (m *GenerationsSafeMap[K, V]) Set(key K, value V) {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Check if preserve on SET
	if CheckBinaryContains(m.PresenceLevel, SetPreserveGenerationsSafeMapPreserveLevel, false) {
		m.next[key] = value
	}

	//Check if already exists in next
	_, has := m.next[key]
	if has {
		m.next[key] = value
	}

	//Set
	m.current[key] = value
}

// Delete deletes safely value to map
func (m *GenerationsSafeMap[K, V]) Delete(key K) {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Delete
	delete(m.next, key)
	delete(m.current, key)
}

// GetKeys gets keys safely value to map
func (m *GenerationsSafeMap[K, V]) GetKeys(deleteListedKeys bool) []K {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Range map
	result := make([]K, len(m.current))
	i := 0
	m.rangeLocal(func(key K) (doBreak bool, delete bool) {
		result[i] = key
		i++
		return false, deleteListedKeys
	})
	return result
}

// GetValues gets values safely value to map
func (m *GenerationsSafeMap[K, V]) GetValues(deleteListedValues bool) []V {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Range map
	result := make([]V, len(m.current))
	i := 0
	m.rangeLocal(func(key K) (doBreak bool, delete bool) {
		result[i] = m.current[key]
		i++
		return false, deleteListedValues
	})
	return result
}

// Len retuns lenght of map
func (m *GenerationsSafeMap[K, V]) Len() int {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return len(m.current)
}

// Clear clears map
func (m *GenerationsSafeMap[K, V]) Clear() {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Clear
	clear(m.current)
	clear(m.next)
}

// GetData gets keys and values safely value to map
func (m *GenerationsSafeMap[K, V]) GetData(deleteListedKeyValues bool) []KeyValuePair[K, V] {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Range map
	result := make([]KeyValuePair[K, V], len(m.current))
	i := 0
	m.rangeLocal(func(key K) (doBreak bool, delete bool) {
		result[i] = KeyValuePair[K, V]{Key: key, Value: m.current[key]}
		i++
		return false, deleteListedKeyValues
	})
	return result
}

// SetMutex sets new Mutex
func (m *GenerationsSafeMap[K, V]) SetMutex(mutex *sync.Mutex) bool {
	if mutex == nil {
		return false
	}
	m.mutex = mutex
	return true
}

// GetMutex gets Mutex
func (m *GenerationsSafeMap[K, V]) GetMutex() *sync.Mutex {
	return m.mutex
}

// rangeLocal is local function for range over map. It should not be used externally
func (m *GenerationsSafeMap[K, V]) rangeLocal(rangeFunc func(key K) (doBreak bool, delete bool)) {
	//Range map
	for k, v := range m.current {
		//Check if preserve on GET
		if CheckBinaryContains(m.PresenceLevel, GetPreserveGenerationsSafeMapPreserveLevel, false) {
			m.next[k] = v
		}

		//Call rangeFunc
		doBreak, del := rangeFunc(k)

		//Delete if needed
		if del {
			//Delete
			delete(m.next, k)
			delete(m.current, k)
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
func (m *GenerationsSafeMap[K, V]) RangeKeys(rangeFunc func(key K) (doBreak bool, delete bool)) {
	//Check if can run
	if rangeFunc == nil {
		return
	}

	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Range
	m.rangeLocal(rangeFunc)
}

// RangeValues iterates trought map values without creating slice
//
// Map is locked, so no write operations involving the map should be called in rangeFunc
func (m *GenerationsSafeMap[K, V]) RangeValues(rangeFunc func(value V) (doBreak bool, delete bool)) {
	//Check if can run
	if rangeFunc == nil {
		return
	}

	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Range
	m.rangeLocal(func(key K) (doBreak bool, delete bool) {
		return rangeFunc(m.current[key])
	})
}

// RangeData iterates trought map keys and values without creating slice
//
// Map is locked, so no write operations involving the map should be called in rangeFunc
func (m *GenerationsSafeMap[K, V]) RangeData(rangeFunc func(key K, value V) (doBreak bool, delete bool)) {
	//Check if can run
	if rangeFunc == nil {
		return
	}

	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Range
	m.rangeLocal(func(key K) (doBreak bool, delete bool) {
		return rangeFunc(key, m.current[key])
	})
}

// StartGenerationTimer stars generation timer (generation switching and map sweeping). Does not lock exection thread
//
// Warning: If map is configured wrongly it can cause loose of data
func (m *GenerationsSafeMap[K, V]) StartGenerationTimer(interval time.Duration) bool {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Check if can start new
	if m.ticker != nil {
		return false
	}

	//Start timer
	m.ticker = time.NewTicker(interval)
	var ctx context.Context
	ctx, m.contextCancelFunc = context.WithCancel(context.Background())
	defer m.ticker.Stop()

	//Run loop
	go func() {
		for {
			select {
			case <-m.ticker.C:
				m.NewGeneration()
			case <-ctx.Done():
				return
			}
		}
	}()
	return true
}

// StopGenerationTimer stops generation timer
func (m *GenerationsSafeMap[K, V]) StopGenerationTimer() {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Check if can stop
	if m.ticker != nil {
		m.ticker.Stop()
		m.ticker = nil
	}

	//Stop func
	if m.contextCancelFunc != nil {
		m.contextCancelFunc()
	}
}

// NewGeneration switches maps to new generation
func (m *GenerationsSafeMap[K, V]) NewGeneration() {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Copy when needed
	if CheckBinaryContains(m.PresenceLevel, CopyOnSwapPreserveGenerationsSafeMapPreserveLevel, false) {
		clear(m.next)
		maps.Copy(m.next, m.current)
	}

	//Swap
	m.current = m.next
	m.next = make(map[K]V, m.capacity)
}
