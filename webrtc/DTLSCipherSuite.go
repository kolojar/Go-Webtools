package webrtc

import (
	"bytes"
	"crypto"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
	"webtools/database"
)

// const TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384 DTLSCipherSuite = 0xC02C
//const TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 DTLSCipherSuite = 0xC02F
//const TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256 DTLSCipherSuite = 0xCCA9

// Supported Key Exchange Methods
type DTLSKeyExchangeMethod uint8

const ECDHE_EXCHANGE_METHOD DTLSKeyExchangeMethod = 1

// Supported Key Authentication Methods
type DTLSKeyAuthenticationMethod uint8

const ECDSA_AUTHENTICATION_METHOD DTLSKeyAuthenticationMethod = 1

// Supported Curves - Specification: https://datatracker.ietf.org/doc/html/rfc4492#section-5.4
type DTLSEECCurveType uint8

const ECC_NAMED_CURVE DTLSEECCurveType = 3

// Supported Named Curves - Specification: https://datatracker.ietf.org/doc/html/rfc4492#section-5.1.1
type DTLSEECCurveName uint16

func (curveName DTLSEECCurveName) GetEECCurve() ecdh.Curve {
	if curveName == ECC_CURVE_NAME_SECP256r1 {
		return ecdh.P256()
	} else {
		panic("unimplemented EEC curve")
	}
}

const ECC_CURVE_NAME_SECP256r1 DTLSEECCurveName = 23

// Supported Hash Algorithms - Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.1.4.1
type DTLSHashAlgorithm uint8

const SHA256HashAlgorithm DTLSHashAlgorithm = 4

func (algorithm DTLSHashAlgorithm) GetCurve() elliptic.Curve {
	if algorithm == SHA256HashAlgorithm {
		return elliptic.P256()
	} else {
		panic("unimplemented Hash curve")
	}
}

// Supported Signature Algorithms - Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.1.4.1
type DTLSSignatureAlgorithm uint8

const ECDSASignatureAlgorithm DTLSSignatureAlgorithm = 3

// DTLSKeyExchangeMethod, DTLSKeyAuthenticationMethod, EncryptionMethod (not implemented yet), CurveName, HashAlgorithm
// Supported Algorithms
type DTLSCipherSuite uint16

const TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256 DTLSCipherSuite = 0xC02B

func (suite DTLSCipherSuite) GetSuiteConfig() (DTLSKeyExchangeMethod, DTLSKeyAuthenticationMethod, DTLSEECCurveName, DTLSHashAlgorithm, error) {
	switch suite {
	case TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:
		return ECDHE_EXCHANGE_METHOD, ECDSA_AUTHENTICATION_METHOD, ECC_CURVE_NAME_SECP256r1, SHA256HashAlgorithm, nil
	default:
		return 0, 0, 0, 0, errors.New("invalid cipherSuite: " + strconv.FormatUint(uint64(suite), 10))
	}
}

