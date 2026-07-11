package helpertools

import "sync"

// ReplayWindow is struct for Replay Window that allows to check for Replay Attack (can determine if packet with specified sequence number arrived or not).
// Handles history of 64 numbers = if rightEdge is 128, it will allow to pass all numbers to 65 (inclueded).
//
// Sometimes it is called BitMask
type ReplayWindow[checkedValueType ~uint8 | ~uint16 | ~uint32 | ~uint64] struct {
	window         uint64
	maxForwardJump checkedValueType
	rightEdge      checkedValueType
	mutex          *sync.RWMutex
	isFirstValue   bool
}

// MakeReplayWindow initializes ReplayWindow.
func MakeReplayWindow[checkedValueType ~uint8 | ~uint16 | ~uint32 | ~uint64]() ReplayWindow[checkedValueType] {
	return ReplayWindow[checkedValueType]{window: 0, rightEdge: 0, mutex: &sync.RWMutex{}, isFirstValue: true, maxForwardJump: (checkedValueType(0) - 1) >> 1}
}

// ApplyWindowCheck checks if number is in window and if it was already set or not.
//
// Returns True if value is in range of window but was not set.
//
// Returns False if value is out of range of window.
//
// Returns False if value is already set in window.
func (window *ReplayWindow[checkedValueType]) ApplyWindowCheck(number checkedValueType) bool {
	//Check if first value
	window.mutex.Lock()
	defer window.mutex.Unlock()
	if window.isFirstValue {
		window.rightEdge = number
		window.window = 1
		window.isFirstValue = false
		return true
	}

	//Calculate forward jump
	forwardJump := number - window.rightEdge
	if forwardJump > 0 && forwardJump < window.maxForwardJump {
		//Valid forward jump
		if forwardJump >= 64 {
			//Overflow window
			window.rightEdge = number
			window.window = 1
			return true
		}
		//Shift window
		window.rightEdge = number
		window.window <<= forwardJump
		window.window |= 1
		return true
	}

	//Check if value is same
	if forwardJump == 0 {
		return false
	}

	//Calculate older value
	olderValue := window.rightEdge - number
	if olderValue >= 64 {
		//Out of range
		return false
	}

	//Check bit
	if CheckBitUint64(window.window, uint8(olderValue)) {
		return false
	}
	window.window = SetBitUint64(window.window, uint8(olderValue))
	return true
}
