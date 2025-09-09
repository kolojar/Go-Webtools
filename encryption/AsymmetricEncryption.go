// Package encryption adds support for: symetric - AES + PKCS7Pad; asymetric encryption - OAEP + Signature; passwords - hmac hashes
package encryption

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"

	"webtools"
)

/*
AsymmetricEncryption support using OAEP
*/
type AsymmetricEncryption struct {
	privateKey        *rsa.PrivateKey
	encryptStoredKeys bool
	password          []byte
}

/*
AsymmetricSignedData signature in Base64 format
*/
type AsymmetricSignedData struct {
	Data      []byte
	Signature string
}

/*
* Creates new Asymmetric Encryption structure, should be used only in this class
 */
func newAsymmetricEncryptionStruct(encryptStoredKeys bool) (*AsymmetricEncryption, error) {
	if !encryptStoredKeys {
		// No encryption
		return &AsymmetricEncryption{password: nil}, nil
	}

	// Get password
	pass, err := webtools.ReadLineFromConsole("Enter password for keys: ")
	if err != nil {
		return nil, err
	}

	// Create struct
	return &AsymmetricEncryption{password: pass}, nil
}

/*
NewAsymmetricEncryption creates new Asymmetric Encryption with new private and public key
*/
func NewAsymmetricEncryption(encryptStoredKeys bool) (*AsymmetricEncryption, error) {
	// Get struct
	enc, err := newAsymmetricEncryptionStruct(encryptStoredKeys)
	if err != nil {
		return nil, err
	}

	// Generate key
	enc.privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return enc, nil
}

/*
LoadAsymmetricEncryption loads private and public key for Asymmetric Encryption
*/
func LoadAsymmetricEncryption(encryptStoredKeys bool, privateKeyPath string, publicKeyPath string) (*AsymmetricEncryption, error) {
	// Get struct
	enc, err := newAsymmetricEncryptionStruct(encryptStoredKeys)
	if err != nil {
		return nil, err
	}

	// Load public key from file
	publicKeyDataEnc, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, err
	}

	// Decrypt public key data
	publicKeyData, err := DecryptSymmetric(enc.password, publicKeyDataEnc)
	if err != nil {
		return nil, err
	}

	// Parse public key PEM
	pemPublicBlock, _ := pem.Decode(publicKeyData)
	if pemPublicBlock == nil || pemPublicBlock.Type != "PUBLIC KEY" {
		return nil, errors.New("invalid private key PEM block")
	}

	publicKey, err := x509.ParsePKIXPublicKey(pemPublicBlock.Bytes)
	if err != nil {
		return nil, err
	}

	// Load private key from file
	privateKeyDataEnc, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}

	// Decrypt private key data
	privateKeyData, err := DecryptSymmetric(enc.password, privateKeyDataEnc)
	if err != nil {
		return nil, err
	}

	// Parse private key PEM
	pemPrivateBlock, _ := pem.Decode(privateKeyData)
	if pemPrivateBlock == nil || pemPrivateBlock.Type != "RSA PRIVATE KEY" {
		return nil, errors.New("invalid private key PEM block")
	}

	// Decode private key
	privateKey, err := x509.ParsePKCS8PrivateKey(pemPrivateBlock.Bytes)
	if err != nil {
		return nil, err
	}

	// Insert into object
	enc.privateKey = privateKey.(*rsa.PrivateKey)
	enc.privateKey.PublicKey = publicKey.(rsa.PublicKey)

	return enc, nil
}

/*
EncryptWithLabel encrypts data using destination Public Key
Label is used for providing context for data, must be same in decryption
*/
func (enc *AsymmetricEncryption) EncryptWithLabel(data []byte, label []byte, destinationPublicKey *rsa.PublicKey) ([]byte, error) {
	// Encrypt
	hash := sha256.New()
	return rsa.EncryptOAEP(hash, rand.Reader, destinationPublicKey, data, label)
}

/*
Encrypt encrypts data using destination Public Key
*/
func (enc *AsymmetricEncryption) Encrypt(data []byte, destinationPublicKey *rsa.PublicKey) ([]byte, error) {
	return enc.EncryptWithLabel(data, []byte(""), destinationPublicKey)
}

/*
DecryptWithLabel decrypts data using local Private Key
Label is used for providing context for data, must be same in encryption
*/
func (enc *AsymmetricEncryption) DecryptWithLabel(data []byte, label []byte) ([]byte, error) {
	// Decrypt
	hash := sha256.New()
	return rsa.DecryptOAEP(hash, rand.Reader, enc.privateKey, data, label)
}

/*
Decrypt decrypts data using local Private Key
*/
func (enc *AsymmetricEncryption) Decrypt(data []byte) ([]byte, error) {
	return enc.DecryptWithLabel(data, []byte(""))
}

/*
Sign signs data using local Private Key
*/
func (enc *AsymmetricEncryption) Sign(data []byte) (*AsymmetricSignedData, error) {
	// Create hash
	hashData := sha256.Sum256(data)

	// Create signature
	signature, err := rsa.SignPSS(rand.Reader, enc.privateKey, crypto.SHA256, hashData[:], nil)
	if err != nil {
		return nil, err
	}

	// Return signature
	return &AsymmetricSignedData{Data: data, Signature: base64.StdEncoding.EncodeToString(signature)}, nil
}

/*
Verify verifies data using source Public Key
Returns original data if verification was successfull (nil error)
*/
func (enc *AsymmetricEncryption) Verify(signedData *AsymmetricSignedData, sourcePublicKey *rsa.PublicKey) ([]byte, error) {
	// Get signature
	signature, err := base64.StdEncoding.DecodeString(signedData.Signature)
	if err != nil {
		return nil, err
	}

	// Create hash
	hashData := sha256.Sum256(signedData.Data)

	// Create signature
	err = rsa.VerifyPSS(sourcePublicKey, crypto.SHA256, hashData[:], signature, nil)
	if err != nil {
		// Invalid signature
		return nil, err
	}

	// Return data
	return signedData.Data, nil
}

/*
SaveAsymmetricEncryption saves private and public key for Asymmetric Encryption
*/
func (enc *AsymmetricEncryption) SaveAsymmetricEncryption(privateKeyPath string, publicKeyPath string) error {
	// Encode public key
	pemPublicBlock, err := x509.MarshalPKIXPublicKey(enc.privateKey.PublicKey)
	if err != nil {
		return err
	}

	// Encode public key PEM'
	publicKeyData := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pemPublicBlock})

	// Encrypt public key data
	publicKeyDataEnc, err := EncryptSymmetric(enc.password, publicKeyData)
	if err != nil {
		return err
	}

	// Save public key from file
	err = os.WriteFile(publicKeyPath, publicKeyDataEnc, 0o600)
	if err != nil {
		return err
	}

	// Encode private key
	pemPrivateBlock, err := x509.MarshalPKCS8PrivateKey(enc.privateKey)
	if err != nil {
		return err
	}

	// Encode private key PEM'
	privateKeyData := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pemPrivateBlock})

	// Encrypt private key data
	privateKeyDataEnc, err := EncryptSymmetric(enc.password, privateKeyData)
	if err != nil {
		return err
	}

	// Save public key from file
	return os.WriteFile(privateKeyPath, privateKeyDataEnc, 0o600)
}
