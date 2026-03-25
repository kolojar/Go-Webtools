package webrtc

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"time"
	"webtools"
	"webtools/udp"
)

type DTLSProcessor struct {
	fragmentedHandshakeBuilders          webtools.SafeMap[uint16, DTLSRecord]
	currentEpoch                         uint16
	replayWindowPrevious                 ReplayWindow[uint64]
	replayWindowCurrent                  ReplayWindow[uint64]
	Logger                               *webtools.ConsoleLogger
	unknownPacketReadFunc                udp.ServerReadFunc
	toWritePackets                       webtools.SafeSlice[DTLSRecord]
	handshakeMessageSequence             uint16
	sendingSequenceNumber                uint64 //uint48
	lastRecievedHandshakeMessageSequence uint16
	lastSendBuffer                       [][]byte
	resendTimer                          *time.Timer
	resendCount                          uint8
}

/*
NewDTLSProcessor creates new DTLS processor. WindowSize recommended value is 64. Feed values using ProcessData or ReadData.
*/
func NewDTLSProcessor(unknownPacketReadFunc udp.ServerReadFunc, windowSize int, resendCount uint8, reportTraffic bool) *DTLSProcessor {
	return &DTLSProcessor{
		fragmentedHandshakeBuilders: webtools.MakeSafeMap[uint16, DTLSRecord](),
		currentEpoch:                0,
		replayWindowPrevious:        MakeReplayWindow[uint64](windowSize),
		replayWindowCurrent:         MakeReplayWindow[uint64](windowSize),
		Logger:                      webtools.NewConsoleLoggerForTraffic("DTLSProcessor", reportTraffic),
		unknownPacketReadFunc:       unknownPacketReadFunc,
		toWritePackets:              webtools.MakeSafeSlice[DTLSRecord](),
		handshakeMessageSequence:    0,
		resendCount:                 resendCount,
	}
}

