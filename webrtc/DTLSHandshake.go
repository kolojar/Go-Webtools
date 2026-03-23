package webrtc

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"webtools"
	"webtools/database"
)

type DTLSHandshakeType uint8

const ClientHelloHType DTLSHandshakeType = 1
const HelloVerifyRequestHType DTLSHandshakeType = 3

// Supported Algorythms
type DTLSCipherSuite uint16

const TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384 DTLSCipherSuite = 0xC02C
const TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256 DTLSCipherSuite = 0xC02B
const TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 DTLSCipherSuite = 0xC02F

//const TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256 DTLSCipherSuite = 0xCCA9

type DTLSHandshakeFragmentProcessor struct {
	fragments []DTLSHandshakeFragment
	//Packet max size is 1200 bytes -> Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-3.2.3
	recievedBytes ReplayWindow[uint32]
	fragment      []byte
}

func MakeDTLSHandshakeFragmentProcessor(totalLength int) DTLSHandshakeFragmentProcessor {
	return DTLSHandshakeFragmentProcessor{fragments: make([]DTLSHandshakeFragment, 0), recievedBytes: MakeReplayWindow[uint32](webtools.CeilDivision(totalLength, 8)), fragment: make([]byte, totalLength)}
}

func (processor *DTLSHandshakeFragmentProcessor) Process(fragment DTLSHandshakeFragment) (isCompleteHandshake bool, handshake DTLSHandshake) {
	for i := uint32(0); i < fragment.FragmentLength; i++ {
		if processor.recievedBytes.ApplyWindowCheck(i + fragment.FragmentOffset) {
			//Put to fragment
			processor.fragment[fragment.FragmentOffset+i] = fragment.FragmentData[i]
		}
	}

	//Check if window complete
	if processor.recievedBytes.IsWindowFull(len(processor.fragment)) {
		return true, DTLSHandshake{HandshakeType: fragment.HandshakeType, MessageSequence: fragment.MessageSequence, Body: processor.fragment}
	}
	return false, DTLSHandshake{}
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.2
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.2
Type: 22
*/
type DTLSHandshakeFragment struct {
	HandshakeType   DTLSHandshakeType //Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.2
	Length          uint32            //uint24
	MessageSequence uint16
	FragmentOffset  uint32 //uint24
	FragmentLength  uint32 //uint24
	FragmentData    []byte
}

func UnpackDTLSHandshakeFragment(reader io.Reader) (handshake DTLSHandshakeFragment, err error) {
	handshake = DTLSHandshakeFragment{}

	//Read Handshake Type
	handshakeType, err := database.ReadUint8(reader)
	if err != nil {
		return handshake, err
	}
	handshake.HandshakeType = DTLSHandshakeType(handshakeType)

	//Read Length
	handshake.Length, err = database.ReadUint24(reader)
	if err != nil {
		return handshake, err
	}

	//Read Message Sequence
	handshake.MessageSequence, err = database.ReadUint16(reader)
	if err != nil {
		return handshake, err
	}

	//Read Fragment Offset
	handshake.FragmentOffset, err = database.ReadUint24(reader)
	if err != nil {
		return handshake, err
	}

	//Read Fragment Length
	//handshake.FragmentLength, err = database.ReadUint24(reader)
	//if err != nil {
	//	return handshake, err
	//}

	//Read Fragment Data
	//handshake.FragmentData = make([]byte, handshake.FragmentLength)
	//n, err := reader.Read(handshake.FragmentData)
	//if err != nil {
	//	return handshake, err
	//}
	//if int(handshake.FragmentLength) != n {
	//	return handshake, errors.New("handshake data too short - wants: " + strconv.Itoa(int(handshake.FragmentLength)) + " has: " + strconv.Itoa(n))
	//}

	//Read Fragment Data
	handshake.FragmentData, err = database.ReadByteArray(reader, 3, nil)
	if err != nil {
		return handshake, err
	}
	handshake.FragmentLength = uint32(len(handshake.FragmentData))

	//Check for remaining data
	afterData, err := io.ReadAll(reader)
	if err != nil {
		return handshake, err
	}
	if len(afterData) != 0 {
		return handshake, errors.New("data after fragment: " + hex.EncodeToString(afterData))
	}
	return handshake, nil
}

func (handshake DTLSHandshakeFragment) MakeBytes() (result []byte, err error) {
	buffer := bytes.NewBuffer(make([]byte, 0))

	//Put HandshakeType
	err = database.AppendUint8(buffer, uint8(handshake.HandshakeType))
	if err != nil {
		return nil, err
	}

	//Put HandshakeType
	err = database.AppendUint24(buffer, handshake.Length)
	if err != nil {
		return nil, err
	}

	//Put MessageSequence
	err = database.AppendUint16(buffer, handshake.MessageSequence)
	if err != nil {
		return nil, err
	}

	//Put FragmentOffset
	err = database.AppendUint24(buffer, handshake.FragmentOffset)
	if err != nil {
		return nil, err
	}

	//Put FragmentLength
	err = database.AppendUint24(buffer, handshake.FragmentLength)
	if err != nil {
		return nil, err
	}

	//Put FragmentData
	_, err = buffer.Write(handshake.FragmentData)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.2
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.2
Type: 22
*/
type DTLSHandshake struct {
	HandshakeType DTLSHandshakeType //Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.2
	//length          uint32 //uint24
	MessageSequence uint16
	Body            any
}

func (handshake *DTLSHandshake) MakeBodyBytes(MessageSequence uint16) (result []byte, err error) {
	//buffer := bytes.NewBuffer(make([]byte, 0))

	//Put HandshakeType
	//err = database.AppendUint8(buffer, handshake.HandshakeType)
	//if err != nil {
	//	return nil, err
	//}

	//Put MessageSequence
	handshake.MessageSequence = MessageSequence
	//err = database.AppendUint16(buffer, handshake.MessageSequence)
	//if err != nil {
	//	return nil, err
	//}

	//Convert Body
	var bodyBytes []byte
	if handshake.HandshakeType == HelloVerifyRequestHType {
		bodyBytes, err = handshake.Body.(DTLSHelloVerifyRequest).MakeBytes()
		if err != nil {
			return nil, err
		}
	}

	//Put Body
	//err = database.AppendByteArray(buffer, 3, bodyBytes, nil)
	//if err != nil {
	//	return nil, err
	//}
	return bodyBytes, nil
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.1
Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.1.2
Type: 1
*/
type DTLSClientHello struct {
	ClientVersion          uint16
	Random                 [32]byte
	SessionID              []byte
	Cookie                 []byte
	CipherSuites           []DTLSCipherSuite //Specification: https://datatracker.ietf.org/doc/html/rfc5246#appendix-A.5
	CompressionMethodsData []uint8           //Specification: https://datatracker.ietf.org/doc/html/rfc5246#appendix-A.4.1
	ExtensionsData         []byte            //Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.1.4
}

func UnpackDTLSClientHello(reader io.Reader) (clientHello DTLSClientHello, err error) {
	clientHello = DTLSClientHello{}

	//Read ClientVersion
	clientHello.ClientVersion, err = database.ReadUint16(reader)
	if err != nil {
		return clientHello, err
	}

	//Check ClientVersion
	if clientHello.ClientVersion != DTLSVersion10 && clientHello.ClientVersion != DTLSVersion12 {
		return clientHello, errors.New("invalid client hello client version: " + strconv.FormatUint(uint64(clientHello.ClientVersion), 10))
	}

	//Read Random
	clientHello.Random = [32]byte{}
	n, err := reader.Read(clientHello.Random[:])
	if err != nil {
		return clientHello, err
	}
	if n != len(clientHello.Random) {
		return clientHello, errors.New("invalid random data length - max: 32; got: " + strconv.Itoa(n))
	}

	//Read SessionID
	clientHello.SessionID, err = database.ReadByteArray(reader, 1, func(length uint64) (err error) {
		if length > 32 {
			return errors.New("invalid sessionId data length - max: 32; got: " + strconv.FormatUint(length, 10))
		}
		return nil
	})
	if err != nil {
		return clientHello, err
	}

	//Read Cookie
	clientHello.Cookie, err = database.ReadByteArray(reader, 1, nil)
	if err != nil {
		return clientHello, err
	}

	//Read CipherSuitesLenght (multiples of 2)
	cipherSuitesLength, err := database.ReadUint16(reader)
	if err != nil {
		return clientHello, err
	}
	if cipherSuitesLength%2 != 0 {
		return clientHello, errors.New("length of cipherSuites must be multiple of 2, got: " + strconv.FormatUint(uint64(cipherSuitesLength), 10))
	}

	//Parse CipherSuites
	clientHello.CipherSuites = make([]DTLSCipherSuite, cipherSuitesLength/2)
	for i := uint16(0); i < cipherSuitesLength; i += 2 {
		chipherSuiteData, err := database.ReadUint16(reader)
		if err != nil {
			return clientHello, err
		}
		clientHello.CipherSuites = append(clientHello.CipherSuites, DTLSCipherSuite(chipherSuiteData))
	}

	//Read CompressionMethods
	clientHello.CompressionMethodsData, err = database.ReadByteArray(reader, 1, nil)
	if err != nil {
		return clientHello, err
	}

	//Read Extensions
	clientHello.ExtensionsData, err = database.ReadByteArray(reader, 2, nil)
	if err != nil {
		return clientHello, err
	}
	return clientHello, nil
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.1
Type: 3
*/
type DTLSHelloVerifyRequest struct {
	ServerVersion uint16
	Cookie        []byte
}

func (request DTLSHelloVerifyRequest) MakeBytes() (result []byte, err error) {
	buffer := bytes.NewBuffer(make([]byte, 0))

	//Put ServerVersion
	err = database.AppendUint16(buffer, request.ServerVersion)
	if err != nil {
		return nil, err
	}

	//Put Cookie
	err = database.AppendByteArray(buffer, 1, request.Cookie, nil)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.2
Type: 11
*/
type DTLSCertificateData struct {
	//Length of all Certificates = 3 bytes, length of each certificate = 3 bytes, data
	Certificates [][]byte
}

func MakeDTLSCertificate(certificates ...[]byte) DTLSCertificateData {
	return DTLSCertificateData{
		Certificates: certificates,
	}
}

func (certificate DTLSCertificateData) MakeBytes() (result []byte, err error) {
	//Create complete fragment
	bufferFragment := bytes.NewBuffer(make([]byte, 0))
	for _, certificate := range certificate.Certificates {
		err = database.AppendByteArray(bufferFragment, 3, certificate, nil)
		if err != nil {
			return nil, err
		}
	}

	//Build result
	buffer := bytes.NewBuffer(make([]byte, 0))
	fragment := bufferFragment.Bytes()

	//Put TotalLength - 3 bytes
	err = database.AppendUint24(buffer, uint32(len(fragment)))
	if err != nil {
		return nil, err
	}

	//Put Fragment
	_, err = buffer.Write(fragment)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.1.3
Type: 2
*/
type DTLSServerHello struct {
	ServerVersion     uint16
	Random            [32]byte
	SessionID         []byte
	CipherSuite       DTLSCipherSuite
	CompressionMethod uint8
}

func (serverHello DTLSServerHello) MakeBytes() (result []byte, err error) {
	buffer := bytes.NewBuffer(make([]byte, 0))

	//Put ServerVersion
	err = database.AppendUint16(buffer, serverHello.ServerVersion)
	if err != nil {
		return nil, err
	}

	//Put Random
	_, err = buffer.Write(serverHello.Random[:])
	if err != nil {
		return nil, err
	}

	//Put SessionID
	err = database.AppendByteArray(buffer, 1, serverHello.SessionID, nil)
	if err != nil {
		return nil, err
	}

	//Put CipherSuite
	err = database.AppendUint16(buffer, uint16(serverHello.CipherSuite))
	if err != nil {
		return nil, err
	}

	//Put CompressionMethod
	err = database.AppendUint8(buffer, serverHello.CompressionMethod)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

///*
//Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.3
//*/
//type DTLSServerDHParams struct {
//	DiffieHellmanPrimeModulus      uint16
//	DiffieHellmanGenerator         uint16
//	DiffieHellmanServerPublicValue uint16
//}
//
///*
//Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.3
//*/
//type DTLSServerKeyExchangeDigitallySignedParams struct {
//	ClientRandom [32]byte
//	ServerRandom [32]byte
//	Params       DTLSServerDHParams
//	AreUsed      bool
//}
//
///*
//Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.3
//Type: 12
//*/
//type DTLSServerKeyExchange struct {
//	Params       DTLSServerDHParams
//	SignedParams DTLSServerKeyExchangeDigitallySignedParams
//}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.1.4.1
Specification: https://datatracker.ietf.org/doc/html/rfc4492#section-5.4
Type: 12
*/
type DTLSServerKeyExchange struct {
	CurveType     uint8
	CurveName     uint16
	PublicKey     []byte
	HashAlgorythm uint8
	SignAlgorythm uint8
	Signature     []byte
}
