package webrtc

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"net"
	"webtools"
	"webtools/udp"
	"webtools/udpastcp"
)

type DTLSConn struct {
	conn                    *tls.Conn
	sequenceNumbersPerEpoch webtools.SafeMap[uint16, uint64]
	originalPacket          DTLSPacket
}

func (conn *DTLSConn) GetConn() *tls.Conn {
	return conn.conn
}

/*
DTLSConnectionProcessor is processor for UDP connections that tries to read DTLS data
*/
type DTLSConnectionProcessor struct {
	conns     webtools.SafeMap[*udp.ServerConn, *udpastcp.Conn]
	dtlsConns webtools.SafeMap[*udpastcp.Conn, *DTLSConn]
	config    *tls.Config
}

/*
NewServer creates new processor for UDP connections
*/
func NewDTLSConnectionProcessor(certificates []tls.Certificate) *DTLSConnectionProcessor {
	return &DTLSConnectionProcessor{conns: webtools.MakeSafeMap[*udp.ServerConn, *udpastcp.Conn](), dtlsConns: webtools.MakeSafeMap[*udpastcp.Conn, *DTLSConn](),
		config: &tls.Config{
			Rand:         rand.Reader,
			Certificates: certificates,
		}}
}

/*
Handles UDP Read for processor
*/
func (processor *DTLSConnectionProcessor) ProcessUDPConn(localAddress net.Addr, conn *udp.ServerConn, data []byte, ended bool) (dtlsConn *DTLSConn, err error) {
	if !ended {
		//Get connection association
		var udpConn *udpastcp.Conn = processor.conns.Get(conn)
		if udpConn == nil {
			//No connection, create new
			udpConn = udpastcp.NewConn(localAddress, conn.Address, func(data []byte) (n int, err error) {
				//Write func
				fmt.Println(data)
				buffer := bytes.NewBuffer(data)

				//Parse all packets
				packets := make([]DTLSPacket, 0)
				for buffer.Len() > 0 {
					packet, err := dtlsConn.originalPacket.ParseTLSPacket(buffer, dtlsConn.sequenceNumbersPerEpoch.Get(dtlsConn.originalPacket.Epoch))
					if err != nil {
						return 0, err
					}
					packets = append(packets, packet)
				}
				dtlsConn.sequenceNumbersPerEpoch.Set(dtlsConn.originalPacket.Epoch, dtlsConn.sequenceNumbersPerEpoch.Get(dtlsConn.originalPacket.Epoch)+1)

				//Convert all packets
				dataToSend := bytes.NewBuffer(make([]byte, 0))
				for _, packet := range packets {
					dataToSend.Write(packet.BuildDTLSPacket())
				}

				//Send
				return udpConn.Write(dataToSend.Bytes())
			}, func() error {
				//Close func
				processor.dtlsConns.Delete(processor.conns.Get(conn))
				processor.conns.Delete(conn)
				return conn.Close()
			}, true)
			processor.conns.Set(conn, udpConn)

			//Open TLS Conn
			tlsConn := tls.Server(udpConn, processor.config)
			processor.dtlsConns.Set(udpConn, &DTLSConn{conn: tlsConn, sequenceNumbersPerEpoch: webtools.MakeSafeMap[uint16, uint64](), originalPacket: DTLSPacket{}})
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
		dtlsConn := processor.dtlsConns.Get(udpConn)
		dtlsConn.originalPacket, err = ParseDTLSPacket(bytes.NewReader(data))
		if err != nil {
			panic(err.Error())
		}
		tlsData := dtlsConn.originalPacket.BuildTLSPacket()
		fmt.Println("Writing", tlsData)

		//Process read
		err = udpConn.WriteToReadBuffer(tlsData)
		if err != nil {
			return nil, err
		}
		return dtlsConn, nil
	}
	return nil, nil
}
