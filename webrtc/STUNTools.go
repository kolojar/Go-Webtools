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
type STUNPacketAttributeType uint16

const MessageTypeSTUNBinding MessageTypeSTUN = 0b000000000001

const MessageClassSTUNRequest MessageClassSTUN = 0b00
const MessageClassSTUNIndication MessageClassSTUN = 0b01
const MessageClassSTUNSuccessResponse MessageClassSTUN = 0b10
const MessageClassSTUNErrorResponse MessageClassSTUN = 0b11

// Data: family = string = IPv4 / IPv6; port = uint16; address = string
const STUNPacketAttributeTypeMappedAddress STUNPacketAttributeType = 0x0001

// Data: family = string = IPv4 / IPv6; port = uint16; address = string
const STUNPacketAttributeTypeXORMappedAddress STUNPacketAttributeType = 0x0020

type STUNPacketAttribute struct {
	Type          STUNPacketAttributeType
	Data          []byte
	TransactionID []byte
}

type STUNPacketDecodedAttribute struct {
	Type          STUNPacketAttributeType
	Data          map[string]any
	TransactionID []byte //When creating should be nil
}

func (decodedAttribute *STUNPacketDecodedAttribute) Print() {
	fmt.Println("Decoded attribute:")
	for k, v := range decodedAttribute.Data {
		fmt.Println(" -", k, v)
	}
}

/*
EncodeSTUNPacketAttribute encodes type and data to STUNPacketAttribute
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-18.2
*/
func EncodeSTUNPacketAttribute(data STUNPacketDecodedAttribute, transactionID []byte) (STUNPacketAttribute, error) {
	attribute := STUNPacketAttribute{Type: data.Type, Data: nil, TransactionID: data.TransactionID}
	if data.TransactionID == nil && transactionID == nil {
		return attribute, errors.New("invalid transaction ID")
	}
	if data.Type == STUNPacketAttributeTypeMappedAddress {
		//Mapped address
		//Parse address
		address := net.ParseIP(data.Data["address"].(string))
		if address == nil {
			return attribute, errors.New("invalid address")
		}

		//Parse family
		isIPv4 := false
		if data.Data["family"].(string) == "IPv4" {
			isIPv4 = true
		} else if data.Data["family"].(string) == "IPv6" {
			isIPv4 = false
		} else {
			return attribute, errors.New("invalid family address: must be IPv4 or IPv6 - inserted: " + data.Data["family"].(string))
		}

		//Check IP address type
		if address.To4() == nil && isIPv4 {
			return attribute, errors.New("invalid ip address")
		}
		if address.To16() == nil && !isIPv4 {
			return attribute, errors.New("invalid ip address")
		}

		//Build data frame - put first 0 and Family
		if isIPv4 {
			attribute.Data = make([]byte, 8)
			attribute.Data[1] = 0x01
		} else {
			attribute.Data = make([]byte, 20)
			attribute.Data[1] = 0x02
		}
		attribute.Data[0] = 0

		//Put port
		binary.BigEndian.PutUint16(attribute.Data[2:4], data.Data["port"].(uint16))

		//Put address
		if isIPv4 {
			ipv4 := address.To4()
			for i := 0; i < 4; i++ {
				attribute.Data[4+i] = ipv4[i]
			}
		} else {
			ipv6 := address.To16()
			for i := 0; i < 16; i++ {
				attribute.Data[4+i] = ipv6[i]
			}
		}
	} else if data.Type == STUNPacketAttributeTypeXORMappedAddress {
		//XOR Mapped address
		//Parse address
		address := net.ParseIP(data.Data["address"].(string))
		if address == nil {
			return attribute, errors.New("invalid address")
		}

		//Parse family
		isIPv4 := false
		if data.Data["family"].(string) == "IPv4" {
			isIPv4 = true
		} else if data.Data["family"].(string) == "IPv6" {
			isIPv4 = false
		} else {
			return attribute, errors.New("invalid family address: must be IPv4 or IPv6 - inserted: " + data.Data["family"].(string))
		}

		//Check IP address type
		if address.To4() == nil && isIPv4 {
			return attribute, errors.New("invalid ip address")
		}
		if address.To16() == nil && !isIPv4 {
			return attribute, errors.New("invalid ip address")
		}

		//Build data frame - put first 0 and Family
		if isIPv4 {
			attribute.Data = make([]byte, 8)
			attribute.Data[1] = 0x01
		} else {
			attribute.Data = make([]byte, 20)
			attribute.Data[1] = 0x02
		}
		attribute.Data[0] = 0

		//Put port + XOR
		binary.BigEndian.PutUint16(attribute.Data[2:4], data.Data["port"].(uint16))
		copy(attribute.Data[2:4], webtools.XORArrays(attribute.Data[2:4], constMagicCookie[0:2]))

		//Put address + XOR
		if isIPv4 {
			ipv4 := address.To4()
			for i := 0; i < 4; i++ {
				attribute.Data[4+i] = ipv4[i]
			}
			copy(attribute.Data[4:8], webtools.XORArrays(attribute.Data[4:8], constMagicCookie[:]))
		} else {
			ipv6 := address.To16()
			for i := 0; i < 16; i++ {
				attribute.Data[4+i] = ipv6[i]
			}
			copy(attribute.Data[4:20], webtools.XORArrays(attribute.Data[4:20], append(constMagicCookie[:], attribute.TransactionID...)))
		}
		return attribute, nil
	}
	return attribute, os.ErrInvalid
}

