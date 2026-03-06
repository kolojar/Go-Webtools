package webtools

/*
SetBitValueArray sets bit to value at specific position in array - position in array = pos/8, position in bit = pos%8 -> 0 = 128, 7 = 0
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
SetBitValue sets bit to value at specific position -> 0 = 128, 7 = 0
*/
func SetBitValue(b byte, pos uint8, value bool) byte {
	if value {
		return SetBit(b, pos)
	}
	return ClearBit(b, pos)
}

/*
SetBit sets bit (sets 1) at specific position -> 0 = 128, 7 = 0
*/
func SetBit(b byte, pos uint8) byte {
	//Check for overflow
	if pos > 7 {
		return b
	}
	b |= 1 << (7 - pos)
	return b
}

/*
ClearBit clears bit (sets 0) at specific position -> 0 = 128, 7 = 0
*/
func ClearBit(b byte, pos uint8) byte {
	//Check for overflow
	if pos > 7 {
		return b
	}
	b &^= 1 << (7 - pos)
	return b
}

/*
CheckBitArray checks bit at specific position in array - position in array = pos/8, position in bit = pos%8 -> 0 = 128, 7 = 0
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
CheckBit checks if bit is set (1). pos -> 0 = 128, 7 = 0
*/
func CheckBit(b byte, pos uint8) bool {
	//Check for overflow
	if pos > 7 {
		return false
	}
	return b&(1<<(7-pos)) == 1
}

/*
XORArrays applies XOR operation to whole arrays -> a[i] ^ b[i].
*/
func XORArrays(a []byte, b []byte) []byte {
	if len(a) != len(b) {
		return nil
	}

	//Do XOR
	result := make([]byte, len(a))
	for i := range result {
		result[i] = a[i] ^ b[i]
	}
	return result
}
