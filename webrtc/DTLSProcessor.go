package webrtc

import (
	"io"
	"strconv"
	"webtools"
)

type DTLSProcessor struct {
	buildedFragmentedRequests webtools.SafeMap[uint64, DTLSRecord]
	currentEpoch              uint16
	replayWindowPrevious      ReplayWindow[uint64]
	replayWindowCurrent       ReplayWindow[uint64]
	logger                    *webtools.ConsoleLogger
}

func (processor *DTLSProcessor) ProcessData(reader io.Reader) (completeRecords []DTLSRecord, hasNonDTLSData bool, err error) {
	//Read all records
	records, hasNonDTLSData, err := UnpackDTLSRecords(reader)
	if err != nil {
		processor.logger.Log(3, "Error while reading DTLS packet: "+err.Error())
		return nil, hasNonDTLSData, err
	}
	if hasNonDTLSData {
		processor.logger.Log(3, "Invalid DTLS data!")
		return nil, true, err
	}

	for _, record := range records {
		//Check if Epoch is valid
		processor.logger.Log(0, "Processing DTLS record: type="+strconv.FormatUint(uint64(record.ContentType), 10)+"; epoch="+strconv.FormatUint(uint64(record.Epoch), 10)+"; sequenceNumber="+strconv.FormatUint(record.SequenceNumber, 10))
		if !(processor.currentEpoch == record.Epoch || processor.currentEpoch-1 == record.Epoch) {
			processor.logger.Log(1, "Dropping record with epoch: "+strconv.FormatUint(uint64(record.Epoch), 10))
			continue
		}

		//Check if packet was already recieved
		if processor.currentEpoch == record.Epoch {
			if !processor.replayWindowCurrent.ApplyWindowCheck(record.SequenceNumber) {
				processor.logger.Log(1, "Dropping record with sequence number: "+strconv.FormatUint(record.SequenceNumber, 10))
				continue
			}
		} else if processor.currentEpoch-1 == record.Epoch {
			if !processor.replayWindowPrevious.ApplyWindowCheck(record.SequenceNumber) {
				processor.logger.Log(1, "Dropping record with sequence number: "+strconv.FormatUint(record.SequenceNumber, 10))
				continue
			}
		}

		//Valid packet
		if record.ContentType == DTLSRecordContentTypeHandshake {
			//Handshake
			record.Fragment.(DTLSHandshakeFragment)
		}
	}
}
