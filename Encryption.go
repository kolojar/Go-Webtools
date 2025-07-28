package webtools

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
)

type PasswordObject struct {
	Hash string //In hex format
	Salt string //In hex format
}

/*
Encrypts text using specified key
*/
func EncryptText(plainKey string, textToEncrypt string) (string, error) {
	//Make key
	salt, _ := GeneratePasswordSalt(32)
	saltData, _ := hex.DecodeString(salt)
	key := generate32ByteKeyPBKDF2(plainKey, saltData)

	//Pad text
	//plainText := []byte(padText(textToEncrypt))

	//Create Cipher (encryptor) and block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	//Pad text
	plainBytes := pkcs7Pad([]byte(textToEncrypt), aes.BlockSize)

	//Generate IV
	iv := make([]byte, aes.BlockSize)
	_, err = io.ReadFull(rand.Reader, iv)
	if err != nil {
		return "", err
	}

	//New cipherText
	// 32 = Salt lenght
	cipherText := make([]byte, len(plainBytes))
	if len(cipherText) < aes.BlockSize {
		return "", errors.New("message to encrypt is too short")
	}

	//Encrypt
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherText, plainBytes)

	//Add salt + IV
	result := append(saltData, iv...)
	result = append(result, cipherText...)
	return base64.StdEncoding.EncodeToString(result), nil
}

/*
Decrypts text using specified key
*/
func DecryptText(plainKey string, encryptedText string) (string, error) {
	//Decode encryptedData
	encryptedData, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	//Check encrypted text lenght
	if len(encryptedData) < 48 {
		return "", errors.New("ciphertext too short")
	}

	//Get salt + key
	salt := encryptedData[:32]
	key := generate32ByteKeyPBKDF2(plainKey, salt)

	//Get initialization vector (iv) + remainder
	iv := encryptedData[32:48]
	cipherText := encryptedData[48:]

	//Create Cipher (encryptor) and block
	block, err2 := aes.NewCipher(key)
	if err2 != nil {
		return "", err2
	}

	//Check lenght of blocks
	if len(cipherText)%aes.BlockSize != 0 {
		return "", errors.New("message to decrypt is not in valid blocks " + strconv.FormatInt(int64(len(cipherText)%aes.BlockSize), 10))
	}

	//Decrypt
	mode := cipher.NewCBCDecrypter(block, iv)
	resultPadded := make([]byte, len(cipherText))
	mode.CryptBlocks(resultPadded, cipherText)

	result, err3 := pkcs7Unpad(resultPadded)
	if err3 != nil {
		return "", err3
	}
	//Unpad
	return string(result), nil
}

/*
Generates 32 byte key from any string key
*/
//func generate32ByteKey(plainKey string) []byte {
//	hash := sha256.New()
//	hash.Write([]byte(plainKey))
//	return hash.Sum(nil)
//}

/*
Generates 32 byte key from any string key and from salt
*/
func generate32ByteKeyPBKDF2(plainKey string, salt []byte) []byte {
	//Create hmac from key
	password := []byte(plainKey)
	h := hmac.New(sha256.New, password)

	//Write salt
	h.Write(salt)

	//Encoding constants (in more bit usage are changed, for 32 bit not needed to change)
	h.Write([]byte{
		byte(1 >> 24),
		byte(1 >> 16),
		byte(1 >> 8),
		byte(1),
	})
	currentSum := h.Sum(nil)
	resultSum := make([]byte, len(currentSum))
	copy(resultSum, currentSum)

	//Do iterations (for compatibility with JS)
	// 10000 = Constant, count of iterations
	for i := 1; i < 10000; i++ {
		h = hmac.New(sha256.New, password)
		h.Write(currentSum)
		currentSum = h.Sum(nil)
		//XOR
		for j := 0; j < len(currentSum); j++ {
			resultSum[j] ^= currentSum[j]
		}
	}
	return resultSum
}

/*
Pads text to support encryption algorythm
*/
//func padText(text string) string {
//	text += "|" //Separator for added bytes
//
//	//Calculate needed bytes (letters)
//	neededBytes := (aes.BlockSize - len(text)%aes.BlockSize) % aes.BlockSize
//
//	//Convert to hex and add to end
//	byteText := strconv.FormatInt(int64(neededBytes), aes.BlockSize)
//	text += strings.Repeat(byteText, neededBytes)
//	return text
//}

/*
Make PKCS7 padding
*/
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

/*
Unpad PKCS7
*/
func pkcs7Unpad(data []byte) ([]byte, error) {
	//No data
	if len(data) == 0 {
		return nil, errors.New("data empty")
	}

	//Remove excess \x10 and keep last \x10
	for data[len(data)-1] == 16 && data[len(data)-2] <= 16 {
		data = data[:len(data)-1]
	}

	//Get padding
	padding := int(data[len(data)-1])
	if padding == 0 || padding > len(data) {
		return nil, errors.New("invalid padding")
	}

	return data[:len(data)-padding], nil
}

/*
Removes padding from text to support encryption algorythm
*/
//func unpadText(text string) string {
//	lastChar := text[len(text)-1]
//	for strings.HasSuffix(text, "\x10") {
//		text = strings.TrimSuffix(text, "\x10")
//	}
//	if lastChar != '|' {
//		charCount, _ := strconv.ParseInt(string(lastChar), aes.BlockSize, 32)
//		text = text[:len(text)-int(charCount)]
//	}
//	return strings.TrimSuffix(text, "|")
//}

/*
Creates password hash in hexadecimal format
*/
func MakePasswordHash(plainPassword string, plainSalt string) (PasswordObject, error) {
	//Decode salt
	salt, err := hex.DecodeString(plainSalt)
	if err != nil {
		return PasswordObject{}, err
	}

	//Make hash
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
	//Decode hex format
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
