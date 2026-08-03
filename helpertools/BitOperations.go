// helpertools package provides some other nonspecific tools for generic usage
package helpertools

import (
	"encoding/binary"
	"unsafe"
)

// GetByteSize gets byte size of T
func GetByteSize[T ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~int8 | ~int16 | ~int32 | ~int64]() uint8 {
	var zero T
	return uint8(unsafe.Sizeof(zero))
}

// GetBitSize gets bit size of T
func GetBitSize[T ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~int8 | ~int16 | ~int32 | ~int64]() uint8 {
	var zero T
	return uint8(unsafe.Sizeof(zero) << 3)
}

// GetBitShiftSize gets bit shift value of one T (for uint8 it retuns 3)
func GetBitShiftSize[T ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~int8 | ~int16 | ~int32 | ~int64]() uint8 {
	var zero T
	switch unsafe.Sizeof(zero) {
	case 1:
		{
			return 3
		}
	case 2:
		{
			return 4
		}
	case 4:
		{
			return 5
		}
	case 8:
		{
			return 6
		}
	}
	return 0
}

/*
SetBit sets bit (sets 1) at specific position -> 0 = 128 (MSB), 7 = 1 (LSB)
*/
func SetBit(b byte, pos uint8) byte {
	if pos > 7 {
		return b
	}
	return b | (uint8(1) << (7 - pos))
}

/*
SetBitUint16 sets bit (sets 1) at specific position -> 0 = 2^15 (MSB), 15 = 1 (LSB)
*/
func SetBitUint16(b uint16, pos uint8) uint16 {
	if pos > 15 {
		return b
	}
	return b | (uint16(1) << (15 - pos))
}

/*
SetBitUint32 sets bit (sets 1) at specific position -> 0 = 2^31 (MSB), 31 = 1 (LSB)
*/
func SetBitUint32(b uint32, pos uint8) uint32 {
	if pos > 31 {
		return b
	}
	return b | (uint32(1) << (31 - pos))
}

/*
SetBitUint64 sets bit (sets 1) at specific position -> 0 = 2^63 (MSB), 63 = 1 (LSB)
*/
func SetBitUint64(b uint64, pos uint8) uint64 {
	if pos > 63 {
		return b
	}
	return b | (uint64(1) << (63 - pos))
}

/*
ClearBit clears bit (sets 0) at specific position -> 0 = 128 (MSB), 7 = 1 (LSB)
*/
func ClearBit(b byte, pos uint8) byte {
	if pos > 7 {
		return b
	}
	return b &^ (uint8(1) << (7 - pos))
}

/*
ClearBitUint16 clears bit (sets 0) at specific position -> 0 = 2^15 (MSB), 15 = 1 (LSB)
*/
func ClearBitUint16(b uint16, pos uint8) uint16 {
	if pos > 15 {
		return b
	}
	return b &^ (uint16(1) << (15 - pos))
}

/*
ClearBitUint32 clears bit (sets 0) at specific position -> 0 = 2^31 (MSB), 31 = 1 (LSB)
*/
func ClearBitUint32(b uint32, pos uint8) uint32 {
	if pos > 31 {
		return b
	}
	return b &^ (uint32(1) << (31 - pos))
}

/*
ClearBitUint64 clears bit (sets 0) at specific position -> 0 = 2^63 (MSB), 63 = 1 (LSB)
*/
func ClearBitUint64(b uint64, pos uint8) uint64 {
	if pos > 63 {
		return b
	}
	return b &^ (uint64(1) << (63 - pos))
}

/*
CheckBit checks if bit is set (1). pos -> 0 = 128 (MSB), 7 = 1 (LSB)
*/
func CheckBit(b byte, pos uint8) bool {
	if pos > 7 {
		return false
	}
	return b&(uint8(1)<<(7-pos)) != 0
}

/*
CheckBitUint16 checks if bit is set (1). pos -> 0 = 2^15 (MSB), 15 = 1 (LSB)
*/
func CheckBitUint16(b uint16, pos uint8) bool {
	if pos > 15 {
		return false
	}
	return b&(uint16(1)<<(15-pos)) != 0
}

/*
CheckBitUint32 checks if bit is set (1). pos -> 0 = 2^31 (MSB), 31 = 1 (LSB)
*/
func CheckBitUint32(b uint32, pos uint8) bool {
	if pos > 31 {
		return false
	}
	return b&(uint32(1)<<(31-pos)) != 0
}

/*
CheckBitUint64 checks if bit is set (1). pos -> 0 = 2^63 (MSB), 63 = 1 (LSB)
*/
func CheckBitUint64(b uint64, pos uint8) bool {
	if pos > 63 {
		return false
	}
	return b&(uint64(1)<<(63-pos)) != 0
}

