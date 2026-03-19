package webrtc

import (
	"sync"
	"webtools"
)

/*
ReplayWindow is struct for Replay Window that allows to check for Replay Attack
Used in WebRTC DTLS, uses sequencial numbered packets
*/
type ReplayWindow[checkedValueType uint8 | uint16 | uint32 | uint64] struct {
	window    []byte
	rightEdge checkedValueType
	mutex     *sync.RWMutex
}

/*
MakeReplayWindow initializes ReplayWindow.
WindowSize must fit in checkedValue type, else the program trims to this value and can intoduces invalid values (program can panic).
WindowSize is in bytes and it can fit n*8 values, co if n is 2, it can store 16 values.
*/
func MakeReplayWindow[checkedValueType uint8 | uint16 | uint32 | uint64](windowSize int) ReplayWindow[checkedValueType] {
	if uint64(checkedValueType(windowSize)*8) != uint64(windowSize*8) {
		panic("windowsSize bigger than rightEdge max value")
	}
	return ReplayWindow[checkedValueType]{window: make([]byte, windowSize), rightEdge: checkedValueType(windowSize*8) - 1, mutex: &sync.RWMutex{}}
}

/*
ApplyWindowCheck checks if number is in window and if it was already set or not.
Returns True if value is in range of window but was not set.
Returns False if value is out of range of window.
Returns False if value is already set in window.
*/
func (window *ReplayWindow[checkedValueType]) ApplyWindowCheck(number checkedValueType) bool {
	window.mutex.Lock()
	defer window.mutex.Unlock()
	if number < window.rightEdge+1-checkedValueType(len(window.window)*8) {
		//Value smaller that window
		return false
	}
	if number > window.rightEdge {
		//Value bigger that window - shift
		webtools.BitshiftArrayLeft(window.window, int(number-window.rightEdge))
		window.rightEdge = number
		return true
	}
	//Value in window
	bitPos := uint64(uint(len(window.window)*8)) - uint64(window.rightEdge-number) - 1
	found := webtools.CheckBitArray(window.window, bitPos)
	webtools.SetBitValueArray(window.window, bitPos, true)
	return !found
}

/*
IsWindowFull checks if window has all bits in range full
*/
func (window *ReplayWindow[checkedValueType]) IsWindowFull(bitsFromLeft int) bool {
	window.mutex.Lock()
	defer window.mutex.Unlock()
	var byteCheck int = 0
	//Check whole bytes
	for byteCheck = 0; byteCheck < int(bitsFromLeft/8); byteCheck++ {
		if window.window[byteCheck] != 255 {
			return false
		}
	}

	//Check last bits
	for bitCheck := uint8(0); bitCheck < uint8(bitsFromLeft%8); bitCheck++ {
		if !webtools.CheckBit(window.window[byteCheck], bitCheck) {
			return false
		}
	}
	return true
}