/*
ReadData handles reading from conn - data in data array. If not DTLS packet, is is forwarded using unknownPacketReadFunc
*/
func (processor *DTLSProcessor) ReadData(conn *udp.ServerConn, data []byte, ended bool) (completeRecords []DTLSRecord, err error) {
	var hasNonDTLSData bool
	if ended {
		return nil, nil
	}

	//Process
	completeRecords, hasNonDTLSData, err = processor.ProcessData(bytes.NewReader(data), conn)
	if hasNonDTLSData {
		if processor.unknownPacketReadFunc != nil {
			processor.unknownPacketReadFunc(conn, data, ended)
		}
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return completeRecords, nil
}

func (processor *DTLSProcessor) ProcessData(reader io.Reader, conn *udp.ServerConn) (completeRecords []DTLSRecord, hasNonDTLSData bool, err error) {
	//Read all records
	records, hasNonDTLSData, err := UnpackDTLSRecords(reader)
	if err != nil {
		processor.Logger.Log(3, "Error while reading DTLS packet: "+err.Error())
		return nil, hasNonDTLSData, err
	}
	if hasNonDTLSData {
		processor.Logger.Log(3, "Invalid DTLS data!")
		return nil, true, err
	}

	if processor.resendTimer != nil {
		processor.resendTimer.Stop()
	}
	completeRecords = make([]DTLSRecord, 0)
	for _, record := range records {
		//Check if Epoch is valid
		processor.Logger.Log(0, "Processing DTLS record: type="+strconv.FormatUint(uint64(record.ContentType), 10)+"; epoch="+strconv.FormatUint(uint64(record.Epoch), 10)+"; sequenceNumber="+strconv.FormatUint(record.SequenceNumber, 10))
		if !(processor.currentEpoch == record.Epoch || processor.currentEpoch-1 == record.Epoch) {
			processor.Logger.Log(2, "Dropping record with epoch: "+strconv.FormatUint(uint64(record.Epoch), 10))
			continue
		}

		//Check if packet was already recieved
		if processor.currentEpoch == record.Epoch {
			if !processor.replayWindowCurrent.ApplyWindowCheck(record.SequenceNumber) {
				processor.Logger.Log(2, "Dropping record with sequence number: "+strconv.FormatUint(record.SequenceNumber, 10))
				continue
			}
		} else if processor.currentEpoch-1 == record.Epoch {
			if !processor.replayWindowPrevious.ApplyWindowCheck(record.SequenceNumber) {
				processor.Logger.Log(2, "Dropping record with sequence number: "+strconv.FormatUint(record.SequenceNumber, 10))
				continue
			}
		}

		//Valid packet
		if record.ContentType == HandshakeCType {
			//Handshake
			fragment := record.Fragment.(DTLSHandshakeFragment)
			var fragmentProcessor DTLSHandshakeFragmentProcessor
			if processor.fragmentedHandshakeBuilders.Has(fragment.MessageSequence) {
				//Get processor
				fragmentProcessor = processor.fragmentedHandshakeBuilders.Get(fragment.MessageSequence).Fragment.(DTLSHandshakeFragmentProcessor)
			} else {
				//Create new processor
				fragmentProcessor = MakeDTLSHandshakeFragmentProcessor(int(fragment.Length))
			}

			//Process
			isCompleteHandshake, handshake := fragmentProcessor.Process(fragment)
			if isCompleteHandshake {
				//Completed
				processor.fragmentedHandshakeBuilders.Delete(fragment.MessageSequence)
				if processor.fragmentedHandshakeBuilders.Has(fragment.MessageSequence) {
					record = processor.fragmentedHandshakeBuilders.Get(fragment.MessageSequence)
				}

				//Convert by types
				if handshake.HandshakeType == ClientHelloHType {
					handshake.Body, err = UnpackDTLSClientHello(bytes.NewReader(handshake.Body.([]byte)))
					if err != nil {
						processor.Logger.Log(3, "Error unpacking DTLSClientHello: "+err.Error())
						return nil, false, err
					}
				} else {
					panic("unknown handshake type: " + strconv.FormatUint(uint64(fragment.HandshakeType), 10))
				}

				//Finish
				record.Fragment = handshake
				processor.Logger.Log(1, "Completed record with message sequence: "+strconv.FormatUint(uint64(fragment.MessageSequence), 10))
				if handshake.MessageSequence > 0 && handshake.MessageSequence == processor.lastRecievedHandshakeMessageSequence {
					//Retransmit
					processor.RetransmitLastSendBuffer(conn)
					continue
				}
				completeRecords = append(completeRecords, record)
				if handshake.MessageSequence > processor.lastRecievedHandshakeMessageSequence {
					processor.lastRecievedHandshakeMessageSequence = handshake.MessageSequence
				}
				continue
			}

			//Not complete
			record.Fragment = fragmentProcessor
			processor.Logger.Log(0, "Fragmenting record with message sequence: "+strconv.FormatUint(uint64(fragment.MessageSequence), 10))
			processor.fragmentedHandshakeBuilders.Set(fragment.MessageSequence, record)
			continue
		}
	}

	//Check for retransission
	return completeRecords, false, nil
}

func (processor *DTLSProcessor) AddWriteRecord(record DTLSRecord) {
	processor.Logger.Log(3, "Adding to Write queue with type: "+strconv.FormatUint(uint64(record.ContentType), 10))
	processor.toWritePackets.Append(record)
}

func (processor *DTLSProcessor) ProcessWrite() ([][]byte, error) {
	defer func() {
		processor.toWritePackets.Clear()
	}()
	if processor.toWritePackets.Len() == 0 {
		return nil, nil
	}

	//Make fragments
	fragmentedRecords := make([]DTLSRecord, 0)
	for _, record := range processor.toWritePackets.GetValuesAndClear() {
		processor.Logger.Log(1, "Processing write record with type: "+strconv.FormatUint(uint64(record.ContentType), 10))
		record.SequenceNumber = processor.sendingSequenceNumber
		processor.sendingSequenceNumber++
		if record.ContentType == HandshakeCType {
			//Handshake - make fragment
			handshake := record.Fragment.(DTLSHandshake)
			handshakeBytes, err := handshake.MakeBodyBytes(uint16(processor.handshakeMessageSequence))
			if err != nil {
				processor.Logger.Log(3, "Error making DTLSHandshake bytes: "+err.Error())
				return nil, err
			}

			//Split into fragments
			handshakeBytesReader := bytes.NewReader(handshakeBytes)
			fragmentOffset := uint32(0)
			fragmentNumber := 0
			for {
				//Read fragment part
				fragment := make([]byte, 1000)
				n, err := handshakeBytesReader.Read(fragment)
				if err != nil {
					if !errors.Is(err, io.EOF) {
						processor.Logger.Log(3, "Error fragmenting DTLSHandshake bytes: "+err.Error())
						return nil, err
					}
				}

				//Make fragment record
				recordFragment := record
				recordFragment.Fragment = DTLSHandshakeFragment{
					HandshakeType:   handshake.HandshakeType,
					Length:          uint32(len(handshakeBytes)),
					MessageSequence: processor.handshakeMessageSequence,
					FragmentOffset:  fragmentOffset,
					FragmentLength:  uint32(n),
					FragmentData:    fragment[:n],
				}
				fragmentedRecords = append(fragmentedRecords, recordFragment)
				fragmentNumber++
				processor.Logger.Log(0, "Fragmented DTLSHandshake: Length="+strconv.Itoa(len(handshakeBytes))+"; MessageSequence="+strconv.FormatUint(uint64(processor.handshakeMessageSequence), 10)+"; FragmentOffset="+strconv.FormatUint(uint64(fragmentOffset), 10)+"; FragmentLength="+strconv.Itoa(n))

				if handshakeBytesReader.Len() == 0 {
					break
				}
			}
			processor.handshakeMessageSequence++
		} else {
			//Not fragmentable type
			processor.Logger.Log(2, "Processing nonfragmentable type")
			fragmentedRecords = append(fragmentedRecords, record)
		}
	}

	//Put Fragments into packets
	resultFragments := make([][]byte, 0)
	processor.Logger.Log(1, "Created fragments: "+strconv.Itoa(len(fragmentedRecords)))
	for _, record := range fragmentedRecords {
		foundFragment := false
		recordBytes, err := record.MakeBytes()
		if err != nil {
			processor.Logger.Log(3, "Error encoding DTLSRecord: "+err.Error())
			return nil, err
		}
		for i, fragment := range resultFragments {
			//Packet max size is 1200 bytes -> Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-3.2.3
			if len(fragment)+len(recordBytes) < 1200 {
				resultFragments[i] = append(resultFragments[i], recordBytes...)
				foundFragment = true
				break
			}
		}
		if foundFragment {
			continue
		}

		//Add new
		resultFragments = append(resultFragments, recordBytes)
	}
	processor.Logger.Log(1, "Created packets: "+strconv.Itoa(len(resultFragments)))
	return resultFragments, nil
}

func (processor *DTLSProcessor) RetransmitLastSendBuffer(conn *udp.ServerConn) error {
	//Send data
	processor.Logger.Log(2, "Retransmitting last packet.")
	for _, packet := range processor.lastSendBuffer {
		_, err := conn.Send(packet)
		if err != nil {
			return err
		}
	}
	return nil
}

func (processor *DTLSProcessor) ProcessWriteSend(conn *udp.ServerConn) error {
	//Make data
	data, err := processor.ProcessWrite()
	if len(data) > 0 {
		processor.lastSendBuffer = data
		if err != nil {
			return err
		}
		for _, packet := range data {
			_, err = conn.Send(packet)
			if err != nil {
				return err
			}
		}
		processor.resendTimer = time.AfterFunc(time.Second, func() { processor.doRetransmittion(time.Second*2, 1, conn) })
	}
	return nil
}

func (processor *DTLSProcessor) doRetransmittion(duration time.Duration, resendCount uint8, conn *udp.ServerConn) {
	processor.RetransmitLastSendBuffer(conn)
	if processor.resendCount > resendCount {
		processor.resendTimer = time.AfterFunc(time.Second, func() { processor.doRetransmittion(duration*2, resendCount+1, conn) })
	} else {
		processor.Logger.Log(2, "Could not do retransmittion - timeout reached.")
	}
}
