/*
Package encryption provides encryption tools for symmetric and asymmetric encryption. Can encrypt and store passwords too.
*/
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

// timeoutLimitOfSignatureInMinutes tells how long will the signature be valid
const timeoutLimitOfSignatureInMinutes = 5

/*
Asymmetric provides asymmetric encryption support using OAEP
*/
type Asymmetric struct {
	privateKey        *rsa.PrivateKey
	encryptStoredKeys bool
	password          []byte
}

/*
GetPublicKey gets public key of assymetric encryption
*/
func (enc *Asymmetric) GetPublicKey() *rsa.PublicKey {
	return &enc.privateKey.PublicKey
}

/*
AsymmetricSignedData is signature in Base64 format with signed data
*/
type AsymmetricSignedData struct {
	Data      []byte
	Signature string
	Timestamp time.Time
	Expires   time.Time
}

/*
JSON creates json from this signed data
*/
func (data *AsymmetricSignedData) JSON() ([]byte, error) {
	return json.Marshal(data)
}

/*
ParseAsymmetricSignedData parses JSON to AsymmetricSignedData
*/
func ParseAsymmetricSignedData(jsonData []byte) (*AsymmetricSignedData, error) {
	var result *AsymmetricSignedData
	err := json.Unmarshal(jsonData, result)
	return result, err
}

/*
Creates new Asymmetric Encryption structure, should be used only in this class
*/
func newAsymmetricEncryptionStruct(encryptStoredKeys bool) (*Asymmetric, error) {
	if !encryptStoredKeys {
		// No encryption
		return &Asymmetric{password: nil}, nil
	}

	// Get password
	pass, err := webtools.ReadLineFromConsole("Enter password for keys: ")
	if err != nil {
		return nil, err
	}

	// Create struct
	return &Asymmetric{password: pass}, nil
}

/*
NewAsymmetricEncryption Creates new Asymmetric Encryption with new private and public key
*/
func NewAsymmetricEncryption(encryptStoredKeys bool) (*Asymmetric, error) {
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
ParsePublicKey parses public key encoded with PEM and stored in x509.PKIX format
*/
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

/*
ParsePrivateKey parses private key encoded with PEM and stored in x509.PKCS8 format
*/
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
LoadAsymmetricEncryption loads private and public key for Asymmetric encryption
*/
func LoadAsymmetricEncryption(encryptStoredKeys bool, privateKeyPath string, publicKeyPath string) (*Asymmetric, error) {
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
EncryptWithLabel encrypts data using destination Public Key
Label is used for providing context for data, must be same in decryption
*/
func (enc *Asymmetric) EncryptWithLabel(data []byte, label []byte, destinationPublicKey *rsa.PublicKey) ([]byte, error) {
	// Encrypt
	hash := sha256.New()
	return rsa.EncryptOAEP(hash, rand.Reader, destinationPublicKey, data, label)
}

/*
Encrypt encrypts data using destination Public Key
*/
func (enc *Asymmetric) Encrypt(data []byte, destinationPublicKey *rsa.PublicKey) ([]byte, error) {
	return enc.EncryptWithLabel(data, []byte(""), destinationPublicKey)
}

/*
DecryptWithLabel decrypts data using local Private Key
Label is used for providing context for data, must be same in encryption
*/
func (enc *Asymmetric) DecryptWithLabel(data []byte, label []byte) ([]byte, error) {
	// Decrypt
	hash := sha256.New()
	return rsa.DecryptOAEP(hash, rand.Reader, enc.privateKey, data, label)
}

/*
Decrypt decrypts data using local Private Key
*/
func (enc *Asymmetric) Decrypt(data []byte) ([]byte, error) {
	return enc.DecryptWithLabel(data, []byte(""))
}

/*
Sign signs data using local Private Key
*/
func (enc *Asymmetric) Sign(data []byte) (*AsymmetricSignedData, error) {
	// Make expiration
	timestamp := time.Now().UTC()
	expires := timestamp.Add(time.Minute * timeoutLimitOfSignatureInMinutes)

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
SignToJSON signs data using local Private Key directly to JSON format
*/
func (enc *Asymmetric) SignToJSON(data []byte) ([]byte, error) {
	//Sign
	signed, err := enc.Sign(data)
	if err != nil {
		return nil, err
	}

	//Json
	return signed.JSON()
}

/*
Verify cerifies data using source Public Key
Returns original data if verification was successfull (nil error)
*/
func (enc *Asymmetric) Verify(signedData *AsymmetricSignedData, sourcePublicKey *rsa.PublicKey) ([]byte, error) {
	// Get signature
	signature, err := base64.StdEncoding.DecodeString(signedData.Signature)
	if err != nil {
		return nil, err
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
VerifyFromJSON verifies data using source Public Key from JSON
Returns original data if verification was successfull (nil error)
*/
func (enc *Asymmetric) VerifyFromJSON(signedDataJSON []byte, sourcePublicKey *rsa.PublicKey) ([]byte, error) {
	//Decode JSON
	signed, err := ParseAsymmetricSignedData(signedDataJSON)
	if err != nil {
		return nil, err
	}

	//Verify
	return enc.Verify(signed, sourcePublicKey)
}

/*
EncodePublicKey encodes public key in 509.PKIX format and using PEM encoding
*/
func EncodePublicKey(publicKey *rsa.PublicKey) ([]byte, error) {
	// Encode public key
	pemPublicBlock, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	// Encode public key PEM
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pemPublicBlock}), nil
}

/*
EncodePrivateKey encodes private key in x509.PKCS8 format and using PEM encoding
*/
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
func (enc *Asymmetric) SaveAsymmetricEncryption(privateKeyPath string, publicKeyPath string) error {
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

/*
EncodePublicKey encodes public key using EncodePublicKey function
*/
func (enc *Asymmetric) EncodePublicKey() ([]byte, error) {
	return EncodePublicKey(&enc.privateKey.PublicKey)
}

/*
EncodePrivateKey encodes private key using EncodePrivateKey function
*/
func (enc *Asymmetric) EncodePrivateKey() ([]byte, error) {
	return EncodePrivateKey(enc.privateKey)
}
