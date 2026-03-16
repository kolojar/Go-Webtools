package webrtc

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"webtools/database"
)

/*
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.1
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.1
*/
type DTLSPacket struct {
	Type            uint8
	ProtocolVersion uint16
	Epoch           uint16
	SequenceNumber  uint64 //uint48
	//Length          uint16
	FragmentHandshake DTLSHandshakePacket
}

func ParseDTLSPacket(reader io.Reader) (DTLSPacket, error) {
	packet := DTLSPacket{}
	var err error

	//1 byte Type
	packet.Type, err = database.ParseUint8DB(reader)
	if err != nil {
		return packet, err
	}

	//2 bytes Protocol Version
	packet.ProtocolVersion, err = database.ParseUint16DB(reader)
	if err != nil {
		return packet, err
	}

	//2 bytes Epoch
	packet.Epoch, err = database.ParseUint16DB(reader)
	if err != nil {
		return packet, err
	}

	//6 bytes Sequence Number
	packet.SequenceNumber, err = database.ParseUint48DB(reader)
	if err != nil {
		return packet, err
	}

	//2 bytes Length
	lenght, err := database.ParseUint16DB(reader)
	if err != nil {
		return packet, err
	}

	//Read Fragment
	limitedReader := io.LimitReader(reader, int64(lenght))

	//Sort type
	if packet.Type == 22 {
		//Handshake
		packet.FragmentHandshake, err = ParseDTLSHandshakePacket(limitedReader)
	} else {
		panic("unknown DTLS packet type:" + strconv.FormatUint(uint64(packet.Type), 10))
	}
	return packet, err
}

func (dtlsPacket DTLSPacket) BuildTLSPacket() []byte {
	//Make packet holder
	packet := bytes.NewBuffer(make([]byte, 0))

	//Write Type
	packet.WriteByte(dtlsPacket.Type)

	//Write Version
	if dtlsPacket.ProtocolVersion == 0xFEFD || dtlsPacket.ProtocolVersion == 0xFEFF {
		database.ConvertUint16ToBytesDB(packet, 0x0303)
	} else {
		panic("Unknown version of DTLS:" + strconv.FormatUint(uint64(dtlsPacket.ProtocolVersion), 16))
	}

	//Build Fragment
	var fragment []byte
	if dtlsPacket.Type == 22 {
		//Handshake
		fragment = dtlsPacket.FragmentHandshake.BuildTLSPacket()
	} else {
		panic("unknown DTLS packet type:" + strconv.FormatUint(uint64(dtlsPacket.Type), 10))
	}

	//Write Length
	database.ConvertUint16ToBytesDB(packet, uint16(len(fragment)))

	//Write Fragment
	packet.Write(fragment)
	return packet.Bytes()
}

func (original DTLSPacket) ParseTLSPacket(reader io.Reader, sequenceNumber uint64) (DTLSPacket, error) {
	packet := DTLSPacket{}
	var err error

	//1 byte Type
	packet.Type, err = database.ParseUint8DB(reader)
	if err != nil {
		return packet, err
	}

	//2 bytes Protocol Version
	packet.ProtocolVersion, err = database.ParseUint16DB(reader)
	if err != nil {
		return packet, err
	}

	//Pass 2 bytes Epoch
	packet.Epoch = original.Epoch

	//Pass 6 bytes Sequence Number
	packet.SequenceNumber = sequenceNumber

	//2 bytes Length
	lenght, err := database.ParseUint16DB(reader)
	if err != nil {
		return packet, err
	}

	//Read Fragment
	limitedReader := io.LimitReader(reader, int64(lenght))

	//Sort type
	if packet.Type == 22 {
		//Handshake
		packet.FragmentHandshake, err = original.FragmentHandshake.ParseTLSHandshakePacket(limitedReader)
	} else {
		panic("unknown DTLS packet type:" + strconv.FormatUint(uint64(packet.Type), 10))
	}
	r, e := io.ReadAll(limitedReader)
	fmt.Println("Left in limited reader:", r, e)
	return packet, err
}

