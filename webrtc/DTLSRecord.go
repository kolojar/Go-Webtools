package webrtc

import (
	"errors"
	"io"
	"strconv"
	"webtools/database"
)

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

func UnpackDTLSRecord(reader io.Reader) (record DTLSRecord, isDTLS bool, err error) {
	record = DTLSRecord{}

	//Read ContentType
	record.ContentType, err = database.ReadUint8(reader)
	if err != nil {
		return record, false, err
	}

	//Check for DTLS
	if record.ContentType != 20 && record.ContentType != 21 && record.ContentType != 22 && record.ContentType != 23 {
		return record, false, errors.New("not a DTLS packet - invalid content type: " + strconv.FormatUint(uint64(record.ContentType), 10))
	}

	//Read Protocol Version
	record.ProtocolVersion, err = database.ReadUint16(reader)
	if err != nil {
		return record, false, err
	}

	//Check for DTLS version
	if record.ProtocolVersion != 0xfeff && record.ProtocolVersion != 0xfefd {
		return record, false, errors.New("not a DTLS packet - invalid protocol version: " + strconv.FormatUint(uint64(record.ProtocolVersion), 10))
	}

	//Read Epoch
	record.Epoch, err = database.ReadUint16(reader)
	if err != nil {
		return record, false, err
	}

	//Read Sequence Number
	record.SequenceNumber, err = database.ReadUint48(reader)
	if err != nil {
		return record, false, err
	}

	//Read Length
	length, err := database.ReadUint16(reader)
	if err != nil {
		return record, false, err
	}

	//Limit reader for Fragment
	limitedReader := io.LimitReader(reader, int64(length))

	//Read
	if record.ContentType == 22 {
		//Handshake
		record.Fragment, err = UnpackDTLSHandshake(limitedReader)
	} else {
		panic("content type: " + strconv.Itoa(int(record.ContentType)) + " not implemented")
	}
	return record, true, err
}
