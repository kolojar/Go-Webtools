package webrtc

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

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
func GenerateDTLSCertificate(commonName string, notBefore time.Time, notAfter time.Time, cipher DTLSCipherSuite) (certificate *DTLSCertificate, err error) {
	//Generate private key
	var privateKey crypto.Signer
	switch cipher {
	case TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:
		privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, err
		}
	case TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:
		//RSA
		privateKey, err = rsa.GenerateKey(rand.Reader, 1024)
		if err != nil {
			return nil, err
		}
	case TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:
		privateKey, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		if err != nil {
			return nil, err
		}
	}

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