/*
DecodeSTUNPacketAttribute decodes STUNPacketAttribute using type and returns decoded value
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-18.2
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-15
*/
func (attribute *STUNPacketAttribute) DecodeSTUNPacketAttribute() (STUNPacketDecodedAttribute, error) {
	result := STUNPacketDecodedAttribute{Type: attribute.Type, TransactionID: attribute.TransactionID, Data: make(map[string]any)}
	if attribute.Type == STUNPacketAttributeTypeMappedAddress {
		//Mapped address
		//Check for length
		if len(attribute.Data) < 4 {
			return result, errors.New("data to short for mapped address")
		}

		//First byte must be 0
		if attribute.Data[0] != 0 {
			return result, errors.New("invalid first byte of attribute")
		}

		//Get IP family
		family := attribute.Data[1]
		if family != 1 && family != 2 {
			return result, errors.New("invalid ip family")
		}
		addressLength := webtools.FormatByBool(family == 1, 4, 16)

		//Check for length
		if len(attribute.Data) < 4+addressLength {
			return result, errors.New("data to short for mapped address")
		}

		//Get port
		port := binary.BigEndian.Uint16(attribute.Data[2:4])

		//Get address
		address := net.IP(attribute.Data[4:(4 + addressLength)])

		//Make result map - Specified in: https://datatracker.ietf.org/doc/html/rfc5389#section-15
		result.Data["family"] = webtools.FormatByBool(addressLength == 4, "IPv4", "IPv6")
		result.Data["port"] = port
		if addressLength == 4 {
			result.Data["address"] = address.To4().String()
		} else {
			result.Data["address"] = address.To16().String()
		}
		return result, nil
	}
	if attribute.Type == STUNPacketAttributeTypeXORMappedAddress {
		//XOR Mapped address
		//Check for length
		if len(attribute.Data) < 4 {
			return result, errors.New("data to short for mapped address")
		}

		//First byte must be 0
		if attribute.Data[0] != 0 {
			return result, errors.New("invalid first byte of attribute")
		}

		//Get IP family
		family := attribute.Data[1]
		if family != 1 && family != 2 {
			return result, errors.New("invalid ip family")
		}
		addressLength := webtools.FormatByBool(family == 1, 4, 16)

		//Check for length
		if len(attribute.Data) < 4+addressLength {
			return result, errors.New("data to short for mapped address")
		}

		//Get port = 16 bits XOR
		port := binary.BigEndian.Uint16(webtools.XORArrays(attribute.Data[2:4], constMagicCookie[0:2]))

		//Get address
		addressBytesXOR := attribute.Data[4:(4 + addressLength)]

		//Make result map - Specified in: https://datatracker.ietf.org/doc/html/rfc5389#section-15
		result.Data["family"] = webtools.FormatByBool(addressLength == 4, "IPv4", "IPv6")
		result.Data["port"] = port
		if addressLength == 4 {
			//Decode using 32 bits of magicCookie
			addressBytesXOR = webtools.XORArrays(addressBytesXOR, constMagicCookie[:])

			//Write results
			address := net.IP(addressBytesXOR)
			result.Data["address"] = address.To4().String()
		} else {
			//Decode using 32 bits of magicCookie and 96 bits of transactionID
			addressBytesXOR = webtools.XORArrays(addressBytesXOR, append(constMagicCookie[:], attribute.TransactionID...))

			//Write results
			address := net.IP(addressBytesXOR)
			result.Data["address"] = address.To16().String()
		}
		return result, nil
	}

	return result, os.ErrInvalid
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-6
Returns transtactionID, packet, error
*/
func PackSTUNPacket(messageType MessageTypeSTUN, messageClass MessageClassSTUN, attributes []STUNPacketDecodedAttribute) (string, []byte, error) {
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
	packet[0] = webtools.SetBitValue(packet[0], 7, webtools.CheckBit(byte(messageClass), 6))
	packet[1] = webtools.SetBitValue(packet[1], 0, webtools.CheckBit(messageTypeAsBytes[1], 1))
	packet[1] = webtools.SetBitValue(packet[1], 1, webtools.CheckBit(messageTypeAsBytes[1], 2))
	packet[1] = webtools.SetBitValue(packet[1], 2, webtools.CheckBit(messageTypeAsBytes[1], 3))
	packet[1] = webtools.SetBitValue(packet[1], 3, webtools.CheckBit(byte(messageClass), 7))
	packet[1] = webtools.SetBitValue(packet[1], 4, webtools.CheckBit(messageTypeAsBytes[1], 4))
	packet[1] = webtools.SetBitValue(packet[1], 5, webtools.CheckBit(messageTypeAsBytes[1], 5))
	packet[1] = webtools.SetBitValue(packet[1], 6, webtools.CheckBit(messageTypeAsBytes[1], 6))
	packet[1] = webtools.SetBitValue(packet[1], 7, webtools.CheckBit(messageTypeAsBytes[1], 7))
	fmt.Println(packet[0:2])
	fmt.Println("message type:", messageType, "message class:", messageClass)

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

	//Encode attributes
	messageBuffer := bytes.NewBuffer(make([]byte, 0))
	for _, attribute := range attributes {
		//Encode attribute
		attributeEncoded, err := EncodeSTUNPacketAttribute(attribute, packet[8:20])
		if err != nil {
			return "", nil, err
		}

		//Write attribute Type
		attributeTypeBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(attributeTypeBytes, uint16(attribute.Type))
		messageBuffer.Write(attributeTypeBytes)

		//Write attribute Length
		attributeLengthBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(attributeLengthBytes, uint16(len(attributeEncoded.Data)))
		messageBuffer.Write(attributeLengthBytes)

		//Write attribute
		messageBuffer.Write(attributeEncoded.Data)

		//Write pading
		for i := 4 - len(attributeEncoded.Data)%4; i < 4; i++ {
			messageBuffer.WriteByte(0)
		}
	}

	//Write Message Length = 16 bits
	message := messageBuffer.Bytes()
	fmt.Println(message)
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(message)))

	//Last 2 bits must be 0
	webtools.ClearBit(packet[3], 6)
	webtools.ClearBit(packet[3], 7)

	//Write Message
	packet = append(packet, message...)
	return hex.EncodeToString(packet[8:20]), packet, nil
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-6
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-15
Returns: MessageTypeSTUN, transactionID (as hex), message,isStunPacket error
*/
func UnpackSTUNPacket(data []byte, isIPv4 bool) (MessageTypeSTUN, MessageClassSTUN, string, []STUNPacketAttribute, bool, error) {
	//Check length
	if len(data) < 20 || len(data) > webtools.FormatByBool(isIPv4, 576, 1280) {
		return 0, 0, "", nil, false, errors.New("invalid packet size")
	}

	//Check if first 2 bits are 0
	if webtools.CheckBitArray(data, 0) || webtools.CheckBitArray(data, 1) {
		return 0, 0, "", nil, false, errors.New("invalid bit 0 or 1")
	}

	//Get STUN Message Type + STUN Message Class = 14 bits
	messageTypeAsBytes := make([]byte, 2)
	messageClass := uint8(0)
	messageTypeAsBytes[0] = webtools.SetBitValue(messageTypeAsBytes[0], 4, webtools.CheckBit(data[0], 2))
	messageTypeAsBytes[0] = webtools.SetBitValue(messageTypeAsBytes[0], 5, webtools.CheckBit(data[0], 3))
	messageTypeAsBytes[0] = webtools.SetBitValue(messageTypeAsBytes[0], 6, webtools.CheckBit(data[0], 4))
	messageTypeAsBytes[0] = webtools.SetBitValue(messageTypeAsBytes[0], 7, webtools.CheckBit(data[0], 5))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 0, webtools.CheckBit(data[0], 6))
	messageClass = webtools.SetBitValue(messageClass, 6, webtools.CheckBit(data[0], 7))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 1, webtools.CheckBit(data[1], 0))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 2, webtools.CheckBit(data[1], 1))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 3, webtools.CheckBit(data[1], 2))
	messageClass = webtools.SetBitValue(messageClass, 7, webtools.CheckBit(data[1], 3))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 4, webtools.CheckBit(data[1], 4))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 5, webtools.CheckBit(data[1], 5))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 6, webtools.CheckBit(data[1], 6))
	messageTypeAsBytes[1] = webtools.SetBitValue(messageTypeAsBytes[1], 7, webtools.CheckBit(data[1], 7))
	messageType := binary.BigEndian.Uint16(messageTypeAsBytes)
	fmt.Println("message type:", messageType, "message class:", messageClass)

	//Check Message Length = 16 bits - last 2 bits must be 0
	if webtools.CheckBit(data[3], 6) || webtools.CheckBit(data[3], 7) {
		return 0, 0, "", nil, false, errors.New("invalid MessageLength suffix")
	}

	//Get Message Length = 16 bits
	messageLength := binary.BigEndian.Uint16(data[2:4])
	if webtools.CheckBit(data[3], 6) || webtools.CheckBit(data[3], 7) {
		return 0, 0, "", nil, false, errors.New("invalid bits in length")
	}

	//Get Magic Cookie = 32 bits + check
	if !slices.Equal(data[4:8], constMagicCookie[:]) {
		return 0, 0, "", nil, false, errors.New("invalid MagicCookie")
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
			return 0, 0, "", nil, true, err
		}
		attributeType := STUNPacketAttributeType(binary.BigEndian.Uint16(attributeTypeBytes))

		//Get attribute Length
		attributeLengthBytes := make([]byte, 2)
		_, err = messageBuffer.Read(attributeLengthBytes)
		if err != nil {
			return 0, 0, "", nil, true, err
		}
		attributeLength := binary.BigEndian.Uint16(attributeLengthBytes)

		//Get attribute Value
		attributeValueBytes := make([]byte, attributeLength)
		_, err = messageBuffer.Read(attributeValueBytes)
		if err != nil {
			return 0, 0, "", nil, true, err
		}

		//Align to end
		for i := 4 - attributeLength%4; i < 4; i++ {
			_, err := messageBuffer.ReadByte()
			if err != nil {
				return 0, 0, "", nil, true, err
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

	//Check for FINGERPRINT attribute = TODO

	return MessageTypeSTUN(messageType), MessageClassSTUN(messageClass), transactionID, resultAttributes, true, nil
}
