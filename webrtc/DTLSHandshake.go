package webrtc

import (
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"webtools"
	"webtools/database"
)

const DTLSHandshakeTypeClientHello = uint8(1)

type DTLSHandshakeFragmentProcessor struct {
	fragments []DTLSHandshakeFragment
	//Packet max size is 1500 bytes -> Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-3.2.3
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
	HandshakeType   uint8  //Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.2
	Length          uint32 //uint24
	MessageSequence uint16
	FragmentOffset  uint32 //uint24
	FragmentLength  uint32 //uint24
	FragmentData    []byte
}

func UnpackDTLSHandshakeFragment(reader io.Reader) (handshake DTLSHandshakeFragment, err error) {
	handshake = DTLSHandshakeFragment{}

	//Read Handshake Type
	handshake.HandshakeType, err = database.ReadUint8(reader)
	if err != nil {
		return handshake, err
	}

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

/*
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.2
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.2
Type: 22
*/
type DTLSHandshake struct {
	HandshakeType uint8 //Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.2
	//length          uint32 //uint24
	MessageSequence uint16
	Body            any
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
	CipherSuitesData       []byte //Specification: https://datatracker.ietf.org/doc/html/rfc5246#appendix-A.5
	CompressionMethodsData []byte //Specification: https://datatracker.ietf.org/doc/html/rfc5246#appendix-A.4.1
	ExtensionsData         []byte //Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.1.4
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

	//Read CipherSuites (multiples of 2)
	clientHello.CipherSuitesData, err = database.ReadByteArray(reader, 2, func(length uint64) (err error) {
		if length%2 != 0 {
			return errors.New("length of cipherSuites must be multiple of 2, got: " + strconv.FormatUint(length, 10))
		}
		return nil
	})
	if err != nil {
		return clientHello, err
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
