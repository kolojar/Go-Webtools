package encryption

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"io"
)

type PasswordObject struct {
	Hash string // In hex format
	Salt string // In hex format
}

/*
Creates password hash in hexadecimal format
*/
func MakePasswordHash(plainPassword string, plainSalt string) (PasswordObject, error) {
	// Decode salt
	salt, err := hex.DecodeString(plainSalt)
	if err != nil {
		return PasswordObject{}, err
	}

	// Make hash
	hmacHash := hmac.New(sha512.New, salt)
	hmacHash.Write([]byte(plainPassword))
	return PasswordObject{Hash: hex.EncodeToString(hmacHash.Sum(nil)), Salt: plainSalt}, nil
}

/*
Checks password (automatically hashes)
*/
func (passwordObject PasswordObject) CheckPassword(plainPassword string) (bool, error) {
	obj, err := MakePasswordHash(plainPassword, passwordObject.Salt)
	if err != nil {
		return false, err
	}
	return passwordObject.CheckPasswordHash(obj.Hash)
}

/*
Checks password hash
*/
func (passwordObject PasswordObject) CheckPasswordHash(passwordHash string) (bool, error) {
	// Decode hex format
	hash1, err := hex.DecodeString(passwordHash)
	if err != nil {
		return false, err
	}
	hash2, err := hex.DecodeString(passwordObject.Hash)
	if err != nil {
		return false, err
	}
	return hmac.Equal(hash1, hash2), nil
}

/*
Generates salt for password hashing in hexadecimal format
Use 32 or 64 byte salt
*/
func GeneratePasswordSalt(lenght int) (string, error) {
	salt := make([]byte, lenght)
	_, err := io.ReadFull(rand.Reader, salt)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(salt), nil
}
