package encryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
)

/*
Encrypts data using specified key (symmetric encryption)
*/
func EncryptSymmetric(password []byte, dataToEncrypt []byte) ([]byte, error) {
	if password == nil {
		// NO password, return original
		return dataToEncrypt, nil
	}

	// Make key
	salt, _ := GeneratePasswordSalt(32)
	saltData, _ := hex.DecodeString(salt)
	key := generate32ByteKeyPBKDF2(password, saltData)

	// Pad text
	// plainText := []byte(padText(textToEncrypt))

	// Create Cipher (encryptor) and block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Pad text
	plainBytes := pkcs7Pad(dataToEncrypt, aes.BlockSize)

	// Generate IV
	iv := make([]byte, aes.BlockSize)
	_, err = io.ReadFull(rand.Reader, iv)
	if err != nil {
		return nil, err
	}

	// New cipherText
	// 32 = Salt lenght
	cipherText := make([]byte, len(plainBytes))
	if len(cipherText) < aes.BlockSize {
		return nil, errors.New("message to encrypt is too short")
	}

	// Encrypt
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherText, plainBytes)

	// Add salt + IV
	result := append(saltData, iv...)
	result = append(result, cipherText...)
	return result, nil
}

/*
Decrypts data using specified key (symmetric encryption)
*/
func DecryptSymmetric(password []byte, encryptedData []byte) ([]byte, error) {
	if password == nil {
		// NO password, return original
		return encryptedData, nil
	}

	// Check encrypted text lenght
	if len(encryptedData) < 48 {
		return nil, errors.New("ciphertext too short")
	}

	// Get salt + key
	salt := encryptedData[:32]
	key := generate32ByteKeyPBKDF2(password, salt)

	// Get initialization vector (iv) + remainder
	iv := encryptedData[32:48]
	cipherText := encryptedData[48:]

	// Create Cipher (encryptor) and block
	block, err2 := aes.NewCipher(key)
	if err2 != nil {
		return nil, err2
	}

	// Check lenght of blocks
	if len(cipherText)%aes.BlockSize != 0 {
		return nil, errors.New("message to decrypt is not in valid blocks " + strconv.FormatInt(int64(len(cipherText)%aes.BlockSize), 10))
	}

	// Decrypt
	mode := cipher.NewCBCDecrypter(block, iv)
	resultPadded := make([]byte, len(cipherText))
	mode.CryptBlocks(resultPadded, cipherText)

	result, err3 := pkcs7Unpad(resultPadded)
	if err3 != nil {
		return nil, err3
	}

	// Unpad
	return result, nil
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
func generate32ByteKeyPBKDF2(password []byte, salt []byte) []byte {
	// Create hmac from key
	h := hmac.New(sha256.New, password)

	// Write salt
	h.Write(salt)

	// Encoding constants (in more bit usage are changed, for 32 bit not needed to change)
	h.Write([]byte{
		byte(1 >> 24),
		byte(1 >> 16),
		byte(1 >> 8),
		byte(1),
	})
	currentSum := h.Sum(nil)
	resultSum := make([]byte, len(currentSum))
	copy(resultSum, currentSum)

	// Do iterations (for compatibility with JS)
	// 10000 = Constant, count of iterations
	for i := 1; i < 10000; i++ {
		h = hmac.New(sha256.New, password)
		h.Write(currentSum)
		currentSum = h.Sum(nil)
		// XOR
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
	// No data
	if len(data) == 0 {
		return nil, errors.New("data empty")
	}

	// Remove excess \x10 and keep last \x10
	for data[len(data)-1] == 16 && data[len(data)-2] <= 16 {
		data = data[:len(data)-1]
	}

	// Get padding
	padding := int(data[len(data)-1])
	if padding == 0 || padding > len(data) {
		return nil, errors.New("invalid padding")
	}

	return data[:len(data)-padding], nil
}
