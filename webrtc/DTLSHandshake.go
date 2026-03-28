package webrtc

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"webtools"
	"webtools/database"
)

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4
*/
type DTLSHandshakeType uint8

const ClientHelloHType DTLSHandshakeType = 1
const HelloVerifyRequestHType DTLSHandshakeType = 3
const ServerHelloHType DTLSHandshakeType = 2
const CertificateHType DTLSHandshakeType = 11
const ServerKeyExchangeHType DTLSHandshakeType = 12
const ServerHelloDoneHType DTLSHandshakeType = 14
const ClientKeyExchangeHType DTLSHandshakeType = 16

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

func MakeDTLSHandshake(HandshakeType DTLSHandshakeType, Body any) DTLSHandshake {
	return DTLSHandshake{
		HandshakeType:   HandshakeType,
		MessageSequence: 0, //Set in runtime
		Body:            Body,
	}
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
	} else if handshake.HandshakeType == ServerHelloHType {
		bodyBytes, err = handshake.Body.(DTLSServerHello).MakeBytes()
		if err != nil {
			return nil, err
		}
	} else if handshake.HandshakeType == ServerKeyExchangeHType {
		//FIX TO GLOBAL, NOT ONLY ECDHE
		bodyBytes, err = handshake.Body.(DTLSServerKeyExchangeECDHE).MakeBytes()
		if err != nil {
			return nil, err
		}
	} else if handshake.HandshakeType == CertificateHType {
		bodyBytes, err = handshake.Body.(DTLSCertificateData).MakeBytes()
		if err != nil {
			return nil, err
		}
	} else if handshake.HandshakeType == ServerHelloDoneHType {
		//Do nothing
		bodyBytes = nil
	} else {
		panic("unknown handshake type: " + strconv.FormatUint(uint64(handshake.HandshakeType), 10))
	}

	//Put Body
	//err = database.AppendByteArray(buffer, 3, bodyBytes, nil)
	//if err != nil {
	//	return nil, err
	//}
	return bodyBytes, nil
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.1.4
*/
type DTLSHelloExtension struct {
	ExtensionType uint16
	ExtensionData []byte
}

func UnpackDTLSHelloExtension(reader io.Reader) (extension DTLSHelloExtension, err error) {
	extension = DTLSHelloExtension{}

	//Read ExtensionType
	extension.ExtensionType, err = database.ReadUint16(reader)
	if err != nil {
		return extension, err
	}

	//Read ExtensionData
	extension.ExtensionData, err = database.ReadByteArray(reader, 2, nil)
	return extension, err
}

func (extension DTLSHelloExtension) MakeBytes() (result []byte, err error) {
	buffer := bytes.NewBuffer(make([]byte, 0))

	//Put ExtensionType
	err = database.AppendUint16(buffer, extension.ExtensionType)
	if err != nil {
		return nil, err
	}

	//Put ExtensionData
	err = database.AppendByteArray(buffer, 2, extension.ExtensionData, nil)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
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
	Extensions             []DTLSHelloExtension
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

	//Read ExtensionData
	extensionData, err := database.ReadByteArray(reader, 2, nil)
	if err != nil {
		return clientHello, err
	}
	extensionDataBytes := bytes.NewReader(extensionData)

	//Read Extensions
	for extensionDataBytes.Len() > 0 {
		extension, err := UnpackDTLSHelloExtension(extensionDataBytes)
		if err != nil {
			return clientHello, err
		}
		clientHello.Extensions = append(clientHello.Extensions, extension)
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

func MakeDTLSCertificateData(certificates ...*DTLSCertificate) DTLSCertificateData {
	certificatesData := make([][]byte, 0)
	for _, v := range certificates {
		certificatesData = append(certificatesData, v.certificateData)
	}
	return DTLSCertificateData{
		Certificates: certificatesData,
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
	Extensions        []DTLSHelloExtension
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

	//Make Extensions
	extensionBuffer := make([]byte, 0)
	for _, v := range serverHello.Extensions {
		extensionBytes, err := v.MakeBytes()
		if err != nil {
			return nil, err
		}
		extensionBuffer = append(extensionBuffer, extensionBytes...)
	}

	//Put Extensions
	err = database.AppendByteArray(buffer, 2, extensionBuffer, nil)
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
https://datatracker.ietf.org/doc/html/rfc5246#appendix-A.2
Type: 20
*/
type DTLSChangeCipherSpecification struct {
	Type uint8
}

func UnpackDTLSChangeCipherSpecFragment(reader io.Reader) (cipherSpecification DTLSChangeCipherSpecification, err error) {
	cipherSpecification = DTLSChangeCipherSpecification{}

	//Read Type
	cipherSpecification.Type, err = database.ReadUint8(reader)
	return cipherSpecification, err
}