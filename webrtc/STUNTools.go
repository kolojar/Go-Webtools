package webrtc

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"webtools"
)

type MessageTypeSTUN uint16
type MessageClassSTUN uint8

const MessageTypeSTUNBinding MessageTypeSTUN = 0b000000000001

const MessageClassSTUNRequest MessageClassSTUN = 0b00
const MessageClassSTUNIndication MessageClassSTUN = 0b01
const MessageClassSTUNSuccessResponce MessageClassSTUN = 0b10
const MessageClassSTUNErrorResponce MessageClassSTUN = 0b11

const magicCookie uint32 = 0x2112A442

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-6
Returns transtactionID, packet, error
*/
func PackSTUNPacket(messageType MessageTypeSTUN, messageClass MessageClassSTUN, message []byte) (string, []byte, error) {
	//Make packet header - 20 bytes is size of header
	packet := make([]byte, 20)

	//First 2 bits must be 0
	webtools.ClearBit(packet[0], 0)
	webtools.ClearBit(packet[0], 1)

	//Convert MessageType and MessageClass to bytes
	messageTypeAsBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(messageTypeAsBytes, uint16(messageType))

	//Write 14 bits for MessageType and MessageClass
	packet[0] = webtools.SetBitValue(packet[0], 2, webtools.CheckBit(messageTypeAsBytes[0], 4))
	packet[0] = webtools.SetBitValue(packet[0], 3, webtools.CheckBit(messageTypeAsBytes[0], 5))
	packet[0] = webtools.SetBitValue(packet[0], 4, webtools.CheckBit(messageTypeAsBytes[0], 6))
	packet[0] = webtools.SetBitValue(packet[0], 5, webtools.CheckBit(messageTypeAsBytes[0], 7))
	packet[0] = webtools.SetBitValue(packet[0], 6, webtools.CheckBit(messageTypeAsBytes[1], 0))
	packet[0] = webtools.SetBitValue(packet[0], 7, webtools.CheckBit(messageTypeAsBytes[1], 1))
	packet[1] = webtools.SetBitValue(packet[1], 0, webtools.CheckBit(byte(messageClass), 6))
	packet[1] = webtools.SetBitValue(packet[1], 1, webtools.CheckBit(messageTypeAsBytes[1], 2))
	packet[1] = webtools.SetBitValue(packet[1], 2, webtools.CheckBit(messageTypeAsBytes[1], 3))
	packet[1] = webtools.SetBitValue(packet[1], 3, webtools.CheckBit(byte(messageClass), 7))
	packet[1] = webtools.SetBitValue(packet[1], 4, webtools.CheckBit(messageTypeAsBytes[1], 4))
	packet[1] = webtools.SetBitValue(packet[1], 5, webtools.CheckBit(messageTypeAsBytes[1], 5))
	packet[1] = webtools.SetBitValue(packet[1], 6, webtools.CheckBit(messageTypeAsBytes[1], 6))
	packet[1] = webtools.SetBitValue(packet[1], 7, webtools.CheckBit(messageTypeAsBytes[1], 7))

	//Write Message Length = 16 bits
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(message))<<2)

	//Last 2 bits must be 0
	webtools.ClearBit(packet[3], 6)
	webtools.ClearBit(packet[3], 7)

	//Write Magic Cookie
	binary.BigEndian.PutUint32(packet[4:8], magicCookie)

	//Generate and write Transaction ID
	_, err := rand.Read(packet[8:20])
	if err != nil {
		return "", nil, err
	}

	//Write Message
	packet = append(packet, message...)
	return hex.EncodeToString(packet[8:20]), packet, nil
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-6
Returns: MessageTypeSTUN, transactionID (as hex), message, error
*/
func UnpackSTUNPacket(data []byte, isIPv4 bool) (MessageTypeSTUN, MessageClassSTUN, string, []byte, error) {
	//Check length
	if len(data) < 20 || len(data) > webtools.FormatByBool(isIPv4, 576, 1280) {
		return 0, 0, "", nil, errors.New("invalid packet size")
	}

	//Check if first 2 bits are 0
	if webtools.CheckBitArray(data, 0) || webtools.CheckBitArray(data, 1) {
		return 0, 0, "", nil, errors.New("invalid bit 0 or 1")
	}

	//Get STUN Message Type + STUN Message Class = 14 bits
	messageTypeAsBytes := make([]byte, 2)
	messageClass := uint8(0)
	messageTypeAsBytes[0] = webtools.SetBitValue(messageTypeAsBytes[0], 4, webtools.CheckBit(data[0], 2))
	messageTypeAsBytes[0] = webtools.SetBitValue(messageTypeAsBytes[0], 5, webtools.CheckBit(data[0], 3))
	messageTypeAsBytes[0] = webtools.SetBitValue(messageTypeAsBytes[0], 6, webtools.CheckBit(data[0], 4))
	messageTypeAsBytes[0] = webtools.SetBitValue(messageTypeAsBytes[0], 7, webtools.CheckBit(data[0], 5))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 0, webtools.CheckBit(data[0], 6))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 1, webtools.CheckBit(data[0], 7))
	messageClass = webtools.SetBitValue(messageClass, 6, webtools.CheckBit(data[1], 0))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 2, webtools.CheckBit(data[1], 1))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 3, webtools.CheckBit(data[1], 2))
	messageClass = webtools.SetBitValue(messageClass, 7, webtools.CheckBit(data[1], 3))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 4, webtools.CheckBit(data[1], 4))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 5, webtools.CheckBit(data[1], 5))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 6, webtools.CheckBit(data[1], 6))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 7, webtools.CheckBit(data[1], 7))
	messageType := binary.BigEndian.Uint16(messageTypeAsBytes)

	//Check Message Length = 16 bits - last 2 bits must be 0
	if webtools.CheckBit(data[3], 6) || webtools.CheckBit(data[3], 7) {
		return 0, 0, "", nil, errors.New("invalid MessageLength suffix")
	}

	//Get Message Length = 16 bits
	messageLength := binary.BigEndian.Uint16(data[2:4]) >> 2

	//Get Magic Cookie = 32 bits + check
	magicCookieGet := binary.BigEndian.Uint32(data[4:8])
	if magicCookieGet != magicCookie {
		return 0, 0, "", nil, errors.New("invalid MagicCookie")
	}

	//Get Transaction ID = 96 bits
	transactionID := hex.EncodeToString(data[8:20])

	//Get Message
	message := data[20 : 20+messageLength]
	return MessageTypeSTUN(messageType), MessageClassSTUN(messageClass), transactionID, message, nil
}