/*
SetBitArray sets bit (set 1) at specific position in array - position in array = pos/8, position in bit = pos%8 -> 0 = 128 (MSB), 7 = 1 (LSB)
*/
func SetBitArray(b []byte, pos uint64) []byte {
	//Check for overflow
	byteIndex := pos >> 3
	if byteIndex >= uint64(len(b)) {
		return b
	}

	//Set bit
	b[byteIndex] |= (uint8(1) << (7 - pos&7))
	return b
}

/*
ClearBitArray clears bit (set 0) at specific position in array - position in array = pos/8, position in bit = pos%8 -> 0 = 128 (MSB), 7 = 1 (LSB)
*/
func ClearBitArray(b []byte, pos uint64) []byte {
	//Check for overflow
	byteIndex := pos >> 3
	if byteIndex >= uint64(len(b)) {
		return b
	}

	//Clear bit
	b[byteIndex] &^= (uint8(1) << (7 - pos&7))
	return b
}

/*
CheckBitArray checks bit (1) at specific position in array - position in array = pos/8, position in bit = pos%8 -> 0 = 128, 7 = 1
*/
func CheckBitArray(b []byte, pos uint64) bool {
	//Check for overflow
	byteIndex := pos >> 3
	if byteIndex >= uint64(len(b)) {
		return false
	}

	//Select byte
	return b[byteIndex]&(uint8(1)<<(7-pos&7)) != 0
}

/*
XORArrays applies XOR operation to whole arrays -> target[i] ^= source[i].
*/
func XORArrays(target []byte, source []byte) {
	n := len(source)
	if n > len(target) {
		n = len(target)
	}

	//Do XOR
	for i := range n {
		target[i] ^= source[i]
	}
}

/*
BitshiftArrayLeft bitshifts array left by n bits (negative does right)
*/
func BitshiftArrayLeft(data []byte, bitShift int) {
	if bitShift < 0 {
		BitshiftArrayRight(data, -bitShift)
	} else if bitShift == 0 {
		return
	} else {
		//Bitshift left
		bytes := bitShift >> 3
		if bytes >= len(data) {
			for i := range data {
				data[i] = 0
			}
			return
		} else if bytes > 0 {
			//Byteshift left
			for i := 0; i < len(data)-bytes; i++ {
				data[i] = data[i+bytes]
			}
			for i := len(data) - bytes; i < len(data); i++ {
				data[i] = 0
			}
		}
		bitShift &= 7
		if bitShift > 0 {
			for i := 0; i < len(data)-1; i++ {
				data[i] = data[i]<<bitShift | data[i+1]>>(8-bitShift)
			}
			data[len(data)-1] <<= bitShift
		}
	}
}

/*
BitshiftArrayRight bitshifts array right by n bits (negative does left)
*/
func BitshiftArrayRight(data []byte, bitShift int) {
	if bitShift < 0 {
		BitshiftArrayLeft(data, -bitShift)
	} else if bitShift == 0 {
		return
	} else {
		//Bitshift right
		bytes := bitShift >> 3
		if bytes >= len(data) {
			for i := range data {
				data[i] = 0
			}
			return
		} else if bytes > 0 {
			//Byteshift right
			for i := len(data) - 1; i >= bytes; i-- {
				data[i] = data[i-bytes]
			}
			for i := 0; i < bytes; i++ {
				data[i] = 0
			}
		}
		bitShift &= 7
		if bitShift > 0 {
			for i := len(data) - 1; i > 0; i-- {
				data[i] = data[i]>>bitShift | data[i-1]<<(8-bitShift)
			}
			data[0] >>= bitShift
		}
	}
}

/*
BitshiftUint64ArrayLeft bitshifts array left by n bits (negative does right)
*/
func BitshiftUint64ArrayLeft(data []uint64, bitShift int) {
	if bitShift < 0 {
		BitshiftUint64ArrayRight(data, -bitShift)
	} else if bitShift == 0 {
		return
	} else {
		//Bitshift left
		bytes := bitShift >> 6
		if bytes >= len(data) {
			for i := range data {
				data[i] = 0
			}
			return
		} else if bytes > 0 {
			//Byteshift left
			for i := 0; i < len(data)-bytes; i++ {
				data[i] = data[i+bytes]
			}
			for i := len(data) - bytes; i < len(data); i++ {
				data[i] = 0
			}
		}
		bitShift &= 63
		if bitShift > 0 {
			for i := 0; i < len(data)-1; i++ {
				data[i] = data[i]<<bitShift | data[i+1]>>(64-bitShift)
			}
			data[len(data)-1] <<= bitShift
		}
	}
}

