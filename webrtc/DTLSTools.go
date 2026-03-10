package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
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
*/
func GenerateDTLSCertificate(commonName string, notBefore time.Time, notAfter time.Time) (string, error) {
	//Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", err
	}

	//Generate random serial number
	randBigInt, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		return "", err
	}

	//Generate certificate template
	certificateTemplate := x509.Certificate{
		SerialNumber: randBigInt,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}

	//Create certificate
	certificate, err := x509.CreateCertificate(rand.Reader, &certificateTemplate, &certificateTemplate, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", err
	}

	//Generate SHA256
	certificateSum := sha256.Sum256(certificate)
	fmt.Println(certificateSum)
	result := ""
	for i := 0; i < len(certificateSum); i++ {
		result += strings.ToUpper(strconv.FormatUint(uint64(certificateSum[i]), 16)) + ":"
	}
	result = strings.TrimSuffix(result, ":")
	return result, nil
}

func DecryptDTLS() {

}
