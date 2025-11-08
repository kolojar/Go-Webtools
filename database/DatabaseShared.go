/*
Package database provides some databases for some kinds of use cases
Please keep in mind that these databases are really simple
*/
package database

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"webtools/encryption"
)

/*
IDatabaseObject adds support for databases to use this objects
*/
type IDatabaseObject interface {
	ConvertToBytesDB(buffer *bytes.Buffer) error
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
func ConvertPasswordObjectToBytesDB(buffer *bytes.Buffer, data encryption.PasswordObject) error {
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
	buffer.WriteString(string(salt))

	//Write hash
	buffer.WriteString(string(hash))
	return nil
}
