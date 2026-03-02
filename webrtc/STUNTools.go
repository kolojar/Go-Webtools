package webrtc

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"webtools"
)

type MessageTypeSTUN uint16

const magicCookie uint32 = 0x2112A442

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-6
*/
func PackSTUNPacket(messageType MessageTypeSTUN) []byte {
	//Make packet header - 20 bytes is size of header
	packetHeader := make([]byte, 20)

	//First 2 bits must be 0
	webtools.ClearBit(packetHeader[0], 0)
	webtools.ClearBit(packetHeader[0], 1)

	//Next 14 bits for MessageType

	binary.BigEndian.PutUint32(_, magicCookie)
	return nil
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-6
Returns: MessageTypeSTUN, transactionID (as hex), message, error
*/
func UnpackSTUNPacket(data []byte, isIPv4 bool) (MessageTypeSTUN, string, []byte, error) {
	//Check length
	if len(data) < 20 || len(data) > webtools.FormatByBool(isIPv4, 576, 1280) {
		return 0, "", nil, errors.New("invalid packet size")
	}

	//Check if first 2 bits are 0
	if webtools.CheckBitArray(data, 0) || webtools.CheckBitArray(data, 1) {
		return 0, "", nil, errors.New("invalid bit 0 or 1")
	}

	//Get STUN Message Type = 14 bits
	messageType := binary.BigEndian.Uint16(data[0:2])

	//Check Message Length = 16 bits - last 2 bits must be 0
	if webtools.CheckBit(data[3], 6) || webtools.CheckBit(data[3], 7) {
		return 0, "", nil, errors.New("invalid MessageLength suffix")
	}

	//Get Message Length = 16 bits
	messageLength := binary.BigEndian.Uint16(data[2:4]) >> 2

	//Get Magic Cookie = 32 bits + check
	magicCookieGet := binary.BigEndian.Uint32(data[4:8])
	if magicCookieGet != magicCookie {
		return 0, "", nil, errors.New("invalid MagicCookie")
	}

	//Get Transaction ID = 96 bits
	transactionID := hex.EncodeToString(data[8:20])

	//Get Message
	message := data[20 : 20+messageLength]
	return MessageTypeSTUN(messageType), transactionID, message, nil
}
