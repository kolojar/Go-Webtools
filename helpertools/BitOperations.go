// helpertools package provides some other nonspecific tools for generic usage
package helpertools

/*
SetBitValueArray sets bit to value at specific position in array - position in array = pos/8, position in bit = pos%8 -> 0 = 128, 7 = 1
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
SetBitValue sets bit to value at specific position -> 0 = 128, 7 = 1
*/
func SetBitValue(b byte, pos uint8, value bool) byte {
	if value {
		return SetBit(b, pos)
	}
	return ClearBit(b, pos)
}

/*
SetBit sets bit (sets 1) at specific position -> 0 = 128, 7 = 1
*/
func SetBit(b byte, pos uint8) byte {
	return SetBitGeneric(b, pos, 8)
}

/*
SetBitUint64 sets bit (sets 1) at specific position -> 0 = 2^63, 63 = 1
*/
func SetBitUint64(b uint64, pos uint8) uint64 {
	return SetBitGeneric(b, pos, 64)
}

/*
SetBit sets bit (sets 1) at specific position. Bitsize for uint8 is 8 -> 0 = 128, 7 = 1
*/
func SetBitGeneric[T uint8 | uint16 | uint32 | uint64](b T, pos uint8, bitSize uint8) T {
	//Check for overflow
	if pos > bitSize-1 {
		return b
	}
	b |= 1 << (bitSize - 1 - pos)
	return b
}

/*
ClearBit clears bit (sets 0) at specific position -> 0 = 128, 7 = 1
*/
func ClearBit(b byte, pos uint8) byte {
	return ClearBitGeneric(b, pos, 8)
}

/*
ClearBitGeneric clears bit (sets 0) at specific position. Bitsize for uint8 is 8 -> 0 = 128, 7 = 1
*/
func ClearBitGeneric[T uint8 | uint16 | uint32 | uint64](b T, pos uint8, bitSize uint8) T {
	//Check for overflow
	if pos > bitSize-1 {
		return b
	}
	b &^= 1 << (7 - pos)
	return b
}

/*
CheckBitArray checks bit at specific position in array - position in array = pos/8, position in bit = pos%8 -> 0 = 128, 7 = 1
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
CheckBit checks if bit is set (1). pos -> 0 = 128, 7 = 1
*/
func CheckBit(b byte, pos uint8) bool {
	return CheckBitGenetic(b, pos, 8)
}

/*
CheckBit checks if bit is set (1). pos -> 0 = 2^63, 63 = 1
*/
func CheckBitUint64(b uint64, pos uint8) bool {
	return CheckBitGenetic(b, pos, 64)
}

/*
CheckBitGenetic checks if bit is set (1). pos. Bitsize for uint8 is 8 -> 0 = 128, 7 = 1
*/
func CheckBitGenetic[T uint8 | uint16 | uint32 | uint64](b T, pos uint8, bitSize uint8) bool {
	//Check for overflow
	if pos > bitSize-1 {
		return false
	}
	return b&T(1<<(bitSize-1-pos)) != 0
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

/*
BitshiftArrayLeft bitshifts array left by n bits (negative does right)
*/
func BitshiftArrayLeft(data []byte, bitShift int) {
	if bitShift < 0 {
		BitshiftArrayRight(data, -bitShift)
	} else {
		//Bitshift left
		bytes := bitShift / 8
		if bytes > 0 {
			//Byteshift left
			for i := 0; i < len(data)-bytes; i++ {
				data[i] = data[i+bytes]
			}
			for i := len(data) - bytes; i < len(data); i++ {
				data[i] = 0
			}
		}
		bitShift = bitShift % 8
		for i := 0; i < len(data)-1; i++ {
			data[i] = data[i]<<bitShift | data[i+1]>>(8-bitShift)
		}
		data[len(data)-1] <<= bitShift
	}
}

/*
BitshiftArrayRight bitshifts array right by n bits (negative does left)
*/
func BitshiftArrayRight(data []byte, bits int) {
	if bits < 0 {
		BitshiftArrayLeft(data, -bits)
	} else {
		//Bitshift right
		bytes := bits / 8
		if bytes > 0 {
			//Byteshift right
			for i := len(data) - 1; i >= bytes; i-- {
				data[i] = data[i-bytes]
			}
			for i := 0; i < bytes; i++ {
				data[i] = 0
			}
		}
		bits = bits % 8
		for i := len(data) - 1; i > 0; i-- {
			data[i] = data[i]>>bits | data[i-1]<<(8-bits)
		}
		data[0] >>= bits
	}
}
