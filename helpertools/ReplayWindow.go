package helpertools

import (
	"errors"
	"sync"
)

// ReplayWindow is struct for Replay Window that allows to check for Replay Attack (can determine if packet with specified sequence number arrived or not).
// Handles history of 64 numbers = if rightEdge is 128, it will allow to pass all numbers to 65 (inclueded).
//
// Sometimes it is called BitMask
type ReplayWindow[checkedValueType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowHolderType ~uint8 | ~uint16 | ~uint32 | ~uint64] struct {
	// window is uint64 holder of data (64 bits = 64 values)
	window []windowHolderType
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

// NewReplayWindow initializes ReplayWindow. windowWordCount is count of windowHolderType (total window size = windowWordCount * countOfBytes(windowHolderType) * 8)
func NewReplayWindow[checkedValueType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowHolderType ~uint8 | ~uint16 | ~uint32 | ~uint64](windowWordCount uint8) (*ReplayWindow[checkedValueType, windowHolderType], error) {
	if GetByteSize[windowHolderType]() <= 2 && uint16(GetBitSize[windowHolderType]()*windowWordCount) > uint16(checkedValueType(checkedValueType(0)-1)) {
		return nil, errors.New("window too big for checkedValueType")
	}
	return &ReplayWindow[checkedValueType, windowHolderType]{window: make([]windowHolderType, windowWordCount), rightEdge: 0, mutex: sync.Mutex{}, isFirstValue: true, maxForwardJump: (checkedValueType(0) - 1) >> 1}, nil
}

// GetWindowBitSize retuns bit count of window
func (window *ReplayWindow[checkedValueType, windowHolderType]) GetWindowBitSize() checkedValueType {
	return checkedValueType(len(window.window) << GetBitShiftSize[windowHolderType]())
}

// ApplyWindowCheck checks if number is in window and if it was already set or not.
//
// Returns True if value is in range of window but was not set.
//
// Returns False if value is out of range of window.
//
// Returns False if value is already set in window.
func (window *ReplayWindow[checkedValueType, windowHolderType]) ApplyWindowCheck(number checkedValueType) bool {
	//Lock mutex
	window.mutex.Lock()
	defer window.mutex.Unlock()

	//Check if first value
	if window.isFirstValue {
		//Set rightEdge to number (first value ignores maxForwardJump)
		window.rightEdge = number

		//Set window to 1 to set first bit to 1
		for i := range len(window.window) {
			window.window[i] = 0
		}
		window.window[len(window.window)-1] = 1
		window.isFirstValue = false
		return true
	}

	//Calculate forward jump (example if overflows: uint8(1) - uint8(254) = 3)
	forwardJump := number - window.rightEdge
	windowSizeBits := window.GetWindowBitSize()
	if forwardJump > 0 && forwardJump < window.maxForwardJump {
		//Valid forward jump
		if forwardJump >= windowSizeBits {
			//Overflow window
			window.rightEdge = number
			for i := range len(window.window) {
				window.window[i] = 0
			}
			window.window[len(window.window)-1] = 1
			return true
		}

		//Shift window by forwardJump
		BitShiftArrayLeft(window.window, int(forwardJump))

		//Set rightEdge to new value
		window.rightEdge = number

		//Set window last bit to 1
		window.window[len(window.window)-1] |= 1
		return true
	}

	//Check if value is same
	if forwardJump == 0 {
		return false
	}

	//Calculate older value
	olderValue := window.rightEdge - number
	if olderValue >= windowSizeBits {
		//Out of range
		return false
	}

	//Check bit
	if CheckBitArray(window.window, uint64(olderValue)) {
		return false
	}
	window.window = SetBitArray(window.window, uint64(olderValue))
	return true
}

// CheckValue only checks, if value is in window and if is set to active
func (window *ReplayWindow[checkedValueType, windowHolderType]) CheckValue(value checkedValueType) bool {
	//Lock mutex
	window.mutex.Lock()
	defer window.mutex.Unlock()

	//Check if in range
	location := window.rightEdge - value
	if location >= window.GetWindowBitSize() {
		//Out of range
		return false
	}

	return CheckBitArray(window.window, uint64(location))
}

// SetWindowData sets window data to specified values
func (window *ReplayWindow[checkedValueType, windowHolderType]) SetWindowData(windowBytes []windowHolderType, rightEdge checkedValueType) error {
	//Check window sizes
	if len(window.window) != len(windowBytes) {
		return errors.New("window sizes does not match")
	}

	//Lock mutex
	window.mutex.Lock()
	defer window.mutex.Unlock()

	//Write data
	copy(window.window, windowBytes)
	window.rightEdge = rightEdge
	window.isFirstValue = false
	return nil
}

// SetWindowDataBytes sets rightEdge and window data to specified values in binary format.
//
// Argument wordCount sets how many words should be read from b. Set wordCount to 0 for automatic.
func (window *ReplayWindow[checkedValueType, windowHolderType]) SetWindowDataBytes(b []byte, wordCount uint8) {
	//Lock mutex
	window.mutex.Lock()
	defer window.mutex.Unlock()

	//Parse rightEdge
	var readBytes uint8
	window.rightEdge, readBytes = ParseGenericLitteEndian[checkedValueType](b)

	//Read size
	if wordCount == 0 {
		wordCount = uint8(b[readBytes])
		readBytes++
	}
	b = b[readBytes:]

	//Read data
	for i := range min(wordCount, uint8(len(window.window))) {
		window.window[uint8(len(window.window))-1-i], readBytes = ParseGenericLitteEndian[windowHolderType](b)
		b = b[readBytes:]
	}
}

// GetWindowData gets window data and rightEdge
func (window *ReplayWindow[checkedValueType, windowHolderType]) GetWindowData() (windowBytes []windowHolderType, rightEdge checkedValueType) {
	window.mutex.Lock()
	defer window.mutex.Unlock()
	return window.window, window.rightEdge
}

// GetWindowDataBytes gets rightEdge and window data in binary format.
//
// Use limit to limit word count. Set to 0 for no limit. Limit is counted from right edge.
//
// Argument writeSize sets if function writes count of words written to byte array.
func (window *ReplayWindow[checkedValueType, windowHolderType]) GetWindowDataBytes(limit uint8, writeSize bool) []byte {
	//Lock mutex
	window.mutex.Lock()
	defer window.mutex.Unlock()

	//Write window right edge
	result := make([]byte, 0)
	result, _ = AppendGenericLitteEndian(result, window.rightEdge)

	//Write size
	if limit == 0 || limit > uint8(len(window.window)) {
		limit = uint8(len(window.window))
	}
	if writeSize {
		result = append(result, limit)
	}

	//Write window data
	for i := range limit {
		result, _ = AppendGenericLitteEndian(result, window.window[uint8(len(window.window))-1-i])
	}
	return result
}

// JoinWindowData tries to join windowBytes with current window bytes using bitwise OR.
//
// If distance of rightEdges is bigger than 64 and new rightEdge is bigger than internal, data of internal window is overwriten.
//
// If distance is smaller than 64 and new rightEdge is bigger than internal, internal window is shifted and joined using OR with new one.
//
// If distance is smaller than 64 and new rightEdge is smaller then internal, new window is shifted and joined using OR with old one.
//
// Note: When joining, algorithm goes from end (when windowBytes are shorter than window, 0 is set)
func (window *ReplayWindow[checkedValueType, windowHolderType]) JoinWindowData(windowBytes []windowHolderType, rightEdge checkedValueType) {
	//Lock mutex
	window.mutex.Lock()
	defer window.mutex.Unlock()
	window.isFirstValue = false

	//Calculate forward jump (example if overflows: uint8(1) - uint8(254) = 3)
	forwardJump := rightEdge - window.rightEdge
	windowSizeBits := window.GetWindowBitSize()
	if forwardJump > 0 && forwardJump < window.maxForwardJump {
		//Valid forward jump
		if forwardJump >= windowSizeBits {
			//Overflow window
			window.rightEdge = rightEdge
			clear(window.window)
			for i := 0; i < min(len(window.window), len(windowBytes)); i++ {
				window.window[len(window.window)-1-i] = windowBytes[len(windowBytes)-1-i]
			}
			return
		}

		//Shift window by forwardJump
		BitShiftArrayLeft(window.window, int(forwardJump))

		//Set rightEdge to new value
		window.rightEdge = rightEdge

		//Join windows
		for i := 0; i < min(len(window.window), len(windowBytes)); i++ {
			window.window[len(window.window)-1-i] |= windowBytes[len(windowBytes)-1-i]
		}
		return
	}

	//Check if value is same
	if forwardJump == 0 {
		return
	}

	//Calculate older value
	olderValue := window.rightEdge - rightEdge
	if olderValue >= windowSizeBits {
		//Out of range
		return
	}

	//Move new window
	temp := make([]windowHolderType, len(windowBytes))
	copy(temp, windowBytes)
	BitShiftArrayLeft(temp, int(olderValue))

	//OR Arrays
	for i := 0; i < min(len(window.window), len(temp)); i++ {
		window.window[len(window.window)-1-i] |= temp[len(temp)-1-i]
	}
}

// JoinWindowData tries to join windowBytes with current window bytes using bitwise OR encoded in binary format.
//
// Argument wordCount sets how many words should be read from b. Set wordCount to 0 for automatic.
//
// If distance of rightEdges is bigger than 64 and new rightEdge is bigger than internal, data of internal window is overwriten.
//
// If distance is smaller than 64 and new rightEdge is bigger than internal, internal window is shifted and joined using OR with new one.
//
// If distance is smaller than 64 and new rightEdge is smaller then internal, new window is shifted and joined using OR with old one.
//
// Returns number of read bytes
func (window *ReplayWindow[checkedValueType, windowHolderType]) JoinWindowDataBytes(b []byte, wordCount uint8) int {
	//Parse rightEdge
	rightEdge, readBytes := ParseGenericLitteEndian[checkedValueType](b)

	//Read size
	if wordCount == 0 {
		wordCount = uint8(b[readBytes])
		readBytes++
	}
	b = b[readBytes:]

	//Read data
	windowBytes := make([]windowHolderType, min(wordCount, uint8(len(window.window))))
	for i := range len(windowBytes) {
		windowBytes[len(windowBytes)-1-i], readBytes = ParseGenericLitteEndian[windowHolderType](b)
		b = b[readBytes:]
	}

	//Perform join
	window.JoinWindowData(windowBytes, rightEdge)
	return int(readBytes) + len(windowBytes)*int(GetByteSize[windowHolderType]())
}

// IterateReplayWindowBits goes through every bit and if bit activnes matches isSet it is send to f function callback
func IterateReplayWindowBits[checkedValueType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowHolderType ~uint8 | ~uint16 | ~uint32 | ~uint64](window []windowHolderType, rightEdge checkedValueType, f func(value checkedValueType), isSet bool) {
	//Process all values of windows
	size := uint64(GetBitSize[windowHolderType]()) * uint64(len(window))
	for i := range size {
		if CheckBitArray(window, i) == isSet {
			//Bit matches isSet
			f(rightEdge - (checkedValueType(size) - 1 - checkedValueType(i)))
		}
	}
}

// GetReplayWindowBits gets every bit matching isSet value and returns thme in array
func GetReplayWindowBits[checkedValueType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowHolderType ~uint8 | ~uint16 | ~uint32 | ~uint64](window []windowHolderType, rightEdge checkedValueType, isSet bool) []checkedValueType {
	result := make([]checkedValueType, 0, len(window)*int(GetBitSize[windowHolderType]()))
	IterateReplayWindowBits(window, rightEdge, func(value checkedValueType) {
		result = append(result, value)
	}, isSet)
	return result
}
