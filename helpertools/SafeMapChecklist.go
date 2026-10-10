package helpertools

import "sync"

// SafeMap provides safe locking map for checklist or presence checking for Go Routines
type SafeMapChecklist[K comparable] struct {
	m     map[K]struct{}
	mutex *sync.RWMutex
}

// MakeSafeMapChecklist creates new Safe Map Checklist
func MakeSafeMapChecklist[K comparable](size ...int) SafeMapChecklist[K] {
	capacity := 0
	if len(size) > 0 {
		capacity = size[0]
	}
	return SafeMapChecklist[K]{m: make(map[K]struct{}, capacity), mutex: &sync.RWMutex{}}
}

// IsNill checks if map is nil
func (m *SafeMapChecklist[K]) IsNil() bool {
	return m == nil || m.m == nil
}

// Has checks if value is in map
func (m *SafeMapChecklist[K]) Has(key K) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	_, ok := m.m[key]
	return ok
}

// Add adds key to map
func (m *SafeMapChecklist[K]) Add(key K) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.m[key] = struct{}{}
}

// Remove removes key to map
func (m *SafeMapChecklist[K]) Remove(key K) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.m, key)
}

// AddOnce adds key to map only if it is not present (returns true on success)
func (m *SafeMapChecklist[K]) AddOnce(key K) bool {
	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Check if exitst
	_, ok := m.m[key]
	if ok {
		return false
	}

	//Add
	m.m[key] = struct{}{}
	return true
}

// Range ranges map
func (m *SafeMapChecklist[K]) Range(rangeFunc func(key K) (doBreak bool, delete bool, err error)) error {
	//Check if can run
	if rangeFunc == nil {
		return nil
	}

	//Lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	//Range
	for k, _ := range m.m {
		//Call range
		doBreak, del, err := rangeFunc(k)
		if err != nil {
			return err
		}

		//Delete if needed
		if del {
			delete(m.m, k)
		}

		//Break if needed
		if doBreak {
			break
		}
	}
	return nil
}