type DTLSCertificate struct {
	privateKey      crypto.Signer
	certificateData []byte
	certificate     *x509.Certificate
	fingerprint     string
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc8122#section-5
Some help for generating certificates from Gemini
Returns fingerprint, certificate, certificateData, error
*/
func (cipher DTLSCipherSuite) GenerateDTLSCertificate(commonName string, notBefore time.Time, notAfter time.Time) (certificate *DTLSCertificate, err error) {
	//Get info
	var privateKey crypto.Signer
	_, authenticationMethod, _, hashAlgorithm, err := cipher.GetSuiteConfig()
	if err != nil {
		return nil, err
	}

	//Generate private key
	switch authenticationMethod {
	case ECDSA_AUTHENTICATION_METHOD:
		{
			//All ECDSA
			privateKey, err = ecdsa.GenerateKey(hashAlgorithm.GetCurve(), rand.Reader)
			if err != nil {
				return nil, err
			}
		}
	//case TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:
	//	{ //All RSA TODO
	//		privateKey, err = rsa.GenerateKey(rand.Reader, 1024)
	//		if err != nil {
	//			return nil, err
	//		}
	//	}
	default:
		{
			panic("Unknown authenticationMethod: " + strconv.FormatUint(uint64(authenticationMethod), 10))
		}
	}

	//switch algorithm {
	//case TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:
	//	privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	//	if err != nil {
	//		return nil, err
	//	}
	//
	//case TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:
	//	privateKey, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	//	if err != nil {
	//		return nil, err
	//	}
	//}

	//Generate random serial number
	serialNumber, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		return nil, err
	}

	//Generate certificate template
	certificateTemplate := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	//Create certificateData
	certificateData, err := x509.CreateCertificate(rand.Reader, &certificateTemplate, &certificateTemplate, privateKey.Public(), privateKey)
	if err != nil {
		return nil, err
	}

	//Create certificate from data
	xCertificate, err := x509.ParseCertificate(certificateData)
	if err != nil {
		return nil, err
	}

	//Create tls certificate
	//tlsCertificate := tls.Certificate{
	//	Certificate: [][]byte{certificateData},
	//	PrivateKey:  privateKey,
	//	Leaf:        certificate,
	//}

	//Generate SHA256 for fingerprint
	certificateSum := sha256.Sum256(certificateData)
	fmt.Println(certificateSum)
	result := ""
	for i := 0; i < len(certificateSum); i++ {
		result += strings.ToUpper(strconv.FormatUint(uint64(certificateSum[i]), 16)) + ":"
	}
	result = strings.TrimSuffix(result, ":")
	return &DTLSCertificate{privateKey: privateKey, certificateData: certificateData, certificate: xCertificate, fingerprint: result}, nil
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.1.4.1
Specification: https://datatracker.ietf.org/doc/html/rfc4492#section-5.4
Type: 12
*/
type DTLSServerKeyExchangeECDHE struct {
	CurveType          DTLSEECCurveType       //Set from SetFromCipherSuite
	CurveName          DTLSEECCurveName       //Set from SetFromCipherSuite
	PublicKey          []byte                 //Set from GenerateKey
	HashAlgorithm      DTLSHashAlgorithm      //Set from SetFromCipherSuite
	SignatureAlgorithm DTLSSignatureAlgorithm //Set from Sign
	Signature          []byte                 //Generated in Sign
}

func (keyExchange *DTLSServerKeyExchangeECDHE) SetFromCipherSuite(suite DTLSCipherSuite) (err error) {
	keyExchange.CurveType = ECC_NAMED_CURVE
	_, _, keyExchange.CurveName, keyExchange.HashAlgorithm, err = suite.GetSuiteConfig()
	return err
}

func (keyExchange *DTLSServerKeyExchangeECDHE) GenerateKey() (ephemeralPrivateKey *ecdh.PrivateKey, err error) {
	ephemeralPrivateKey, err = keyExchange.CurveName.GetEECCurve().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	keyExchange.PublicKey = ephemeralPrivateKey.PublicKey().Bytes()
	return ephemeralPrivateKey, nil
}

func (keyExchange *DTLSServerKeyExchangeECDHE) Sign(conn *DTLSServerConn, serverPrivateKey *ecdsa.PrivateKey) (err error) {
	keyExchange.SignatureAlgorithm = ECDSASignatureAlgorithm //Only one for now
	hash := sha256.New()

	//Put ClientRandom
	hash.Write(conn.clientRandom[:])

	//Put ServerRandom
	hash.Write(conn.serverRandom[:])

	//Put CurveType
	err = database.AppendUint8(hash, uint8(keyExchange.CurveType))
	if err != nil {
		return err
	}

	//Put CurveName
	err = database.AppendUint16(hash, uint16(keyExchange.CurveName))
	if err != nil {
		return err
	}

	//Put PublicKey
	err = database.AppendByteArray(hash, 1, keyExchange.PublicKey, nil)

	//Sign ASN.1
	keyExchange.Signature, err = serverPrivateKey.Sign(rand.Reader, hash.Sum(nil), nil)
	return err
}

func (keyExchange DTLSServerKeyExchangeECDHE) MakeBytes() (result []byte, err error) {
	buffer := bytes.NewBuffer(make([]byte, 0))

	//Put CurveType
	err = database.AppendUint8(buffer, uint8(keyExchange.CurveType))
	if err != nil {
		return nil, err
	}

	//Put CurveName
	err = database.AppendUint16(buffer, uint16(keyExchange.CurveName))
	if err != nil {
		return nil, err
	}

	//Put PublicKey
	err = database.AppendByteArray(buffer, 1, keyExchange.PublicKey, nil)
	if err != nil {
		return nil, err
	}

	//Put HashAlgorithm
	err = database.AppendUint8(buffer, uint8(keyExchange.HashAlgorithm))
	if err != nil {
		return nil, err
	}

	//Put SignatureAlgorithm
	err = database.AppendUint8(buffer, uint8(keyExchange.SignatureAlgorithm))
	if err != nil {
		return nil, err
	}

	//Put Signature
	err = database.AppendByteArray(buffer, 2, keyExchange.Signature, nil)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (cipher DTLSCipherSuite) GenerateDTLSKeyExchange(conn *DTLSServerConn, serverCertificate *DTLSCertificate) (keyExchange any, err error) {
	exchangeMethod, _, _, _, err := cipher.GetSuiteConfig()
	if exchangeMethod == ECDHE_EXCHANGE_METHOD {
		//Make ECDHE
		keyExchange := DTLSServerKeyExchangeECDHE{}
		err = keyExchange.SetFromCipherSuite(cipher)
		if err != nil {
			return nil, err
		}
		conn.ephemeralPrivateKey, err = keyExchange.GenerateKey()
		if err != nil {
			return nil, err
		}
		err = keyExchange.Sign(conn, serverCertificate.privateKey.(*ecdsa.PrivateKey))
		if err != nil {
			return nil, err
		}
		return keyExchange, nil
	} else {
		panic("unknown method: " + strconv.FormatUint(uint64(exchangeMethod), 10))
	}
}
