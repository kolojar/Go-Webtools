package webrtc

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"webtools"
	"webtools/database"
	"webtools/udp"
	"webtools/udpastcp"
)

/*
DTLSConnectionProcessor is processor for UDP connections that tries to read DTLS data
*/
type DTLSConnectionProcessor struct {
	conns     webtools.SafeMap[*udp.ServerConn, *udpastcp.Conn]
	dtlsConns webtools.SafeMap[*udpastcp.Conn, *tls.Conn]
	config    *tls.Config
}

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
	Fragment []byte
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
	packet.Fragment = make([]byte, lenght)
	_, err = reader.Read(packet.Fragment)
	return packet, err
}

func (dtlsPacket *DTLSPacket) BuildTLSPacket() []byte {
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

	//Write Length
	database.ConvertUint16ToBytesDB(packet, uint16(len(dtlsPacket.Fragment)))

	//Write Fragment
	packet.Write(dtlsPacket.Fragment)
	return packet.Bytes()
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.2
*/
type DTLSHandshakePacket struct {
	Type uint8
	//Length          uint32 //uint24
	MessageSequence uint16
	FragmentOffset  uint32 //uint24
	//FragmentLength  uint32 //uint24
	Fragment []byte
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
	packet.Fragment = make([]byte, fragmentLength)
	_, err = reader.Read(packet.Fragment)
	return packet, err
}

func (dtlsPacket *DTLSHandshakePacket) BuildTLSPacket() []byte {
	//Make packet holder
	packet := bytes.NewBuffer(make([]byte, 0))

	//Write Type
	packet.WriteByte(dtlsPacket.Type)

	//Write Length
	database.ConvertUint24ToBytesDB(packet, uint32(len(dtlsPacket.Fragment)))

	//Write Data
	packet.Write(dtlsPacket.Fragment)
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

func (dtlsPacket *DTLSClientHelloPacket) BuildTLSPacket() []byte {
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

	//Put data
	packet.Write(dtlsPacket.Data)
	return packet.Bytes()
}

/*
NewServer creates new processor for UDP connections
*/
func NewDTLSConnectionProcessor(certificates []tls.Certificate) *DTLSConnectionProcessor {
	return &DTLSConnectionProcessor{conns: webtools.MakeSafeMap[*udp.ServerConn, *udpastcp.Conn](), dtlsConns: webtools.MakeSafeMap[*udpastcp.Conn, *tls.Conn](),
		config: &tls.Config{
			Rand:         rand.Reader,
			Certificates: certificates,
		}}
}

/*
Handles UDP Read for processor
*/
func (processor *DTLSConnectionProcessor) ProcessUDPConn(localAddress net.Addr, conn *udp.ServerConn, data []byte, ended bool) (tlsConn *tls.Conn, err error) {
	if !ended {
		//Get connection association
		var udpConn *udpastcp.Conn = processor.conns.Get(conn)
		if udpConn == nil {
			//No connection, create new
			udpConn = udpastcp.NewConn(localAddress, conn.Address, func(data []byte) (n int, err error) {
				//Write func
				fmt.Println(data)
				panic("todo dtls write")
				//return tlsConn.Write(data)
				//return conn.Send(data)
			}, func() error {
				//Close func
				processor.dtlsConns.Delete(processor.conns.Get(conn))
				processor.conns.Delete(conn)
				return conn.Close()
			}, true)
			processor.conns.Set(conn, udpConn)

			//Open TLS Conn
			tlsConn = tls.Server(udpConn, processor.config)
			processor.dtlsConns.Set(udpConn, tlsConn)
		}

		//Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.1
		//Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.2
		//Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.1
		//Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4
		//More info in file://./DTLS-Structure.md
		//Convert DTLS (1 byte type, 2 byte , data from 25) to TLS (1 byte type, 2 byte version, 2 bytes length)
		//newData := make([]byte, 5)
		//if data[0] == 0x16 {
		//	//Set content type
		//	newData[0] = 0x16
		//}
		//if data[1] == 0xFE {
		//	//Set protocol version A
		//	//newData[1] = 0x03
		//	data[25] = 0x03
		//}
		//if data[2] == 0xFF {
		//	//Set protocol version B
		//	//newData[2] = 0x03
		//	data[26] = 0x03
		//}
		////Skip 2 bytes for epoch and 3 for sequenceNumber and 4 bytes for 0 (offset maybe)
		//binary.BigEndian.PutUint16(newData[3:], binary.BigEndian.Uint16(data[11:13])-8)
		//newData = append(newData, data[13:17]...)
		//newData = append(newData, data[25:]...)
		//fmt.Println(data)
		//
		////Get rid of cookie - end of random (32 byte) at 41 byte
		//sessionIdLength := uint8(newData[42])
		//
		//fmt.Println(newData)
		//

		//Parse DTLS to TLS
		packet, err := ParseDTLSPacket(bytes.NewReader(data))
		if err != nil {
			panic(err.Error())
		}
		handshake, err := ParseDTLSHandshakePacket(bytes.NewReader(packet.Fragment))
		if err != nil {
			panic(err.Error())
		}
		hello, err := ParseDTLSClientHelloPacket(bytes.NewReader(handshake.Fragment))
		if err != nil {
			panic(err.Error())
		}
		handshake.Fragment = hello.BuildTLSPacket()
		packet.Fragment = handshake.BuildTLSPacket()
		dataBuld := packet.BuildTLSPacket()
		fmt.Println("Writing", dataBuld)

		//Process read
		err = udpConn.WriteToReadBuffer(dataBuld)
		if err != nil {
			return nil, err
		}
		return processor.dtlsConns.Get(udpConn), nil
	}
	return nil, nil
}
