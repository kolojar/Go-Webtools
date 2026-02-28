package database

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"webtools"
)

/*
ErrValueTooLong informs that value is too long for this string. Use errors.Is() for checking
*/
var ErrValueTooLong = errors.New("value too long")

/*
LimitedString is string with limited byte size
*/
type LimitedString struct {
	value               string
	lengthStoreByteSize uint8
}

/*
MakeLimitedString creates new limited string.
Parameter lengthStoreByteSize sets, how big (how much bytes) will the lenght parameter saved in database. It is calulcated like this: 2^(8*lengthStoreByteSize)-1 = maxStringLength
Recommended max size is from 8 to 64 bytes (int64 of real length). Enter byte count for length (values from 1 to 8)
*/
func MakeLimitedString(lengthStoreByteSize uint8) LimitedString {
	if lengthStoreByteSize < 1 || lengthStoreByteSize > 8 {
		panic("invalid argument: lengthStoreByteSize: " + strconv.Itoa(int(lengthStoreByteSize)))
	}
	return LimitedString{lengthStoreByteSize: lengthStoreByteSize}
}

/*
Get gets the value
*/
func (limitedString *LimitedString) Get() string {
	return limitedString.value
}

func (limitedString *LimitedString) calculateMaxLength() int {
	return (1 << (limitedString.lengthStoreByteSize * 8)) - 1
}

/*
Set sets the value
*/
func (limitedString *LimitedString) Set(value string) error {
	if len(value) >= limitedString.calculateMaxLength() {
		return ErrValueTooLong
	}
	limitedString.value = value
	return nil
}

/*
ConvertToBytesDB converts SmartDBString to bytes
*/
func (limitedString *LimitedString) ConvertToBytesDB(writer io.Writer) error {
	//Write length
	val := []byte(limitedString.value)
	err := ConvertUintXToBytesDB(writer, uint64(len(val)), limitedString.lengthStoreByteSize)
	if err != nil {
		return err
	}

	//Write data
	_, err = writer.Write(val)
	return err
}

/*
ParseBytesDB parses bytes to SmartDBString
*/
func (limitedString *LimitedString) ParseBytesDB(reader io.Reader) error {
	//Read length
	length, err := ParseUintXDB(reader, limitedString.lengthStoreByteSize)
	if err != nil {
		return err
	}

	//Read data
	result := make([]byte, length)
	_, err = reader.Read(result)
	if err != nil {
		return err
	}
	limitedString.value = string(result)
	return nil
}

/*
CanParseDBToAny returns false = All values are not written to DB (the object must have prepared lengthStoreByteSize)
*/
func (limitedString *LimitedString) CanParseDBToAny() bool {
	return false
}

func (limitedString *LimitedString) InteractiveRepairDB() (bool, error) {
	//Read length
	data, err := webtools.ReadLineFromConsole("Enter LimitedString lengthStoreByteSize: ")
	if err != nil {
		fmt.Println("input err:", err)
		return false, err
	}

	//Parse to number
	num, err := strconv.Atoi(strings.ReplaceAll(string(data), "\n", ""))
	if err != nil {
		fmt.Println("convert err:", err)
		return false, err
	}

	//Check if value in range
	if num < 1 || num > 8 {
		fmt.Println("Value of LimitedString lengthStoreByteSize out of range of (1 to 8)!")
		return false, nil
	}
	limitedString.lengthStoreByteSize = uint8(num)
	return true, nil
}
