package webtools

/*
SetBitValueArray sets bit to value at specific position in array - position in array = pos/8, position in bit = pos%8
*/
func SetBitValueArray(b []byte, pos uint64, value bool) []byte {
	//Check for overflow
	if pos/8 >= uint64(len(b)) {
		return b
	}

	//Select byte
	b[pos/8] = SetBitValue(b[pos/8], uint8(pos%8), value)
	return b
}

/*
SetBitValue sets bit to value at specific position
*/
func SetBitValue(b byte, pos uint8, value bool) byte {
	if value {
		return SetBit(b, pos)
	}
	return ClearBit(b, pos)
}

/*
SetBit sets bit (sets 1) at specific position
*/
func SetBit(b byte, pos uint8) byte {
	//Check for overflow
	if pos > 7 {
		return b
	}
	b |= 1 << pos
	return b
}

/*
ClearBit clears bit (sets 0) at specific position
*/
func ClearBit(b byte, pos uint8) byte {
	//Check for overflow
	if pos > 7 {
		return b
	}
	b &^= 1 << pos
	return b
}

/*
CheckBitArray checks bit at specific position in array - position in array = pos/8, position in bit = pos%8
*/
func CheckBitArray(b []byte, pos uint64) bool {
	//Check for overflow
	if pos/8 >= uint64(len(b)) {
		return false
	}

	//Select byte
	return CheckBit(b[pos/8], uint8(pos%8))
}

/*
CheckBit checks if bit is set (1)
*/
func CheckBit(b byte, pos uint8) bool {
	//Check for overflow
	if pos > 7 {
		return false
	}
	return b&(1<<pos) == 1
}
