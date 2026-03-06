package webrtc

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"webtools"
)

//const magicCookie uint32 = 0x2112A442

// In BigEndian
var constMagicCookie = [...]byte{0x21, 0x12, 0xA4, 0x42}

type MessageTypeSTUN uint16
type MessageClassSTUN uint8

const MessageTypeSTUNBinding MessageTypeSTUN = 0b000000000001

const MessageClassSTUNRequest MessageClassSTUN = 0b00
const MessageClassSTUNIndication MessageClassSTUN = 0b01
const MessageClassSTUNSuccessResponce MessageClassSTUN = 0b10
const MessageClassSTUNErrorResponce MessageClassSTUN = 0b11

type STUNPacketAttribute struct {
	Type          uint16
	Data          []byte
	TransactionID []byte
}

/*
EncodeSTUNPacketAttribute encodes type and data to STUNPacketAttribute
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-18.2
*/
func EncodeSTUNPacketAttribute(t uint16, data []byte) *STUNPacketAttribute {
	//TODO: ADD SUPPORT FOR ENCODING XOR
	return &STUNPacketAttribute{Type: t, Data: data}
}

/*
DecodeSTUNPacketAttribute decodes STUNPacketAttribute using type and returns decoded value
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-18.2
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-15
*/
func (attribute *STUNPacketAttribute) DecodeSTUNPacketAttribute() (map[string]any, error) {
	if attribute.Type == 0x0001 {
		//Mapped address
		//Check for length
		if len(attribute.Data) < 4 {
			return nil, errors.New("data to short for mapped address")
		}

		//First byte must be 0
		if attribute.Data[0] != 0 {
			return nil, errors.New("invalid first byte of attribute")
		}

		//Get IP family
		family := attribute.Data[1]
		if family != 1 && family != 2 {
			return nil, errors.New("invalid ip family")
		}
		addressLength := webtools.FormatByBool(family == 1, 4, 16)

		//Check for length
		if len(attribute.Data) < 4+addressLength {
			return nil, errors.New("data to short for mapped address")
		}

		//Get port
		port := binary.BigEndian.Uint16(attribute.Data[2:4])

		//Get address
		address := net.IP(attribute.Data[4:(4 + addressLength)])

		//Make result map - Specified in: https://datatracker.ietf.org/doc/html/rfc5389#section-15
		result := make(map[string]any)
		result["family"] = webtools.FormatByBool(addressLength == 4, "IPv4", "IPv6")
		result["port"] = port
		if addressLength == 4 {
			result["family"] = "IPv4"
			result["address"] = address.To4().String()
		} else {
			result["family"] = "IPv6"
			result["address"] = address.To16().String()
		}
		return result, nil
	}
	if attribute.Type == 0x0020 {
		//XOR Mapped address
		//Check for length
		if len(attribute.Data) < 4 {
			return nil, errors.New("data to short for mapped address")
		}

		//First byte must be 0
		if attribute.Data[0] != 0 {
			return nil, errors.New("invalid first byte of attribute")
		}

		//Get IP family
		family := attribute.Data[1]
		if family != 1 && family != 2 {
			return nil, errors.New("invalid ip family")
		}
		addressLength := webtools.FormatByBool(family == 1, 4, 16)

		//Check for length
		if len(attribute.Data) < 4+addressLength {
			return nil, errors.New("data to short for mapped address")
		}

		//Get port = 16 bits XOR
		port := binary.BigEndian.Uint16(webtools.XORArrays(attribute.Data[2:4], constMagicCookie[0:2]))

		//Get address
		addressBytesXOR := attribute.Data[4:(4 + addressLength)]

		//Make result map - Specified in: https://datatracker.ietf.org/doc/html/rfc5389#section-15
		result := make(map[string]any)
		result["family"] = webtools.FormatByBool(addressLength == 4, "IPv4", "IPv6")
		result["port"] = port
		if addressLength == 4 {
			//Decode using 32 bits of magicCookie
			addressBytesXOR = webtools.XORArrays(addressBytesXOR, constMagicCookie[:])

			//Write results
			result["family"] = "IPv4"
			address := net.IP(addressBytesXOR)
			result["address"] = address.To4().String()
		} else {
			//Decode using 32 bits of magicCookie and 96 bits of transactionID
			addressBytesXOR = webtools.XORArrays(addressBytesXOR, append(constMagicCookie[:], attribute.TransactionID...))

			//Write results
			result["family"] = "IPv6"
			address := net.IP(addressBytesXOR)
			result["address"] = address.To16().String()
		}
		return result, nil
	}

	return nil, os.ErrInvalid
}

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
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(message)))

	//Last 2 bits must be 0
	webtools.ClearBit(packet[3], 6)
	webtools.ClearBit(packet[3], 7)

	//Write Magic Cookie
	packet[4] = constMagicCookie[0]
	packet[5] = constMagicCookie[1]
	packet[6] = constMagicCookie[2]
	packet[7] = constMagicCookie[3]
	//fmt.Println(packet[4:8])

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
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-15
Returns: MessageTypeSTUN, transactionID (as hex), message, error
*/
func UnpackSTUNPacket(data []byte, isIPv4 bool) (MessageTypeSTUN, MessageClassSTUN, string, []STUNPacketAttribute, error) {
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
	messageLength := binary.BigEndian.Uint16(data[2:4])

	//Get Magic Cookie = 32 bits + check
	fmt.Println(data[4:8])
	fmt.Println(constMagicCookie[:])
	if !slices.Equal(data[4:8], constMagicCookie[:]) {
		return 0, 0, "", nil, errors.New("invalid MagicCookie")
	}

	//Get Transaction ID = 96 bits
	transactionID := hex.EncodeToString(data[8:20])

	//Get Message
	fmt.Println(messageLength)
	messageBuffer := bytes.NewBuffer(data[20 : 20+messageLength])

	//Unpack atributes
	resultAttributes := make([]STUNPacketAttribute, 0)
	for {
		//Read attribute Type
		attributeTypeBytes := make([]byte, 2)
		_, err := messageBuffer.Read(attributeTypeBytes)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return 0, 0, "", nil, err
		}
		attributeType := binary.BigEndian.Uint16(attributeTypeBytes)

		//Get attribute Length
		attributeLengthBytes := make([]byte, 2)
		_, err = messageBuffer.Read(attributeLengthBytes)
		if err != nil {
			return 0, 0, "", nil, err
		}
		attributeLength := binary.BigEndian.Uint16(attributeLengthBytes)

		//Get attribute Value
		attributeValueBytes := make([]byte, attributeLength)
		_, err = messageBuffer.Read(attributeValueBytes)
		if err != nil {
			return 0, 0, "", nil, err
		}

		//Align to end
		for i := 4 - attributeLength%4; i < 4; i++ {
			_, err := messageBuffer.ReadByte()
			if err != nil {
				return 0, 0, "", nil, err
			}
		}

		//Check if already contains this attribute Type
		found := false
		for _, attribute := range resultAttributes {
			if attribute.Type == attributeType {
				found = true
				break
			}
		}
		if !found {
			resultAttributes = append(resultAttributes, STUNPacketAttribute{Type: attributeType, Data: attributeValueBytes, TransactionID: data[8:20]})
		}
	}
	return MessageTypeSTUN(messageType), MessageClassSTUN(messageClass), transactionID, resultAttributes, nil
}
