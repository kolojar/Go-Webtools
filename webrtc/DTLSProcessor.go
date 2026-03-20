package webrtc

import (
	"bytes"
	"io"
	"strconv"
	"webtools"
	"webtools/udp"
)

type DTLSProcessor struct {
	fragmentedHandshakeBuilders webtools.SafeMap[uint16, DTLSRecord]
	currentEpoch                uint16
	replayWindowPrevious        ReplayWindow[uint64]
	replayWindowCurrent         ReplayWindow[uint64]
	Logger                      *webtools.ConsoleLogger
	unknownPacketReadFunc       udp.ServerReadFunc
}

/*
NewDTLSProcessor creates new DTLS processor. WindowSize recommended value is 64. Feed values using ProcessData or ReadData.
*/
func NewDTLSProcessor(unknownPacketReadFunc udp.ServerReadFunc, windowSize int, reportTraffic bool) *DTLSProcessor {
	return &DTLSProcessor{
		fragmentedHandshakeBuilders: webtools.MakeSafeMap[uint16, DTLSRecord](),
		currentEpoch:                0,
		replayWindowPrevious:        MakeReplayWindow[uint64](windowSize),
		replayWindowCurrent:         MakeReplayWindow[uint64](windowSize),
		Logger:                      webtools.NewConsoleLoggerForTraffic("DTLSProcessor", reportTraffic),
		unknownPacketReadFunc:       unknownPacketReadFunc,
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
	completeRecords, hasNonDTLSData, err = processor.ProcessData(bytes.NewReader(data))
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

func (processor *DTLSProcessor) ProcessData(reader io.Reader) (completeRecords []DTLSRecord, hasNonDTLSData bool, err error) {
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
		if record.ContentType == DTLSRecordContentTypeHandshake {
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
				record := processor.fragmentedHandshakeBuilders.Get(fragment.MessageSequence)
				record.Fragment = handshake
				processor.Logger.Log(1, "Completed record with message sequence number: "+strconv.FormatUint(uint64(fragment.MessageSequence), 10))
				completeRecords = append(completeRecords, record)
				continue
			}

			//Not complete
			record.Fragment = fragmentProcessor
			processor.Logger.Log(0, "Fragmenting record with message sequence number: "+strconv.FormatUint(uint64(fragment.MessageSequence), 10))
			processor.fragmentedHandshakeBuilders.Set(fragment.MessageSequence, record)
			continue
		}
	}
	return completeRecords, false, nil
}
