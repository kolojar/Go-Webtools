package webrtc

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"net"
	"webtools"
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
*/
type DTLSConnn struct {
	ContentType     uint8
	ProtocolVersion uint16
	Epoch           uint16
	SequenceNumber  uint64
	Length          uint16
	TLSData         []byte
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
		newData := make([]byte, 5)
		if data[0] == 0x16 {
			//Set content type
			newData[0] = 0x16
		}
		if data[1] == 0xFE {
			//Set protocol version A
			//newData[1] = 0x03
			data[25] = 0x03
		}
		if data[2] == 0xFF {
			//Set protocol version B
			//newData[2] = 0x03
			data[26] = 0x03
		}
		//Skip 2 bytes for epoch and 3 for sequenceNumber and 4 bytes for 0 (offset maybe)
		binary.BigEndian.PutUint16(newData[3:], binary.BigEndian.Uint16(data[11:13])-8)
		newData = append(newData, data[13:17]...)
		newData = append(newData, data[25:]...)
		fmt.Println(data)

		//Get rid of cookie - end of random (32 byte) at 41 byte
		sessionIdLength := uint8(newData[42])

		fmt.Println(newData)

		//Process read
		err = udpConn.WriteToReadBuffer(newData)
		if err != nil {
			return nil, err
		}
		return processor.dtlsConns.Get(udpConn), nil
	}
	return nil, nil
}
