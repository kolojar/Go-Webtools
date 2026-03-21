package webrtc

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"webtools/database"
)

const DTLSVersion12 = 0xfefd //1.2
const DTLSVersion10 = 0xfeff //1.0

type DTLSContentType uint8

const HandshakeCType DTLSContentType = 22

/*
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.1
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.1
*/
type DTLSRecord struct {
	ContentType     DTLSContentType
	ProtocolVersion uint16
	Epoch           uint16
	SequenceNumber  uint64 //uint48
	//Length          uint16
	Fragment any //DTLSHandshake
}

func UnpackDTLSRecord(reader io.Reader) (record DTLSRecord, hasNonDTLSData bool, firstEOF bool, err error) {
	record = DTLSRecord{}
	//Read ContentType
	contentType, err := database.ReadUint8(reader)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return record, true, true, err
		}
		return record, true, false, err
	}
	record.ContentType = DTLSContentType(contentType)

	//Check for DTLS
	if record.ContentType != 20 && record.ContentType != 21 && record.ContentType != HandshakeCType && record.ContentType != 23 {
		return record, true, false, errors.New("not a DTLS packet - invalid content type: " + strconv.FormatUint(uint64(record.ContentType), 10))
	}

	//Read Protocol Version
	record.ProtocolVersion, err = database.ReadUint16(reader)
	if err != nil {
		return record, true, false, err
	}

	//Check for DTLS version
	if record.ProtocolVersion != DTLSVersion10 && record.ProtocolVersion != DTLSVersion12 {
		return record, true, false, errors.New("not a DTLS packet - invalid protocol version: " + strconv.FormatUint(uint64(record.ProtocolVersion), 10))
	}

	//Read Epoch
	record.Epoch, err = database.ReadUint16(reader)
	if err != nil {
		return record, true, false, err
	}

	//Read Sequence Number
	record.SequenceNumber, err = database.ReadUint48(reader)
	if err != nil {
		return record, true, false, err
	}

	//Read Length
	length, err := database.ReadUint16(reader)
	if err != nil {
		return record, true, false, err
	}

	//Limit reader for Fragment
	limitedReader := io.LimitReader(reader, int64(length))

	//Read
	if record.ContentType == HandshakeCType {
		//Handshake
		record.Fragment, err = UnpackDTLSHandshakeFragment(limitedReader)
	} else {
		panic("content type: " + strconv.Itoa(int(record.ContentType)) + " not implemented")
	}
	return record, false, false, err
}

func UnpackDTLSRecords(reader io.Reader) (records []DTLSRecord, hasNonDTLSData bool, err error) {
	err = nil
	records = make([]DTLSRecord, 0)
	for err == nil {
		//Read next record
		var record DTLSRecord
		var firstEOF bool
		record, hasNonDTLSData, firstEOF, err = UnpackDTLSRecord(reader)
		if err != nil {
			if firstEOF && len(records) > 0 {
				err = nil
				break
			}
			return nil, hasNonDTLSData, err
		}
		if hasNonDTLSData {
			return nil, true, err
		}
		records = append(records, record)
	}
	return records, false, err
}

func (record DTLSRecord) MakeBytes() (result []byte, err error) {
	buffer := bytes.NewBuffer(make([]byte, 0))

	//Put ContentType
	err = database.AppendUint8(buffer, uint8(record.ContentType))
	if err != nil {
		return nil, err
	}

	//Put ProtocolVersion
	err = database.AppendUint16(buffer, record.ProtocolVersion)
	if err != nil {
		return nil, err
	}

	//Put Epoch
	err = database.AppendUint16(buffer, record.Epoch)
	if err != nil {
		return nil, err
	}

	//Put SequenceNumber
	err = database.AppendUint48(buffer, record.SequenceNumber)
	if err != nil {
		return nil, err
	}

	//Put Fragment
	if record.ContentType == HandshakeCType {
		fragment, err := record.Fragment.(DTLSHandshakeFragment).MakeBytes()
		if err != nil {
			return nil, err
		}
		err = database.AppendByteArray(buffer, 2, fragment, nil)
		if err != nil {
			return nil, err
		}
	} else {
		panic("unknown dtls contentType: " + strconv.FormatUint(uint64(record.ContentType), 10))
	}
	return buffer.Bytes(), nil
}
