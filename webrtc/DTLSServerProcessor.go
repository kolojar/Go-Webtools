package webrtc

import (
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"webtools"
	"webtools/udp"
)

type DTLSServerConn struct {
	processor           *DTLSProcessor
	ephemeralPrivateKey *ecdh.PrivateKey
	clientRandom        [32]byte
	serverRandom        [32]byte
	cipherSuite         DTLSCipherSuite
	conn                *udp.ServerConn
	//clientPublicKey     *ecdh.PublicKey
}

type DTLSServerProcessor struct {
	cookieSeed            [32]byte
	version               uint16
	unknownPacketReadFunc udp.ServerReadFunc
	windowSize            int
	reportTraffic         bool
	resendCount           uint8
	certificates          map[DTLSCipherSuite]*DTLSCertificate
	connections           webtools.SafeMap[*udp.ServerConn, *DTLSServerConn]
}

func NewDTLSServerProcessor(dtlsVersion uint16, unknownPacketReadFunc udp.ServerReadFunc, windowSize int, resendCount uint8, certificates map[DTLSCipherSuite]*DTLSCertificate, reportTraffic bool) (*DTLSServerProcessor, error) {
	processor := &DTLSServerProcessor{
		connections:           webtools.MakeSafeMap[*udp.ServerConn, *DTLSServerConn](),
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

func (processor *DTLSServerProcessor) ReadDataAndProcess(conn *udp.ServerConn, data []byte, ended bool) (err error) {
	//Get DTLS Processor
	dtlsConnection := processor.connections.Get(conn)
	if dtlsConnection == nil {
		dtlsConnection = &DTLSServerConn{
			processor:           NewDTLSProcessor(processor.unknownPacketReadFunc, processor.windowSize, processor.resendCount, processor.reportTraffic),
			ephemeralPrivateKey: nil,
			clientRandom:        [32]byte{},
			serverRandom:        [32]byte{},
		}
		processor.connections.Set(conn, dtlsConnection)
	}

	//Process
	records, err := dtlsConnection.processor.ReadData(conn, data, ended)
	if err != nil {
		return err
	}
	return processor.ProcessServer(records, conn)
}

func (processor *DTLSServerProcessor) ProcessServer(records []DTLSRecord, conn *udp.ServerConn) (err error) {
	//Get DTLS Processor
	dtlsConnection := processor.connections.Get(conn)
	if dtlsConnection == nil {
		dtlsConnection = &DTLSServerConn{
			processor:           NewDTLSProcessor(processor.unknownPacketReadFunc, processor.windowSize, processor.resendCount, processor.reportTraffic),
			ephemeralPrivateKey: nil,
			clientRandom:        [32]byte{},
			serverRandom:        [32]byte{},
		}
		processor.connections.Set(conn, dtlsConnection)
	}

	//Process records
	for _, record := range records {
		if record.ContentType == HandshakeCType {
			//Handshake
			handshake := record.Fragment.(DTLSHandshake)
			if handshake.HandshakeType == ClientHelloHType {
				//Client Hello
				dtlsConnection.processor.Logger.Log(1, "Got ClientHello")
				clientHello := handshake.Body.(DTLSClientHello)
				dtlsConnection.clientRandom = clientHello.Random
				cookie := processor.generateCookieHash(conn.Address.String(), clientHello.Random)
				//fmt.Println(len(clientHello.Cookie))
				//fmt.Println(clientHello.Cookie)
				if len(clientHello.Cookie) == 0 {
					//Empty Cookie - Send HelloVerifyRequest
					dtlsConnection.processor.Logger.Log(1, "Sending HelloVerifyRequest")
					dtlsConnection.processor.AddWriteRecord(MakeDTLSRecord(HandshakeCType, record.ProtocolVersion, record.Epoch,
						DTLSHandshake{
							HandshakeType:   HelloVerifyRequestHType,
							MessageSequence: 0, //Set in runtime
							Body: DTLSHelloVerifyRequest{
								ServerVersion: processor.version,
								Cookie:        cookie,
							},
						},
					))
					continue
				}

				//Normal ClientHello - check cookie
				if subtle.ConstantTimeCompare(cookie, clientHello.Cookie) == 0 {
					dtlsConnection.processor.Logger.Log(2, "Got invalid ClientHello cookie: got="+hex.EncodeToString(clientHello.Cookie)+"; wants="+hex.EncodeToString(cookie))
					continue
				} else {
					dtlsConnection.processor.Logger.Log(0, "Got valid ClientHello cookie: got="+hex.EncodeToString(clientHello.Cookie)+"; wants="+hex.EncodeToString(cookie))
				}

				//Valid ClientHello
				fmt.Print("[")
				for _, s := range clientHello.CipherSuites {
					fmt.Printf("%#x, ", s)
				}
				fmt.Println("]")
				for _, v := range clientHello.Extensions {
					fmt.Println(v)
				}

				//Find compatible CipherSuite
				dtlsConnection.cipherSuite = 0
				for i := len(clientHello.CipherSuites) - 1; i > 0; i-- {
					v := clientHello.CipherSuites[i]
					_, _, _, _, _, err = v.GetSuiteConfig()
					if err != nil {
						dtlsConnection.processor.Logger.Log(2, "Unknown cipher suite: "+strconv.FormatUint(uint64(v), 10))
					} else {
						dtlsConnection.cipherSuite = v
						break
					}
				}
				if dtlsConnection.cipherSuite == 0 {
					dtlsConnection.processor.Logger.Log(3, "Uncompatible cipher suites")
					return errors.New("uncompatible cipher suites")
				}

				//Find compatible CompressionMethod
				foundCompressionMethod := false
				for _, v := range clientHello.CompressionMethodsData {
					if v == 0 {
						foundCompressionMethod = true
						break
					}
				}
				if !foundCompressionMethod {
					dtlsConnection.processor.Logger.Log(3, "Uncompatible compression method")
					return errors.New("uncompatible compression method")
				}

				//Send ServerHello
				dtlsConnection.processor.Logger.Log(1, "Sending ServerHello")
				_, err := rand.Read(dtlsConnection.serverRandom[:])
				if err != nil {
					dtlsConnection.processor.Logger.Log(3, "Error generating serverRandom: "+err.Error())
					continue
				}
				dtlsConnection.processor.AddWriteRecord(MakeDTLSRecord(HandshakeCType, processor.version, record.Epoch,
					MakeDTLSHandshake(ServerHelloHType, DTLSServerHello{
						ServerVersion:     processor.version,
						Random:            dtlsConnection.serverRandom,
						SessionID:         nil,
						CipherSuite:       dtlsConnection.cipherSuite,
						CompressionMethod: 0, //SET FOR STATIC NOW
						Extensions: []DTLSHelloExtension{
							{ExtensionType: 0xff01, ExtensionData: []byte{0x00}}, //RenegotiationInfo
						},
					}),
				))

				//Send Certificate
				dtlsConnection.processor.Logger.Log(1, "Sending Certificate")
				dtlsConnection.processor.AddWriteRecord(MakeDTLSRecord(HandshakeCType, processor.version, record.Epoch,
					MakeDTLSHandshake(CertificateHType, MakeDTLSCertificateData(processor.certificates[dtlsConnection.cipherSuite]))))

				//Send KeyExchange
				dtlsConnection.processor.Logger.Log(1, "Sending KeyExchange")
				keyExchange, err := dtlsConnection.cipherSuite.GenerateDTLSKeyExchange(dtlsConnection, processor.certificates[dtlsConnection.cipherSuite])
				if err != nil {
					dtlsConnection.processor.Logger.Log(3, "Error generating KeyExchange: "+err.Error())
					continue
				}
				dtlsConnection.processor.AddWriteRecord(MakeDTLSRecord(HandshakeCType, processor.version, record.Epoch,
					MakeDTLSHandshake(ServerKeyExchangeHType, keyExchange)))

				//Send ServerHelloDone
				dtlsConnection.processor.Logger.Log(1, "Sending ServerHelloDone")
				dtlsConnection.processor.AddWriteRecord(MakeDTLSRecord(HandshakeCType, processor.version, record.Epoch,
					MakeDTLSHandshake(ServerHelloDoneHType, nil)))
			} else if handshake.HandshakeType == ClientKeyExchangeHType {
				//Client Key Exchange
				dtlsConnection.processor.Logger.Log(1, "Got ClientKeyExchange")
				keyExchange := handshake.Body.(DTLSClientKeyExchangeECDHE)
				dtlsConnection.clientPublicKey, err = keyExchange.GetPublicKey()
				if err != nil {
					dtlsConnection.processor.Logger.Log(3, "Error parsing ClientKeyExchange: "+err.Error())
					continue
				}
			} else {
				panic("unknown DTLS handshake type: " + strconv.FormatUint(uint64(handshake.HandshakeType), 10))
			}
		}
	}

	//Send records
	err = dtlsConnection.processor.ProcessWriteSend(conn)
	return err
}
