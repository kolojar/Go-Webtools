/*
Package database provides some databases for some kinds of use cases
Please keep in mind that these databases are really simple
*/
package database

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"time"
	"webtools/encryption"
)

/*
IDatabaseObject adds support for databases to use this objects
*/
type IDatabaseObject interface {
	ConvertToBytesDB(writer io.Writer) error
	ParseBytesDB(reader io.Reader) error
}

/*
ConvertStringToBytesDB converts string to bytes
*/
func ConvertStringToBytesDB(writer io.Writer, data string) {
	//Write length
	ConvertUint64ToBytesDB(writer, uint64(len(data)))

	//Write data
	writer.Write([]byte(data))
}

/*
ParseStringDB parses bytes from reader to string
*/
func ParseStringDB(reader io.Reader) (string, error) {
	//Read length
	lenght, err := ParseUint64DB(reader)
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
ConvertUint64ToBytesDB converts uint64 to bytes
*/
func ConvertUint64ToBytesDB(writer io.Writer, data uint64) {
	//Write number
	dataByte := make([]byte, 8)
	binary.BigEndian.PutUint64(dataByte, data)
	writer.Write(dataByte)
}

/*
ParseUint64DB parses bytes from reader to uint64
*/
func ParseUint64DB(reader io.Reader) (uint64, error) {
	//Read number
	dataByte := make([]byte, 8)
	_, err := reader.Read(dataByte)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(dataByte), nil
}

///*
//LimitedStringDB is special data type for database that is string with maximum length, same as VARCHAR(n)
//*/
//type LimitedStringDB struct {
//	data      string
//	maxLength int32
//}
//
///*
//MakeLimitedStringDB creates new LimitedStringDB, if maxLangth is negative, than limit is not set
//*/
//func MakeLimitedStringDB(maxLength int32) LimitedStringDB {
//	return LimitedStringDB{maxLength: maxLength, data: ""}
//}
//
///*
//Get gets string
//*/
//func (str *LimitedStringDB) Get() string {
//	return str.data
//}
//
///*
//Set sets data, returns true when OK
//*/
//func (str *LimitedStringDB) Set(data string) bool {
//	if str.maxLength >= 0 && len(data) > int(str.maxLength) {
//		return false
//	}
//	str.data = data
//	return true
//}
//
///*
//ConvertToBytesDB converts string to data
//*/
//func (str *LimitedStringDB) ConvertToBytesDB(buffer *bytes.Buffer) error {
//	if str.maxLength < 0 {
//		ConvertStringToBytesDB(buffer, str.data)
//		return nil
//	}
//
//	buffer.WriteString()
//	return nil
//}
//
///*
//ParseBytesDB parses bytes from reader to string
//*/
//func (str *LimitedStringDB) ParseBytesDB(reader io.Reader) error {
//	if str.maxLength < 0 {
//		var err error
//		str.data, err = ParseStringDB(reader)
//		return err
//	}
//	data := make([]byte, str.maxLength)
//	_, err := reader.Read(data)
//	str.data = string(data)
//	return err
//}

/*
ParsePasswordObjectDB parses bytes from reader to PasswordObject
*/
func ParsePasswordObjectDB(reader io.Reader) (encryption.PasswordObject, error) {
	//Read salt
	var salt = make([]byte, 64)
	_, err := reader.Read(salt)
	if err != nil {
		return encryption.PasswordObject{}, err
	}

	//Read hash
	var hash = make([]byte, 64)
	_, err = reader.Read(hash)
	if err != nil {
		return encryption.PasswordObject{}, err
	}

	//Prepare salt
	saltString := hex.EncodeToString(salt)

	//Prepare hash
	hashString := hex.EncodeToString(hash)

	//Make object
	return encryption.PasswordObject{Salt: saltString, Hash: hashString}, nil
}

/*
ConvertPasswordObjectToBytesDB converts PasswordObject to bytes
*/
func ConvertPasswordObjectToBytesDB(writer io.Writer, data encryption.PasswordObject) error {
	//Prepare salt
	salt, err := hex.DecodeString(data.Salt)
	if err != nil {
		return err
	}
	if len(salt) != 64 {
		return errors.New("invalid salt size")
	}

	//Prepare hash
	hash, err := hex.DecodeString(data.Hash)
	if err != nil {
		return err
	}

	//Write salt
	writer.Write(salt)

	//Write hash
	writer.Write(hash)
	return nil
}

/*
ConvertBoolToBytesDB converts bool to bytes
*/
func ConvertBoolToBytesDB(writer io.Writer, data bool) {
	//Write bool
	dataByte := make([]byte, 1)
	dataByte[0] = 0
	if data {
		dataByte[0] = 1
	}
	writer.Write(dataByte)
}

/*
ParseBoolDB parses bytes from reader to bool
*/
func ParseBoolDB(reader io.Reader) (bool, error) {
	//Read bool
	dataByte := make([]byte, 1)
	_, err := reader.Read(dataByte)
	if err != nil {
		return false, err
	}
	return dataByte[0] == 1, nil
}

/*
ConvertTimeToBytesDB converts time to bytes
*/
func ConvertTimeToBytesDB(writer io.Writer, data time.Time) {
	//Write time
	ConvertUint64ToBytesDB(writer, uint64(data.UnixNano()))
}

/*
ParseTimeDB parses bytes from reader to time
*/
func ParseTimeDB(reader io.Reader) (time.Time, error) {
	//Read time
	timeNum, err := ParseUint64DB(reader)
	if err != nil {
		return time.Unix(0, 0), err
	}
	return time.Unix(0, int64(timeNum)), nil
}
