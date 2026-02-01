package database

import (
	"io"
	"os"
)

/*
SmartDBString is string with dynamic data usage in DB files
*/
type SmartDBString struct {
	Value string
}

/*
ConvertToBytesDB converts SmartDBString to bytes
*/
func (smart *SmartDBString) ConvertToBytesDB(writer io.Writer) error {
	//Get byte size
	val := []byte(smart.Value)
	size := calculateByteSizeFromInt(uint(len(val)))
	if size == 255 {
		return os.ErrInvalid
	}

	//Write byte size of length
	err := ConvertUint8ToBytesDB(writer, size)
	if err != nil {
		return err
	}

	//Write length
	err = ConvertUintXToBytesDB(writer, uint64(len(val)), size)
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
func (smart *SmartDBString) ParseBytesDB(reader io.Reader) error {
	//Read byte size of length
	size, err := ParseUint8DB(reader)
	if err != nil {
		return err
	}

	//Read length
	length, err := ParseUintXDB(reader, size)
	if err != nil {
		return err
	}

	//Read data
	result := make([]byte, length)
	_, err = reader.Read(result)
	if err != nil {
		return err
	}
	smart.Value = string(result)
	return nil
}

/*
CanParseDBToAny returns true = All values are written to DB
*/
func (smart *SmartDBString) CanParseDBToAny() bool {
	return true
}

func calculateByteSizeFromInt(value uint) uint8 {
	for i := uint8(1); i < 9; i++ {
		if value <= (1 << (8 * i)) {
			return i
		}
	}
	return 255
}
