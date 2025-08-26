package encryption

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
)

/*
Asymmectric encryption support using OAEP
*/
type AsymmetricEncryption struct {
	privateKey *rsa.PrivateKey
}

/*
Asymmectric signed data, signature in Base64 format
*/
type AsymmetricSignedData struct {
	Data      []byte
	Signature string
}

/*
Creates new Asymmetric Encryption with new private and public key
*/
func NewAsymmetricEncryption() (*AsymmetricEncryption, error) {
	//Generate key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &AsymmetricEncryption{privateKey: privateKey}, nil
}

/*
Encrypts data using destination Public Key
Label is used for providing context for data, must be same in decryption
*/
func (enc *AsymmetricEncryption) EncryptWithLabel(data []byte, label []byte, destinationPublicKey *rsa.PublicKey) ([]byte, error) {
	// Encrypt
	hash := sha256.New()
	return rsa.EncryptOAEP(hash, rand.Reader, destinationPublicKey, data, label)
}

/*
Encrypts data using destination Public Key
*/
func (enc *AsymmetricEncryption) Encrypt(data []byte, destinationPublicKey *rsa.PublicKey) ([]byte, error) {
	return enc.EncryptWithLabel(data, []byte(""), destinationPublicKey)
}

/*
Decrypts data using local Private Key
Label is used for providing context for data, must be same in encryption
*/
func (enc *AsymmetricEncryption) DecryptWithLabel(data []byte, label []byte) ([]byte, error) {
	// Decrypt
	hash := sha256.New()
	return rsa.DecryptOAEP(hash, rand.Reader, enc.privateKey, data, label)
}

/*
Decrypts data using local Private Key
*/
func (enc *AsymmetricEncryption) Decrypt(data []byte) ([]byte, error) {
	return enc.DecryptWithLabel(data, []byte(""))
}

/*
Signs data using local Private Key
*/
func (enc *AsymmetricEncryption) Sign(data []byte) (*AsymmetricSignedData, error) {
	//Create hash
	hashData := sha256.Sum256(data)

	//Create signature
	signature, err := rsa.SignPSS(rand.Reader, enc.privateKey, crypto.SHA256, hashData[:], nil)
	if err != nil {
		return nil, err
	}

	//Return signature
	return &AsymmetricSignedData{Data: data, Signature: base64.StdEncoding.EncodeToString(signature)}, nil
}

/*
Verifies data using source Public Key
Returns original data if verification was successfull (nil error)
*/
func (enc *AsymmetricEncryption) Verify(signedData *AsymmetricSignedData, sourcePublicKey *rsa.PublicKey) ([]byte, error) {
	//Get signature
	signature, err := base64.StdEncoding.DecodeString(signedData.Signature)
	if err != nil {
		return nil, err
	}

	//Create hash
	hashData := sha256.Sum256(signedData.Data)

	//Create signature
	err = rsa.VerifyPSS(sourcePublicKey, crypto.SHA256, hashData[:], signature, nil)
	if err != nil {
		//Invalid signature
		return nil, err
	}

	//Return data
	return signedData.Data, nil
}
