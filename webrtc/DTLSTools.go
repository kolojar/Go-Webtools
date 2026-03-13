package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

/*
Specification: https://datatracker.ietf.org/doc/html/rfc8122#section-5
Some help for generating certificates from Gemini
Returns fingerprint, certificate, error
*/
func GenerateDTLSCertificate(commonName string, notBefore time.Time, notAfter time.Time) (string, tls.Certificate, error) {
	//Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", tls.Certificate{}, err
	}

	//Generate random serial number
	serialNumber, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		return "", tls.Certificate{}, err
	}

	//Generate certificate template
	certificateTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}

	//Create certificateData
	certificateData, err := x509.CreateCertificate(rand.Reader, &certificateTemplate, &certificateTemplate, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", tls.Certificate{}, err
	}

	//Create certificate from data
	certificate, err := x509.ParseCertificate(certificateData)
	if err != nil {
		return "", tls.Certificate{}, err
	}

	//Create tls certificate
	tlsCertificate := tls.Certificate{
		Certificate: [][]byte{certificateData},
		PrivateKey:  privateKey,
		Leaf:        certificate,
	}

	//Generate SHA256 for fingerprint
	certificateSum := sha256.Sum256(certificateData)
	fmt.Println(certificateSum)
	result := ""
	for i := 0; i < len(certificateSum); i++ {
		result += strings.ToUpper(strconv.FormatUint(uint64(certificateSum[i]), 16)) + ":"
	}
	result = strings.TrimSuffix(result, ":")
	return result, tlsCertificate, nil
}
