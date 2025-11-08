/*
Package database provides some databases for some kinds of use cases
Please keep in mind that these databases are really simple
*/
package database

import (
	"bytes"
	"encoding/binary"
	"io"
)

/*
IDatabaseObject adds support for databases to use this objects
*/
type IDatabaseObject interface {
	ConvertToBytesDB(buffer *bytes.Buffer)
	ParseBytesDB(io.Reader) (IDatabaseObject, error)
}

/*
ParseStringDB parses bytes from reader to string
*/
func ParseStringDB(reader io.Reader) (string, error) {
	//Read length
	var lenght int32
	err := binary.Read(reader, binary.BigEndian, lenght)
	if err != nil {
		return "", err
	}

	//Read data
	data := make([]byte, lenght)
	_, err = reader.Read(data)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

/*
ConvertStringToBytesDB converts string to bytes
*/
func ConvertStringToBytesDB(buffer *bytes.Buffer, data string) {
	//Write length
	binary.Write(buffer, binary.BigEndian, int32(len(data)))

	//Write data
	buffer.WriteString(data)
}

/*
LimitedStringDB is special data type for database that is string with maximum length, same as VARCHAR(n)
*/
type LimitedStringDB struct {
	data      string
	maxLength int32
}

/*
MakeLimitedStringDB creates new LimitedStringDB, if maxLangth is negative, than limit is not set
*/
func MakeLimitedStringDB(maxLength int32) LimitedStringDB {
	return LimitedStringDB{maxLength: maxLength, data: ""}
}

/*
Get gets string
*/
func (str *LimitedStringDB) Get() string {
	return str.data
}

/*
Set sets data, returns true when OK
*/
func (str *LimitedStringDB) Set(data string) bool {
	if str.maxLength >= 0 && len(data) > int(str.maxLength) {
		return false
	}
	str.data = data
	return true
}

/*
ConvertToBytesDB converts string to data
*/
func (str *LimitedStringDB) ConvertToBytesDB(buffer *bytes.Buffer) {
	if str.maxLength < 0 {
		ConvertStringToBytesDB(buffer, str.data)
		return
	}
	buffer.WriteString(str.data)
}

/*
ParseBytesDB parses bytes from reader to string
*/
func (str *LimitedStringDB) ParseBytesDB(reader io.Reader) error {
	if str.maxLength < 0 {
		var err error
		str.data, err = ParseStringDB(reader)
		return err
	}
	data := make([]byte, str.maxLength)
	_, err := reader.Read(data)
	str.data = string(data)
	return err
}
