package helpertools

import (
	"encoding/binary"
	"sync"
)

// ReplayWindow is struct for Replay Window that allows to check for Replay Attack (can determine if packet with specified sequence number arrived or not).
// Handles history of 64 numbers = if rightEdge is 128, it will allow to pass all numbers to 65 (inclueded).
//
// Sometimes it is called BitMask
type ReplayWindow[checkedValueType ~uint8 | ~uint16 | ~uint32 | ~uint64] struct {
	// window is uint64 holder of data (64 bits = 64 values)
	window []
	// maxForwardJump is value that takes care of maximum value that can be forwarded.
	//
	// If maxForwardJump is 64, it can jump only to value 63 or smaller
	maxForwardJump checkedValueType
	// rightEdge is current highest value in window
	rightEdge checkedValueType
	// mutex is for locking and provides support on goroutines
	mutex sync.Mutex
	// isFirstValue sets if first value is being set
	isFirstValue bool
}

// NewReplayWindow initializes ReplayWindow
func NewReplayWindow[checkedValueType ~uint8 | ~uint16 | ~uint32 | ~uint64]() *ReplayWindow[checkedValueType] {
	return &ReplayWindow[checkedValueType]{window: 0, rightEdge: 0, mutex: sync.Mutex{}, isFirstValue: true, maxForwardJump: (checkedValueType(0) - 1) >> 1}
}

// ApplyWindowCheck checks if number is in window and if it was already set or not.
//
// Returns True if value is in range of window but was not set.
//
// Returns False if value is out of range of window.
//
// Returns False if value is already set in window.
func (window *ReplayWindow[checkedValueType]) ApplyWindowCheck(number checkedValueType) bool {
	//Lock mutex
	window.mutex.Lock()
	defer window.mutex.Unlock()

	//Check if first value
	if window.isFirstValue {
		//Set rightEdge to number (first value ignores maxForwardJump)
		window.rightEdge = number

		//Set window to 1 to set first bit to 1 and make sure that all other bits are 0
		window.window = 1
		window.isFirstValue = false
		return true
	}

	//Calculate forward jump (example if overflows: uint8(1) - uint8(254) = 3)
	forwardJump := number - window.rightEdge
	if forwardJump > 0 && forwardJump < window.maxForwardJump {
		//Valid forward jump
		if forwardJump >= 64 {
			//Overflow window
			window.rightEdge = number
			window.window = 1
			return true
		}

		//Shift window by maxForwardJump
		window.window <<= forwardJump

		//Set rightEdge to new value
		window.rightEdge = number

		//Set window last bit to 1
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

// CheckValue only checks, if value is in window and if is set to active
func (window *ReplayWindow[checkedValueType]) CheckValue(value checkedValueType) bool {
	//Lock mutex
	window.mutex.Lock()
	defer window.mutex.Unlock()

	//Check if in range
	location := window.rightEdge - value
	if location >= 64 {
		//Out of range
		return false
	}
	if window.rightEdge < value {
		//Out of range
		return false
	}
	return CheckBitUint64(window.window, uint8(location))
}

// SetWindowData sets window data to specified values
func (window *ReplayWindow[checkedValueType]) SetWindowData(windowBytes uint64, rightEdge checkedValueType) {
	window.mutex.Lock()
	defer window.mutex.Unlock()
	window.window = windowBytes
	window.rightEdge = rightEdge
	window.isFirstValue = false
}

// SetWindowDataBytes sets window data and rightEdge to specified values in binary format
func (window *ReplayWindow[checkedValueType]) SetWindowDataBytes(b []byte) {
	windowBytes := binary.LittleEndian.Uint64(b[0:8])
	rightEdge, _ := ParseGenericLitteEndian[checkedValueType](b[8:])
	window.SetWindowData(windowBytes, rightEdge)
}

// GetWindowData gets window data and rightEdge
func (window *ReplayWindow[checkedValueType]) GetWindowData() (windowBytes uint64, rightEdge checkedValueType) {
	window.mutex.Lock()
	defer window.mutex.Unlock()
	return window.window, window.rightEdge
}

// GetWindowDataBytes gets window data nd rightEdge in binary format
func (window *ReplayWindow[checkedValueType]) GetWindowDataBytes() []byte {
	//Lock mutex
	window.mutex.Lock()
	defer window.mutex.Unlock()

	//Write window data
	result := make([]byte, 0)
	binary.LittleEndian.AppendUint64(result, window.window)

	//Write window right edge
	result, _ = AppendGenericLitteEndian(result, window.rightEdge)
	return result
}

// JoinWindowData tries to join windowBytes with current window bytes using bitwise OR.
//
// If distance of rightEdges is bigger than 64 and new rightEdge is bigger than internal, data of internal window is overwriten.
//
// If distance is smaller than 64 and new rightEdge is bigger than internal, internal window is shifted and joined using OR with new one.
//
// If distance is smaller than 64 and new rightEdge is smaller then internal, new window is shifted and joined using OR with old one.
func (window *ReplayWindow[checkedValueType]) JoinWindowData(windowBytes uint64, rightEdge checkedValueType) {
	//Lock mutex
	window.mutex.Lock()
	defer window.mutex.Unlock()
	window.isFirstValue = false

	//Calculate distance of edges
	distance := rightEdge - window.rightEdge
	if rightEdge > window.rightEdge {
		//Moving forward
		if distance < 64 {
			//Shift current window
			window.window <<= distance
			window.window |= windowBytes
			window.rightEdge = rightEdge
			return
		}

		//Distance bigger = overwrite
		window.window = windowBytes
		window.rightEdge = rightEdge
		return
	}

	//New edge is smaller
	if distance > 63 {
		//Windows does not overlap
		return
	}

	//Move new window
	window.window |= windowBytes << distance
}

// JoinWindowData tries to join windowBytes with current window bytes using bitwise OR encoded in binary format.
//
// If distance of rightEdges is bigger than 64 and new rightEdge is bigger than internal, data of internal window is overwriten.
//
// If distance is smaller than 64 and new rightEdge is bigger than internal, internal window is shifted and joined using OR with new one.
//
// If distance is smaller than 64 and new rightEdge is smaller then internal, new window is shifted and joined using OR with old one.
//
// Returns number of read bytes
func (window *ReplayWindow[checkedValueType]) JoinWindowDataBytes(b []byte) uint8 {
	windowBytes := binary.LittleEndian.Uint64(b[0:8])
	rightEdge, bytesRead := ParseGenericLitteEndian[checkedValueType](b[8:])
	window.JoinWindowData(windowBytes, rightEdge)
	return 8 + bytesRead
}

// IterateReplayWindowBits goes through every bit and if bit activnes matches isSet it is send to f function callback
func IterateReplayWindowBits[checkedValueType ~uint8 | ~uint16 | ~uint32 | ~uint64](window uint64, rightEdge checkedValueType, f func(value checkedValueType), isSet bool) {
	//Process all values of windows
	for i := range uint8(64) {
		if CheckBitUint64(window, i) == isSet {
			//Bit matches isSet
			f(rightEdge - checkedValueType((uint8(63) - i)))
		}
	}
}

// GetReplayWindowBits gets every bit matching isSet value and returns thme in array
func GetReplayWindowBits[checkedValueType ~uint8 | ~uint16 | ~uint32 | ~uint64](window uint64, rightEdge checkedValueType, isSet bool) []checkedValueType {
	result := make([]checkedValueType, 0)
	IterateReplayWindowBits(window, rightEdge, func(value checkedValueType) {
		result = append(result, value)
	}, isSet)
	return result
}
