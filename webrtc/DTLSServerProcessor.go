package webrtc

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"webtools"
	"webtools/udp"
)

type DTLSServerProcessor struct {
	processors            webtools.SafeMap[*udp.ServerConn, *DTLSProcessor]
	cookieSeed            [32]byte
	version               uint16
	unknownPacketReadFunc udp.ServerReadFunc
	windowSize            int
	reportTraffic         bool
	resendCount           uint8
	certificates          map[*DTLSCertificate][]DTLSCipherSuite
}

func NewDTLSServerProcessor(dtlsVersion uint16, unknownPacketReadFunc udp.ServerReadFunc, windowSize int, resendCount uint8, certificates map[*DTLSCertificate][]DTLSCipherSuite, reportTraffic bool) (*DTLSServerProcessor, error) {
	processor := &DTLSServerProcessor{
		processors:            webtools.MakeSafeMap[*udp.ServerConn, *DTLSProcessor](),
		cookieSeed:            [32]byte{},
		version:               dtlsVersion,
		unknownPacketReadFunc: unknownPacketReadFunc,
		windowSize:            windowSize,
		reportTraffic:         reportTraffic,
		resendCount:           resendCount,
		certificates:          certificates,
	}
	_, err := rand.Read(processor.cookieSeed[:])
	if err != nil {
		return nil, err
	}
	return processor, nil
}

func (processor *DTLSServerProcessor) generateCookieHash(clientIPPort string, random [32]byte) []byte {
	h := hmac.New(sha256.New, processor.cookieSeed[:])
	h.Write([]byte(clientIPPort))
	h.Write(random[:])
	return h.Sum(nil)
}

func (processor *DTLSServerProcessor) ReadDataAndProcess(conn *udp.ServerConn, data []byte, ended bool) error {
	//Get DTLS Processor
	dtlsProcessor := processor.processors.Get(conn)
	if dtlsProcessor == nil {
		dtlsProcessor = NewDTLSProcessor(processor.unknownPacketReadFunc, processor.windowSize, processor.resendCount, processor.reportTraffic)
		processor.processors.Set(conn, dtlsProcessor)
	}

	//Process
	records, err := dtlsProcessor.ReadData(conn, data, ended)
	if err != nil {
		return err
	}
	return processor.ProcessServer(records, conn)
}

func (processor *DTLSServerProcessor) ProcessServer(records []DTLSRecord, conn *udp.ServerConn) error {
	//Get DTLS Processor
	dtlsProcessor := processor.processors.Get(conn)
	if dtlsProcessor == nil {
		dtlsProcessor = NewDTLSProcessor(processor.unknownPacketReadFunc, processor.windowSize, processor.resendCount, processor.reportTraffic)
		processor.processors.Set(conn, dtlsProcessor)
	}

	//Process records
	for _, record := range records {
		if record.ContentType == HandshakeCType {
			//Handshake
			handshake := record.Fragment.(DTLSHandshake)
			if handshake.HandshakeType == ClientHelloHType {
				//Client Hello
				dtlsProcessor.Logger.Log(1, "Got ClientHello")
				clientHello := handshake.Body.(DTLSClientHello)
				cookie := processor.generateCookieHash(conn.Address.String(), clientHello.Random)
				//fmt.Println(len(clientHello.Cookie))
				//fmt.Println(clientHello.Cookie)
				if len(clientHello.Cookie) == 0 {
					//Empty Cookie - Send HelloVerifyRequest
					dtlsProcessor.Logger.Log(1, "Sending HelloVerifyRequest")
					dtlsProcessor.AddWriteRecord(DTLSRecord{
						ContentType:     HandshakeCType,
						ProtocolVersion: record.ProtocolVersion,
						Epoch:           record.Epoch,
						SequenceNumber:  0, //Set in runtime
						Fragment: DTLSHandshake{
							HandshakeType:   HelloVerifyRequestHType,
							MessageSequence: 0, //Set in runtime
							Body: DTLSHelloVerifyRequest{
								ServerVersion: processor.version,
								Cookie:        cookie,
							},
						},
					})
					continue
				}

				//Normal ClientHello - check cookie
				if subtle.ConstantTimeCompare(cookie, clientHello.Cookie) == 0 {
					dtlsProcessor.Logger.Log(2, "Got invalid ClientHello cookie: got="+hex.EncodeToString(clientHello.Cookie)+"; wants="+hex.EncodeToString(cookie))
					continue
				} else {
					dtlsProcessor.Logger.Log(0, "Got valid ClientHello cookie: got="+hex.EncodeToString(clientHello.Cookie)+"; wants="+hex.EncodeToString(cookie))
				}

				//Valid ClientHello
				fmt.Print("[")
				for _, s := range clientHello.CipherSuites {
					fmt.Printf("%#x, ", s)
				}
				fmt.Println("]")

			} else {
				panic("unknown DTLS handshake type: " + strconv.FormatUint(uint64(handshake.HandshakeType), 10))
			}
		}
	}

	//Send records
	err := dtlsProcessor.ProcessWriteSend(conn)
	return err
}
