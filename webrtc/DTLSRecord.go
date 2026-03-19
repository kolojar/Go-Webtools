package webrtc

import (
	"errors"
	"io"
	"strconv"
	"webtools/database"
)

const DTLSRecordContentTypeHandshake = uint8(22)

/*
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.1
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.1
*/
type DTLSRecord struct {
	ContentType     uint8
	ProtocolVersion uint16
	Epoch           uint16
	SequenceNumber  uint64 //uint48
	//Length          uint16
	Fragment any //DTLSHandshake
}

func UnpackDTLSRecord(reader io.Reader) (record DTLSRecord, hasNonDTLSData bool, err error) {
	record = DTLSRecord{}
	//Read ContentType
	record.ContentType, err = database.ReadUint8(reader)
	if err != nil {
		return record, true, err
	}

	//Check for DTLS
	if record.ContentType != 20 && record.ContentType != 21 && record.ContentType != 22 && record.ContentType != 23 {
		return record, true, errors.New("not a DTLS packet - invalid content type: " + strconv.FormatUint(uint64(record.ContentType), 10))
	}

	//Read Protocol Version
	record.ProtocolVersion, err = database.ReadUint16(reader)
	if err != nil {
		return record, true, err
	}

	//Check for DTLS version
	if record.ProtocolVersion != 0xfeff && record.ProtocolVersion != 0xfefd {
		return record, true, errors.New("not a DTLS packet - invalid protocol version: " + strconv.FormatUint(uint64(record.ProtocolVersion), 10))
	}

	//Read Epoch
	record.Epoch, err = database.ReadUint16(reader)
	if err != nil {
		return record, true, err
	}

	//Read Sequence Number
	record.SequenceNumber, err = database.ReadUint48(reader)
	if err != nil {
		return record, true, err
	}

	//Read Length
	length, err := database.ReadUint16(reader)
	if err != nil {
		return record, true, err
	}

	//Limit reader for Fragment
	limitedReader := io.LimitReader(reader, int64(length))

	//Read
	if record.ContentType == DTLSRecordContentTypeHandshake {
		//Handshake
		record.Fragment, err = UnpackDTLSHandshakeFragment(limitedReader)
	} else {
		panic("content type: " + strconv.Itoa(int(record.ContentType)) + " not implemented")
	}
	return record, false, err
}

func UnpackDTLSRecords(reader io.Reader) (records []DTLSRecord, hasNonDTLSData bool, err error) {
	err = nil
	records = make([]DTLSRecord, 0)
	for err == nil {
		//Read next record
		var record DTLSRecord
		record, hasNonDTLSData, err = UnpackDTLSRecord(reader)
		if err != nil {
			return nil, hasNonDTLSData, err
		}
		if hasNonDTLSData {
			return nil, true, err
		}
		records = append(records, record)
	}
	return records, false, err
}
