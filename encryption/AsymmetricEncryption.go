package encryption

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"time"

	"webtools"
)

// TIMEOUT_LIMIT_OF_SIGNATURE_IN_MINUTES tells how long will the signature be valid
const TIMEOUT_LIMIT_OF_SIGNATURE_IN_MINUTES = 5
const ERR_NO_PUBLIC_KEY = "no public key specified"

/*
Asymmectric encryption support using OAEP
*/
type AsymmetricEncryption struct {
	privateKey        *rsa.PrivateKey
	encryptStoredKeys bool
	password          []byte
}

func (enc *AsymmetricEncryption) GetPublicKey() *rsa.PublicKey {
	return &enc.privateKey.PublicKey
}

/*
Asymmectric signed data, signature in Base64 format
*/
type AsymmetricSignedData struct {
	Data      []byte
	Signature string
	Timestamp time.Time
	Expires   time.Time
}

func (data *AsymmetricSignedData) Json() ([]byte, error) {
	return json.Marshal(data)
}

func ParseAsymmetricSignedData(jsonData []byte) (*AsymmetricSignedData, error) {
	var result *AsymmetricSignedData
	err := json.Unmarshal(jsonData, result)
	return result, err
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
Creates new Asymmetric Encryption with new private and public key
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

func ParsePublicKey(publicKeyData []byte) (*rsa.PublicKey, error) {
	// Decode public key PEM
	pemPublicBlock, _ := pem.Decode(publicKeyData)
	if pemPublicBlock == nil || pemPublicBlock.Type != "PUBLIC KEY" {
		return nil, errors.New("invalid private key PEM block")
	}

	//Parse public key
	publicKey, err := x509.ParsePKIXPublicKey(pemPublicBlock.Bytes)
	if err != nil {
		return nil, err
	}
	return publicKey.(*rsa.PublicKey), nil
}

func ParsePrivateKey(privateKeyData []byte) (*rsa.PrivateKey, error) {
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
	return privateKey.(*rsa.PrivateKey), nil
}

/*
Loads private and public key for Asymmetric Encryption
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

	// Decode public key
	publicKey, err := ParsePublicKey(publicKeyData)
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

	//Decode private key
	privateKey, err := ParsePrivateKey(privateKeyData)
	if err != nil {
		return nil, err
	}

	// Insert into object
	enc.privateKey = privateKey
	enc.privateKey.PublicKey = *publicKey

	return enc, nil
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
	// Make expiration
	timestamp := time.Now().UTC()
	expires := timestamp.Add(time.Minute * TIMEOUT_LIMIT_OF_SIGNATURE_IN_MINUTES)

	// Create hash
	dataForHasher := make([]byte, 0)
	dataForHasher = append(dataForHasher, []byte(timestamp.String())...)
	dataForHasher = append(dataForHasher, []byte(expires.String())...)
	dataForHasher = append(dataForHasher, data...)
	hashData := sha256.Sum256(dataForHasher)

	// Create signature
	signature, err := rsa.SignPSS(rand.Reader, enc.privateKey, crypto.SHA256, hashData[:], nil)
	if err != nil {
		return nil, err
	}

	// Return signature
	return &AsymmetricSignedData{Data: data, Timestamp: timestamp, Expires: expires, Signature: base64.StdEncoding.EncodeToString(signature)}, nil
}

/*
Signs data using local Private Key directly to JSON format
*/
func (enc *AsymmetricEncryption) SignToJson(data []byte) ([]byte, error) {
	//Sign
	signed, err := enc.Sign(data)
	if err != nil {
		return nil, err
	}

	//Json
	return signed.Json()
}

/*
Verifies data using source Public Key
Returns original data if verification was successfull (nil error)
*/
func (enc *AsymmetricEncryption) Verify(signedData *AsymmetricSignedData, sourcePublicKey *rsa.PublicKey) ([]byte, error) {
	// Get signature
	signature, err := base64.StdEncoding.DecodeString(signedData.Signature)
	if err != nil {
		return nil, err
	}

	//No source key
	if sourcePublicKey == nil {
		return signedData.Data, errors.New(ERR_NO_PUBLIC_KEY)
	}

	// Create hash
	dataForHasher := make([]byte, 0)
	dataForHasher = append(dataForHasher, []byte(signedData.Timestamp.String())...)
	dataForHasher = append(dataForHasher, []byte(signedData.Expires.String())...)
	dataForHasher = append(dataForHasher, signedData.Data...)
	hashData := sha256.Sum256(dataForHasher)

	// Verify signature
	err = rsa.VerifyPSS(sourcePublicKey, crypto.SHA256, hashData[:], signature, nil)
	if err != nil {
		// Invalid signature
		return nil, err
	}

	// Return data
	return signedData.Data, nil
}

/*
Verifies data using source Public Key from JSON
Returns original data if verification was successfull (nil error)
*/
func (enc *AsymmetricEncryption) VerifyFromJson(signedDataJson []byte, sourcePublicKey *rsa.PublicKey) ([]byte, error) {
	//Decode JSON
	signed, err := ParseAsymmetricSignedData(signedDataJson)
	if err != nil {
		return nil, err
	}

	//Verify
	return enc.Verify(signed, sourcePublicKey)
}

func EncodePublicKey(publicKey *rsa.PublicKey) ([]byte, error) {
	// Encode public key
	pemPublicBlock, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	// Encode public key PEM
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pemPublicBlock}), nil
}

func EncodePrivateKey(privateKey *rsa.PrivateKey) ([]byte, error) {
	// Encode private key
	pemPrivateBlock, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	// Encode private key PEM
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pemPrivateBlock}), nil
}

/*
SaveAsymmetricEncryption saves private and public key for Asymmetric Encryption
*/
func (enc *AsymmetricEncryption) SaveAsymmetricEncryption(privateKeyPath string, publicKeyPath string) error {
	//Encode public key data
	publicKeyData, err := EncodePublicKey(&enc.privateKey.PublicKey)
	if err != nil {
		return err
	}

	// Encrypt public key data
	publicKeyDataEnc, err := EncryptSymmetric(enc.password, publicKeyData)
	if err != nil {
		return err
	}

	// Save public key from file
	err = os.WriteFile(publicKeyPath, publicKeyDataEnc, 0600)
	if err != nil {
		return err
	}

	//Encode private key
	privateKeyData, err := EncodePrivateKey(enc.privateKey)
	if err != nil {
		return err
	}

	// Encrypt private key data
	privateKeyDataEnc, err := EncryptSymmetric(enc.password, privateKeyData)
	if err != nil {
		return err
	}

	// Save private key from file
	return os.WriteFile(privateKeyPath, privateKeyDataEnc, 0600)
}

func (enc *AsymmetricEncryption) EncodePublicKey() ([]byte, error) {
	return EncodePublicKey(&enc.privateKey.PublicKey)
}

func (enc *AsymmetricEncryption) EncodePrivateKey() ([]byte, error) {
	return EncodePrivateKey(enc.privateKey)
}
