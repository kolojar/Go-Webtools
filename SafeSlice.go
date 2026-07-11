package webtools

import (
	"slices"
	"sync"
)

// SafeSlice provides safe locking slice for Go Routines
type SafeSlice[V any] struct {
	s     []V
	mutex *sync.RWMutex
}

// MakeSafeMap creates new Safe Map
func MakeSafeSlice[V any]() SafeSlice[V] {
	return SafeSlice[V]{s: make([]V, 0), mutex: &sync.RWMutex{}}
}

// IsNill checks if slice is nil
func (s *SafeSlice[V]) IsNil() bool {
	return s == nil || s.s == nil
}

// PopFirst gets safely first value from slice and removes it
func (s *SafeSlice[V]) PopFirst() V {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	v := s.s[0]
	s.s = s.s[1:]
	return v
}

// PopLast gets safely last value from slice and removes it
func (s *SafeSlice[V]) PopLast() V {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	v := s.s[len(s.s)-1]
	s.s = s.s[:len(s.s)-1]
	return v
}

// Append appends safely value to slice
func (s *SafeSlice[V]) Append(value V) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.s = append(s.s, value)
}

// Prepend prepends (inserts at first index) safely value to slice
func (s *SafeSlice[V]) Prepend(value V) {
	s.InsertAt(0, value)
}

// Insert inserts at specified index safely value to slice
func (s *SafeSlice[V]) InsertAt(index int, value V) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.s = InsertElementAtIndex(s.s, index, value)
}

// RemoveAt removes value at index safely from slice
func (s *SafeSlice[V]) RemoveAt(index int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.s = RemoveElementAtIndex(s.s, index)
}

// RemoveAt removes value at index safely from slice
func (s *SafeSlice[V]) Remove(deleteFunction func(v V) bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.s = slices.DeleteFunc(s.s,deleteFunction)
}

// SetAt sets value safely at slice
func (s *SafeSlice[V]) SetAt(index int, value V) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if index < len(s.s) {
		s.s[index] = value
	}
}

// GetValues gets values safely value from slice
func (s *SafeSlice[V]) GetValues() []V {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.getValues()
}

func (s *SafeSlice[V]) getValues() []V {
	values := make([]V, len(s.s))
	copy(values, s.s)
	return values
}

// GetValues gets values safely value from slice
func (s *SafeSlice[V]) GetValuesAndClear() []V {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	values := s.getValues()
	s.clear()
	return values
}

// Len retuns length of slice
func (s *SafeSlice[V]) Len() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.s)
}

// Clear clears slice
func (s *SafeSlice[V]) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
}

func (s *SafeSlice[V]) clear() {
	clear(s.s)
	s.s = s.s[:0]
}

// SetMutex sets new Mutex
func (s *SafeSlice[V]) SetMutex(mutex *sync.RWMutex) bool {
	if mutex == nil {
		return false
	}
	s.mutex = mutex
	return true
}

// GetMutex gets Mutex
func (s *SafeSlice[V]) GetMutex() *sync.RWMutex {
	return s.mutex
}

// SafeSliceComparable is struct build on SafeSlice with comparable interface
type SafeSliceComparable[V comparable] struct {
	Slice SafeSlice[V]
}

func MakeSafeSliceComparable[V comparable]() SafeSliceComparable[V] {
	return SafeSliceComparable[V]{
		Slice: MakeSafeSlice[V](),
	}
}

// Contains checks if value is in slice
func (s SafeSliceComparable[V]) Contains(v V) bool {
	s.Slice.mutex.RLock()
	defer s.Slice.mutex.RUnlock()
	return slices.Contains(s.Slice.s, v)
}

// Remove removes value safely from slice
func (s *SafeSliceComparable[V]) Remove(value V) {
	s.Slice.mutex.Lock()
	defer s.Slice.mutex.Unlock()
	s.Slice.s = RemoveElement(s.Slice.s, value)
}