func (dtlsPacket DTLSPacket) BuildDTLSPacket() []byte {
	//Make packet holder
	packet := bytes.NewBuffer(make([]byte, 0))

	//Write Type
	packet.WriteByte(dtlsPacket.Type)

	//Write Version
	database.ConvertUint16ToBytesDB(packet, dtlsPacket.ProtocolVersion)

	//Write Epoch
	database.ConvertUint16ToBytesDB(packet, dtlsPacket.Epoch)

	//Write Sequence Number
	database.ConvertUint48ToBytesDB(packet, dtlsPacket.SequenceNumber)

	//Build Fragment
	var fragment []byte
	if dtlsPacket.Type == 22 {
		//Handshake
		fragment = dtlsPacket.FragmentHandshake.BuildDTLSPacket()
	} else {
		panic("unknown DTLS packet type:" + strconv.FormatUint(uint64(dtlsPacket.Type), 10))
	}

	//Write Length
	database.ConvertUint16ToBytesDB(packet, uint16(len(fragment)))

	//Write Fragment
	packet.Write(fragment)
	return packet.Bytes()
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.2
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.2
*/
type DTLSHandshakePacket struct {
	//Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.2
	Type uint8
	//Length          uint32 //uint24
	MessageSequence      uint16
	MessageSequenceWrite uint16 //Edited on writes
	FragmentOffset       uint32 //uint24
	//FragmentLength  uint32 //uint24
	//Sorted using Type
	FragmentObject any
}

func ParseDTLSHandshakePacket(reader io.Reader) (DTLSHandshakePacket, error) {
	packet := DTLSHandshakePacket{}
	var err error

	//1 byte Type
	packet.Type, err = database.ParseUint8DB(reader)
	if err != nil {
		return packet, err
	}

	//3 bytes Length
	_, err = database.ParseUint24DB(reader)
	if err != nil {
		return packet, err
	}

	//2 bytes Message Sequence
	packet.MessageSequence, err = database.ParseUint16DB(reader)
	if err != nil {
		return packet, err
	}
	packet.MessageSequenceWrite = packet.MessageSequence

	//3 bytes Fragment offset
	packet.FragmentOffset, err = database.ParseUint24DB(reader)
	if err != nil {
		return packet, err
	}
	if packet.FragmentOffset != 0 {
		fmt.Println(packet.FragmentOffset)
		fmt.Println(io.ReadAll(reader))
		panic("packet fragment offset not 0, TODO: fix")
	}

	//3 bytes Fragment length
	fragmentLength, err := database.ParseUint24DB(reader)
	if err != nil {
		return packet, err
	}

	//Read Fragment
	limitedReader := io.LimitReader(reader, int64(fragmentLength))

	//Sort type
	if packet.Type == 1 {
		//Client Hello
		packet.FragmentObject, err = ParseDTLSClientHelloPacket(limitedReader)
	} else {
		panic("unknown DTLS Handshake type:" + strconv.FormatUint(uint64(packet.Type), 10))
	}

	return packet, err
}

func (dtlsPacket DTLSHandshakePacket) BuildTLSPacket() []byte {
	//Make packet holder
	packet := bytes.NewBuffer(make([]byte, 0))

	//Write Type
	packet.WriteByte(dtlsPacket.Type)

	//Write Data
	var fragment []byte
	if dtlsPacket.Type == 1 {
		//Client Hello
		fragment = dtlsPacket.FragmentObject.(DTLSClientHelloPacket).BuildTLSPacket()
	} else {
		panic("unknown DTLS Handshake type:" + strconv.FormatUint(uint64(dtlsPacket.Type), 10))
	}

	//Write Length
	database.ConvertUint24ToBytesDB(packet, uint32(len(fragment)))

	//Write Fragment
	packet.Write(fragment)
	return packet.Bytes()
}

func (original *DTLSHandshakePacket) ParseTLSHandshakePacket(reader io.Reader) (DTLSHandshakePacket, error) {
	packet := DTLSHandshakePacket{}
	var err error

	//1 byte Type
	packet.Type, err = database.ParseUint8DB(reader)
	if err != nil {
		return packet, err
	}

	//Not in TLS - ingoring: 3 bytes Length
	//Put 2 bytes Message Sequence
	packet.MessageSequenceWrite++
	packet.MessageSequence = packet.MessageSequenceWrite

	//Pass 3 bytes Fragment offset
	packet.FragmentOffset = original.FragmentOffset
	if packet.FragmentOffset != 0 {
		fmt.Println(packet.FragmentOffset)
		fmt.Println(io.ReadAll(reader))
		panic("packet fragment offset not 0, TODO: fix")
	}

	//3 bytes Fragment length
	fragmentLength, err := database.ParseUint24DB(reader)
	if err != nil {
		return packet, err
	}

	//Read Fragment
	limitedReader := io.LimitReader(reader, int64(fragmentLength))

	//Sort type
	if packet.Type == 2 {
		//Server Hello
		if original.Type != 1 {
			//Parent must be Client Hello
			return packet, errors.New("DTLS invalid parent - wants: 1, has: " + strconv.FormatUint(uint64(original.Type), 10))
		}
		packet.FragmentObject, err = original.FragmentObject.(DTLSClientHelloPacket).ParseTLSServerHelloPacket(limitedReader)
	} else if packet.Type == 11 {
		//Certificate
		packet.FragmentObject, err = ParseTLSCertificatePacket(reader)
	} else {
		panic("unknown DTLS Handshake type:" + strconv.FormatUint(uint64(packet.Type), 10))
	}
	r, e := io.ReadAll(limitedReader)
	fmt.Println("Left in limited reader:", r, e)
	return packet, err
}

func (dtlsPacket DTLSHandshakePacket) BuildDTLSPacket() []byte {
	//Make packet holder
	packet := bytes.NewBuffer(make([]byte, 0))

	//Write 1 byte Type
	packet.WriteByte(dtlsPacket.Type)

	//Convert Data
	var data []byte
	if dtlsPacket.Type == 2 {
		data = dtlsPacket.FragmentObject.(DTLSServerHelloPacket).BuildDTLSPacket()
	} else {
		panic("unknown DTLS Handshake type:" + strconv.FormatUint(uint64(dtlsPacket.Type), 10))
	}

	//Write 3 byte Length = 2 bytes for Message Sequence, 3 bytes for Fragment Offset, 3 bytes for Fragment Length and len for length of Data
	database.ConvertUint24ToBytesDB(packet, uint32(2+3+3+len(data)))

	//Write 2 byte Message Sequence
	database.ConvertUint16ToBytesDB(packet, dtlsPacket.MessageSequence)

	//Write 3 bytes Fragment Offset
	database.ConvertUint24ToBytesDB(packet, dtlsPacket.FragmentOffset)
	if dtlsPacket.FragmentOffset != 0 {
		fmt.Println(dtlsPacket.FragmentOffset)
		panic("packet fragment offset not 0, TODO: fix")
	}

	//Write 3 byte Fragment Length
	database.ConvertUint24ToBytesDB(packet, uint32(len(data)))

	//Write Fragment
	packet.Write(data)
	return packet.Bytes()
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.1
*/
type DTLSClientHelloPacket struct {
	ProtocolVersion uint16
	Random          [32]byte
	//SessionIdLength uint8
	SessionId []byte
	//CookieLength uint8
	Cookie []byte
	Data   []byte
}

func ParseDTLSClientHelloPacket(reader io.Reader) (DTLSClientHelloPacket, error) {
	packet := DTLSClientHelloPacket{}
	var err error

	//2 byte Protocol Version
	packet.ProtocolVersion, err = database.ParseUint16DB(reader)
	if err != nil {
		return packet, err
	}

	//32 bytes random
	packet.Random = [32]byte{}
	_, err = reader.Read(packet.Random[:])
	if err != nil {
		return packet, err
	}

	//1 byte Session ID length
	sessionIdLength, err := database.ParseUint8DB(reader)
	if err != nil {
		return packet, err
	}

	//X bytes Session ID
	packet.SessionId = make([]byte, sessionIdLength)
	_, err = reader.Read(packet.SessionId)
	if err != nil {
		return packet, err
	}

	//1 byte Cookie length
	cookieLength, err := database.ParseUint8DB(reader)
	if err != nil {
		return packet, err
	}

	//X byte Cookie
	packet.Cookie = make([]byte, cookieLength)
	_, err = reader.Read(packet.Data)
	if err != nil {
		return packet, err
	}

	//Rest data
	packet.Data, err = io.ReadAll(reader)
	return packet, err
}

func (dtlsPacket DTLSClientHelloPacket) BuildTLSPacket() []byte {
	//Make packet holder
	packet := bytes.NewBuffer(make([]byte, 0))

	//Put Version
	if dtlsPacket.ProtocolVersion == 0xFEFD || dtlsPacket.ProtocolVersion == 0xFEFF {
		database.ConvertUint16ToBytesDB(packet, 0x0303)
	} else {
		panic("Unknown version of DTLS:" + strconv.FormatUint(uint64(dtlsPacket.ProtocolVersion), 16))
	}

	//Put Random
	packet.Write(dtlsPacket.Random[:])

	//Put SessionIDLength
	database.ConvertUint8ToBytesDB(packet, uint8(len(dtlsPacket.SessionId)))

	//Put SessionID
	packet.Write(dtlsPacket.SessionId)

	//Put Data
	packet.Write(dtlsPacket.Data)
	return packet.Bytes()
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.1
*/
type DTLSServerHelloPacket struct {
	ProtocolVersion uint16
	Random          [32]byte
	//SessionIdLength uint8
	SessionId []byte
	//CookieLength uint8
	Cookie []byte
	Data   []byte
}

func (original DTLSClientHelloPacket) ParseTLSServerHelloPacket(reader io.Reader) (DTLSServerHelloPacket, error) {
	var err error
	dtlsPacket := DTLSServerHelloPacket{}

	//2 bytes Version
	dtlsPacket.ProtocolVersion, err = database.ParseUint16DB(reader)
	if err != nil {
		return dtlsPacket, err
	}
	if dtlsPacket.ProtocolVersion == 0x0303 {
		dtlsPacket.ProtocolVersion = 0xFEFD
	} else {
		panic("unknown TLS protocol version: " + strconv.FormatUint(uint64(dtlsPacket.ProtocolVersion), 16))
	}

	//32 bytes Random
	_, err = reader.Read(dtlsPacket.Random[:])
	if err != nil {
		return dtlsPacket, err
	}

	//1 byte Session ID length
	sessionIdLength, err := database.ParseUint8DB(reader)
	if err != nil {
		return dtlsPacket, err
	}

	//X bytes Session ID
	dtlsPacket.SessionId = make([]byte, sessionIdLength)
	_, err = reader.Read(dtlsPacket.SessionId)
	if err != nil {
		return dtlsPacket, err
	}

	//Pass Cookie
	dtlsPacket.Cookie = original.Cookie

	//Read Data
	dtlsPacket.Data, err = io.ReadAll(reader)
	return dtlsPacket, err
}

func (dtlsPacket DTLSServerHelloPacket) BuildDTLSPacket() []byte {
	//Make packet holder
	packet := bytes.NewBuffer(make([]byte, 0))

	//Put Version
	database.ConvertUint16ToBytesDB(packet, dtlsPacket.ProtocolVersion)

	//Put Random
	packet.Write(dtlsPacket.Random[:])

	//Put SessionIDLength
	database.ConvertUint8ToBytesDB(packet, uint8(len(dtlsPacket.SessionId)))

	//Put SessionID
	packet.Write(dtlsPacket.SessionId)

	//Put Cookie Length
	database.ConvertUint8ToBytesDB(packet, uint8(len(dtlsPacket.Cookie)))

	//Put Cookie
	packet.Write(dtlsPacket.Cookie)

	//Put Data
	packet.Write(dtlsPacket.Data)
	return packet.Bytes()
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.2
*/
type DTLSCertificatePacket struct {
	Certificate []byte
}

/*
Can be used for DTLS too
*/
func ParseTLSCertificatePacket(reader io.Reader) (DTLSCertificatePacket, error) {
	var err error
	packet := DTLSCertificatePacket{}

	//Read Certificate
	packet.Certificate, err = io.ReadAll(reader)
	return packet, err
}

/*
Can be used for TLS too
*/
func (dtlsPacket DTLSCertificatePacket) BuildDTLSPacket() []byte {
	return dtlsPacket.Certificate
}
