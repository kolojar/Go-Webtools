package webrtc

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
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
}

func NewDTLSServerProcessor(dtlsVersion uint16, unknownPacketReadFunc udp.ServerReadFunc, windowSize int, reportTraffic bool) (*DTLSServerProcessor, error) {
	processor := &DTLSServerProcessor{
		processors:            webtools.MakeSafeMap[*udp.ServerConn, *DTLSProcessor](),
		cookieSeed:            [32]byte{},
		version:               dtlsVersion,
		unknownPacketReadFunc: unknownPacketReadFunc,
		windowSize:            windowSize,
		reportTraffic:         reportTraffic,
	}
	_, err := rand.Read(processor.cookieSeed[:])
	if err != nil {
		return nil, err
	}
	return processor, nil
}

func (processor *DTLSServerProcessor) generateCookieHash(clientIPPort string, random [32]byte) []byte {
	return hmac.New(sha256.New, processor.cookieSeed[:]).Sum(append([]byte(clientIPPort), random[:]...))
}

func (processor *DTLSServerProcessor) ReadDataAndProcess(conn *udp.ServerConn, data []byte, ended bool) error {
	//Get DTLS Processor
	dtlsProcessor := processor.processors.Get(conn)
	if dtlsProcessor == nil {
		dtlsProcessor = NewDTLSProcessor(processor.unknownPacketReadFunc, processor.windowSize, processor.reportTraffic)
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
		dtlsProcessor = NewDTLSProcessor(processor.unknownPacketReadFunc, processor.windowSize, processor.reportTraffic)
		processor.processors.Set(conn, dtlsProcessor)
	}

	//Process records
	for _, record := range records {
		if record.ContentType == DTLSRecordContentTypeHandshake {
			//Handshake
			handshake := record.Fragment.(DTLSHandshake)
			if handshake.HandshakeType == DTLSHandshakeTypeClientHello {
				//Client Hello
				clientHello := handshake.Body.(DTLSClientHello)
				cookie := processor.generateCookieHash(conn.Address.String(), clientHello.Random)
				if len(clientHello.Cookie) == 0 {
					//Empty Cookie - Send HelloVerifyRequest
					dtlsProcessor.AddWriteRecord(DTLSRecord{
						ContentType:     DTLSRecordContentTypeHandshake,
						ProtocolVersion: record.ProtocolVersion,
						Epoch:           record.Epoch,
						SequenceNumber:  0, //Set in runtime
						Fragment: DTLSHandshake{
							HandshakeType:   DTLSHandshakeTypeHelloVerifyRequest,
							MessageSequence: 0, //Set in runtime
							Body: DTLSHelloVerifyRequest{
								ServerVersion: processor.version,
								Cookie:        cookie,
							},
						},
					})
					continue
				}
			}
		}
	}

	//Send records
	err := dtlsProcessor.ProcessWriteSend(conn)
	return err
}
