// helpertools package provides some other nonspecific tools for generic usage
package helpertools

import (
	"encoding/binary"
	"math/bits"
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
SetBit sets bit (sets 1) at specific position -> for uint8: 0 = 128 (MSB), 7 = 1 (LSB)
*/
func SetBit[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](b valueType, pos uint8) valueType {
	if pos >= GetBitSize[valueType]() {
		return b
	}
	return b | (valueType(1) << (GetBitSize[valueType]() - 1 - pos))
}

/*
ClearBit clears bit (sets 0) at specific position -> for uint8: 0 = 128 (MSB), 7 = 1 (LSB)
*/
func ClearBit[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](b valueType, pos uint8) valueType {
	if pos >= GetBitSize[valueType]() {
		return b
	}
	return b &^ (valueType(1) << (GetBitSize[valueType]() - 1 - pos))
}

/*
CheckBit checks if bit is set (1) at specific position -> for uint8: 0 = 128 (MSB), 7 = 1 (LSB)
*/
func CheckBit[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](b valueType, pos uint8) bool {
	if pos >= GetBitSize[valueType]() {
		return false
	}
	return b&(valueType(1)<<(GetBitSize[valueType]()-1-pos)) != 0
}

/*
SetBitArray sets bit (set 1) at specific position in array - position in array = pos/8, position in bit = pos%8 -> for uint8: 0 = 128 (MSB), 7 = 1 (LSB)
*/
func SetBitArray[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](b []valueType, pos uint64) []valueType {
	//Check for overflow
	byteIndex := pos >> uint64(GetBitShiftSize[valueType]())
	if byteIndex >= uint64(len(b)) {
		return b
	}

	//Set bit
	mask := uint64(GetBitSize[valueType]()) - 1
	b[byteIndex] |= (valueType(1) << (mask - (pos & mask)))
	return b
}

/*
ClearBitArray clears bit (set 0) at specific position in array - position in array = pos/8, position in bit = pos%8 -> for uint8: 0 = 128 (MSB), 7 = 1 (LSB)
*/
func ClearBitArray[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](b []valueType, pos uint64) []valueType {
	//Check for overflow
	byteIndex := pos >> uint64(GetBitShiftSize[valueType]())
	if byteIndex >= uint64(len(b)) {
		return b
	}

	//Clear bit
	mask := uint64(GetBitSize[valueType]()) - 1
	b[byteIndex] &^= (valueType(1) << (mask - (pos & mask)))
	return b
}

/*
CheckBitArray checks bit (1) at specific position in array - position in array = pos/8, position in bit = pos%8 -> for uint8: 0 = 128 (MSB), 7 = 1 (LSB)
*/
func CheckBitArray[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](b []valueType, pos uint64) bool {
	//Check for overflow
	byteIndex := pos >> uint64(GetBitShiftSize[valueType]())
	if byteIndex >= uint64(len(b)) {
		return false
	}

	//Select byte
	mask := uint64(GetBitSize[valueType]()) - 1
	return b[byteIndex]&(valueType(1)<<(mask-(pos&mask))) != 0
}

/*
XORArrays applies XOR operation to whole arrays -> target[i] ^= source[i].
*/
func XORArrays[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](target []valueType, source []valueType) {
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
ORArrays applies OR operation to whole arrays -> target[i] |= source[i].
*/
func ORArrays[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](target []valueType, source []valueType) {
	n := len(source)
	if n > len(target) {
		n = len(target)
	}

	//Do OR
	for i := range n {
		target[i] |= source[i]
	}
}

/*
BitShiftArrayLeft bitshifts array left by n bits (negative does right) <<
*/
func BitShiftArrayLeft[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](data []valueType, bitShift int) {
	if bitShift < 0 {
		BitShiftArrayRight(data, -bitShift)
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
BitshiftUint64ArrayRight bitshifts array right by n bits (negative does left) >>
*/
func BitShiftArrayRight[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64](data []valueType, bitShift int) {
	if bitShift < 0 {
		BitShiftArrayLeft(data, -bitShift)
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
		return append(b, byte(value)), 1
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

// CalculateCountOfUsedBytesOfNumber calculates count of used bytes for value
func CalculateCountOfUsedBytesOfNumber[valueType ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint | ~int8 | ~int16 | ~int32 | ~int64 | ~int](value valueType) uint8 {
	if value == 0 {
		return 1
	}

	//Handle negative numbers
	uValue := uint64(value)
	if value < 0 {
		uValue = uint64(-int64(value))
	}

	//Ceil division
	return (uint8(bits.Len64(uint64(uValue))) + 7) >> 3
}
