package webrtc

import (
	"io"
	"strconv"
	"webtools"
)

type DTLSProcessor struct {
	fragmentedHandshakeBuilders webtools.SafeMap[uint16, DTLSRecord]
	currentEpoch                uint16
	replayWindowPrevious        ReplayWindow[uint64]
	replayWindowCurrent         ReplayWindow[uint64]
	logger                      *webtools.ConsoleLogger
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

	completeRecords = make([]DTLSRecord, 0)
	for _, record := range records {
		//Check if Epoch is valid
		processor.logger.Log(0, "Processing DTLS record: type="+strconv.FormatUint(uint64(record.ContentType), 10)+"; epoch="+strconv.FormatUint(uint64(record.Epoch), 10)+"; sequenceNumber="+strconv.FormatUint(record.SequenceNumber, 10))
		if !(processor.currentEpoch == record.Epoch || processor.currentEpoch-1 == record.Epoch) {
			processor.logger.Log(2, "Dropping record with epoch: "+strconv.FormatUint(uint64(record.Epoch), 10))
			continue
		}

		//Check if packet was already recieved
		if processor.currentEpoch == record.Epoch {
			if !processor.replayWindowCurrent.ApplyWindowCheck(record.SequenceNumber) {
				processor.logger.Log(2, "Dropping record with sequence number: "+strconv.FormatUint(record.SequenceNumber, 10))
				continue
			}
		} else if processor.currentEpoch-1 == record.Epoch {
			if !processor.replayWindowPrevious.ApplyWindowCheck(record.SequenceNumber) {
				processor.logger.Log(2, "Dropping record with sequence number: "+strconv.FormatUint(record.SequenceNumber, 10))
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
				processor.logger.Log(1, "Completed record with message sequence number: "+strconv.FormatUint(uint64(fragment.MessageSequence), 10))
				completeRecords = append(completeRecords, record)
				continue
			}

			//Not complete
			record.Fragment = fragmentProcessor
			processor.logger.Log(0, "Fragmenting record with message sequence number: "+strconv.FormatUint(uint64(fragment.MessageSequence), 10))
			processor.fragmentedHandshakeBuilders.Set(fragment.MessageSequence, record)
			continue
		}
	}
	return completeRecords, false, nil
}