/*
BitshiftUint64ArrayRight bitshifts array right by n bits (negative does left)
*/
func BitshiftUint64ArrayRight(data []uint64, bitShift int) {
	if bitShift < 0 {
		BitshiftUint64ArrayLeft(data, -bitShift)
	} else if bitShift == 0 {
		return
	} else {
		//Bitshift right
		bytes := bitShift >> 6
		if bytes >= len(data) {
			for i := range data {
				data[i] = 0
			}
			return
		} else if bytes > 0 {
			//Byteshift right
			for i := len(data) - 1; i >= bytes; i-- {
				data[i] = data[i-bytes]
			}
			for i := 0; i < bytes; i++ {
				data[i] = 0
			}
		}
		bitShift &= 63
		if bitShift > 0 {
			for i := len(data) - 1; i > 0; i-- {
				data[i] = data[i]>>bitShift | data[i-1]<<(64-bitShift)
			}
			data[0] >>= bitShift
		}
	}
}

/*
BitshiftGenericUintArrayLeft bitshifts array left by n bits (negative does right)
*/
func BitshiftGenericUintArrayLeft[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](data []valueType, bitShift int) {
	if bitShift < 0 {
		BitshiftGenericUintArrayRight(data, -bitShift)
	} else if bitShift == 0 {
		return
	} else {
		//Bitshift left
		bytes := bitShift >> int(GetBitShiftSize[valueType]())
		if bytes >= len(data) {
			for i := range data {
				data[i] = 0
			}
			return
		} else if bytes > 0 {
			//Byteshift left
			for i := 0; i < len(data)-bytes; i++ {
				data[i] = data[i+bytes]
			}
			for i := len(data) - bytes; i < len(data); i++ {
				data[i] = 0
			}
		}
		bitCount := int(GetBitSize[valueType]())
		bitShift &= bitCount - 1
		if bitShift > 0 {
			for i := 0; i < len(data)-1; i++ {
				data[i] = data[i]<<bitShift | data[i+1]>>(bitCount-bitShift)
			}
			data[len(data)-1] <<= bitShift
		}
	}
}

/*
BitshiftUint64ArrayRight bitshifts array right by n bits (negative does left)
*/
func BitshiftGenericUintArrayRight[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](data []valueType, bitShift int) {
	if bitShift < 0 {
		BitshiftGenericUintArrayLeft(data, -bitShift)
	} else if bitShift == 0 {
		return
	} else {
		//Bitshift right
		bytes := bitShift >> int(GetBitShiftSize[valueType]())
		if bytes >= len(data) {
			for i := range data {
				data[i] = 0
			}
			return
		} else if bytes > 0 {
			//Byteshift right
			for i := len(data) - 1; i >= bytes; i-- {
				data[i] = data[i-bytes]
			}
			for i := 0; i < bytes; i++ {
				data[i] = 0
			}
		}
		bitCount := int(GetBitSize[valueType]())
		bitShift &= bitCount - 1
		if bitShift > 0 {
			for i := len(data) - 1; i > 0; i-- {
				data[i] = data[i]>>bitShift | data[i-1]<<(bitCount-bitShift)
			}
			data[0] >>= bitShift
		}
	}
}

// AppendGenericLitteEndian appends encoded LittleEndian to b slice. Return number of written bytes
func AppendGenericLitteEndian[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](b []byte, value valueType) (result []byte, addedBytes uint8) {
	switch any(value).(type) {
	case uint8:
		b = append(b, byte(value))
		return b, 1
	case uint16:
		return binary.LittleEndian.AppendUint16(b, uint16(value)), 2
	case uint32:
		return binary.LittleEndian.AppendUint32(b, uint32(value)), 4
	case uint64:
		return binary.LittleEndian.AppendUint64(b, uint64(value)), 8
	}
	return b, 0
}

// ParseGenericLitteEndian reads bytes from b and parses them using LittleEndian. Returns value and count of read bytes
func ParseGenericLitteEndian[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](b []byte) (value valueType, readBytes uint8) {
	switch any(value).(type) {
	case uint8:
		return valueType(b[0]), 1
	case uint16:
		return valueType(binary.LittleEndian.Uint16(b[0:2])), 2
	case uint32:
		return valueType(binary.LittleEndian.Uint32(b[0:4])), 4
	case uint64:
		return valueType(binary.LittleEndian.Uint64(b[0:8])), 8
	}
	return 0, 0
}
