package webrtc

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"webtools"
)

var constSDPSessionDescriptionOptionalTypes1 = []string{"i", "u", "e", "p", "c", "b"}
var constSDPSessionDescriptionOptionalTypes2 = []string{"z", "k", "a"}
var constSDPMediaDescriptionOptionalTypes = []string{"i", "c", "b", "k", "a"}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5
*/
type SDPMessage struct {
	ProtocolVersion       string                       //v
	Origin                SDPMessageOrigin             //o
	SessionName           string                       //s
	SessionInformation    string                       //i*
	URIOfDescription      string                       //u*
	EmailAddresses        []string                     //e*
	PhoneNumbers          []string                     //p*
	ConnectionData        SDPMessageConnectionData     //c*
	BandwidthInformations []SDPMessageBandwidth        //b*
	TimeDescriptions      []SDPMessageTiming           //complex = t + r*
	TimeZoneAdjustments   []SDPMessageTimeZone         //z*
	EncryptionKey         SDPMessageEncyptionKey       //k*
	Attributes            []SDPMessageAttribute        //a*
	MediaDescriptions     []SDPMessageMediaDescription //m and optional attributes for m: i*, c*, b*, k*, a*
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.2
*/
type SDPMessageOrigin struct {
	Username       string //username
	SessionID      string //sess-id
	SessionVersion string //sess-version
	NetworkType    string //nettype
	AddressType    string //addrtype
	UnicastAddress string //unicast-address
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.7
*/
type SDPMessageConnectionData struct {
	NetworkType       string //nettype
	AddressType       string //addrtype
	ConnectionAddress string //connection-address
	NumberOfAddresses int    //number of addresses
	TimeToLive        uint8  //ttl
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.8
*/
type SDPMessageBandwidth struct {
	BandwidthType string //bwtype
	Bandwidth     string //bandwidth
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.9
*/
type SDPMessageTiming struct {
	StartTime   string                 //t -> start-time
	EndTime     string                 //t -> stop-time
	RepeatTimes []SDPMessageRepeatTime //r*
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.10
*/
type SDPMessageRepeatTime struct {
	RepeatInterval string   //repeat interval
	ActiveDuration string   //active duration
	Offsets        []string //offsets from start-time
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.11
*/
type SDPMessageTimeZone struct {
	AdjustmentTime string //adjustment time
	Offset         string //offset
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.12
*/
type SDPMessageEncyptionKey struct {
	Method        string //method
	EncryptionKey string //encryption key
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.13
*/
type SDPMessageAttribute struct {
	Attribute string //attribute
	Value     string //value
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.14
*/
type SDPMessageMediaDescription struct {
	Media                 string                   //m - media
	Port                  uint16                   //m - port
	NumberOfPorts         uint16                   //m - number of ports
	Protocol              string                   //m - proto
	Formats               []string                 //m - fmt
	MediaTitle            string                   //i*
	ConnectionData        SDPMessageConnectionData //c*
	BandwidthInformations []SDPMessageBandwidth    //b*
	EncryptionKey         SDPMessageEncyptionKey   //k*
	Attributes            []SDPMessageAttribute    //a*
}

func getSDPMessageLineValue(line string, lineType string) (string, error) {
	//Split line to SDP format <type>=<value>
	split := strings.SplitN(line, "=", 2)

	//Check length
	if len(split) <= 0 {
		return "", errors.New("message line too short")
	}
	if len(split) == 1 {
		return split[0], errors.New("message line value empty or invalid")
	}
	if len(split) > 2 {
		return split[0], errors.New("message line too long")
	}

	//Check type
	if split[0] != lineType {
		return split[0], os.ErrInvalid
	}
	return split[1], nil
}

func getSDPMessageOptionalLineValue(line string, lineType string, endLineType string) (string, bool, error) {
	//Get value
	value, err := getSDPMessageLineValue(line, lineType)
	if err != nil {
		if errors.Is(err, os.ErrInvalid) {
			//Different type
			if value == endLineType {
				//Got to end line type
				return value, true, nil
			} else {
				//Other value but not end
				return value, false, err
			}
		}
		//Other error
		return value, false, err
	}

	//Valid value
	return value, false, nil
}

func unpackSDPMessageBroadcastLine(value string) (SDPMessageBandwidth, error) {
	bandwidthSplit := strings.SplitN(value, ":", 2)
	if len(bandwidthSplit) != 2 {
		return SDPMessageBandwidth{}, errors.New("invalid bandwidth length")
	}

	//Add value
	return SDPMessageBandwidth{BandwidthType: bandwidthSplit[0], Bandwidth: bandwidthSplit[1]}, nil
}

func unpackSDPMessageConnectionInformationLine(value string) (SDPMessageConnectionData, error) {
	connectionData := SDPMessageConnectionData{}

	//Process connection data
	valueSplit := strings.SplitN(value, " ", 3)
	if len(valueSplit) != 3 {
		return connectionData, errors.New("invalid connectionData length")
	}
	connectionData.NetworkType = valueSplit[0]
	connectionData.AddressType = valueSplit[1]

	//Process connectionAddress subfield
	connectionAddressSplit := strings.SplitN(valueSplit[2], "/", 3)
	if len(connectionAddressSplit) < 1 {
		return connectionData, errors.New("invalid connectionData-connectionAddress length")
	}

	//Parse IP
	ip, err := netip.ParseAddr(connectionAddressSplit[0])
	if err != nil {
		return connectionData, err
	}
	repeatCount := 1

	//Sort IP types
	switch connectionData.AddressType {
	case "IP4":
		//IPv4 - check
		if !ip.Is4() {
			return connectionData, errors.New("invalid connectionData-connectionAddress - wants ipv4 and got ipv6")
		}

		//Check length
		if len(connectionAddressSplit) == 1 {
			fmt.Println("Warning: connectionData-connectionAddress - No TTL specified")
			connectionData.ConnectionAddress = ip.String()
		}
		if len(connectionAddressSplit) >= 2 {
			//Format: IP/TTL
			connectionData.ConnectionAddress = ip.String()

			//Convert TTL
			ttl, err := strconv.ParseUint(connectionAddressSplit[1], 10, 8)
			if err != nil {
				return connectionData, err
			}
			connectionData.TimeToLive = uint8(ttl)
		}
		if len(connectionAddressSplit) == 3 {
			//Repeat count
			repeatCount, err = strconv.Atoi(connectionAddressSplit[2])
			if err != nil {
				return connectionData, err
			}
		}
	case "IP6":
		//IPv6 - check
		if !ip.Is6() {
			return connectionData, errors.New("invalid connectionData-connectionAddress - wants ipv6 and got ipv4")
		}

		//Check length
		if len(connectionAddressSplit) == 1 {
			//Format: IP
			connectionData.ConnectionAddress = ip.String()
		}
		if len(connectionAddressSplit) == 2 {
			//Repeat count
			repeatCount, err = strconv.Atoi(connectionAddressSplit[1])
			if err != nil {
				return connectionData, err
			}
		}
	}

	//Repeat IP
	connectionData.NumberOfAddresses = repeatCount
	return connectionData, nil
}

func unpackSDPMessageEncryptionKeyLine(value string) (SDPMessageEncyptionKey, error) {
	encryptionKeySplit := strings.SplitN(value, " ", 2)
	if len(encryptionKeySplit) < 1 {
		return SDPMessageEncyptionKey{}, errors.New("invalid encryption key length")
	}

	//Add Encryption Key
	encryptionKey := SDPMessageEncyptionKey{}
	if len(encryptionKeySplit) >= 1 {
		encryptionKey.Method = encryptionKeySplit[0]
	}
	if len(encryptionKeySplit) == 2 {
		encryptionKey.EncryptionKey = encryptionKeySplit[1]
	}
	return encryptionKey, nil
}

func unpackSDPMessageAttributeLine(value string) (SDPMessageAttribute, error) {
	attributeSplit := strings.SplitN(value, ":", 2)
	if len(attributeSplit) < 1 {
		return SDPMessageAttribute{}, errors.New("invalid attribute length")
	}

	//Add Attribute
	attribute := SDPMessageAttribute{}
	if len(attributeSplit) >= 1 {
		attribute.Attribute = attributeSplit[0]
	}
	if len(attributeSplit) == 2 {
		attribute.Value = attributeSplit[1]
	}
	return attribute, nil
}

/*
UnpackSDPMessage converts string message to SDPMessage struct with error checking
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5
Other specifications on sub-structs
*/
func UnpackSDPMessage(message string) (SDPMessage, error) {
	//Split to lines
	messageLines := make([]string, 0)
	for line := range strings.Lines(message) {
		messageLines = append(messageLines, strings.ReplaceAll(line, "\n", ""))
	}
	sdpMessage := SDPMessage{}
	var err error

	//Check length
	if len(messageLines) < 3 {
		return sdpMessage, errors.New("message to short")
	}

	//Read Protocol Version
	sdpMessage.ProtocolVersion, err = getSDPMessageLineValue(messageLines[0], "v")
	if err != nil {
		return sdpMessage, err
	}

	//Read Origin
	originValue, err := getSDPMessageLineValue(messageLines[1], "o")
	if err != nil {
		return sdpMessage, err
	}

	//Process origin
	originSplit := strings.Split(originValue, " ")
	if len(originSplit) != 6 {
		return sdpMessage, errors.New("invalid origin length")
	}
	sdpMessage.Origin = SDPMessageOrigin{
		Username:       originSplit[0],
		SessionID:      originSplit[1],
		SessionVersion: originSplit[2],
		NetworkType:    originSplit[3],
		AddressType:    originSplit[4],
		UnicastAddress: originSplit[5],
	}

	//Read Session Name
	sdpMessage.SessionName, err = getSDPMessageLineValue(messageLines[2], "s")
	if err != nil {
		return sdpMessage, err
	}

	//Validate session name
	if sdpMessage.SessionName == "" {
		return sdpMessage, errors.New("empty sessionName")
	}

	//Read optional types 1
	lineNumber := 3
	foundConnectionDataInSessionDescription := false
	for i := 0; i < len(constSDPSessionDescriptionOptionalTypes1); i++ {
		t := constSDPSessionDescriptionOptionalTypes1[i]
		//Check line number
		if lineNumber >= len(messageLines) {
			//End of lines
			break
		}

		//Read value
		value, gotEndType, err := getSDPMessageOptionalLineValue(messageLines[lineNumber], t, "t")
		if gotEndType {
			break
		}
		if err != nil {
			if errors.Is(err, os.ErrInvalid) {
				continue
			} else {
				return sdpMessage, err
			}
		}

		//Process loaded value
		lineNumber++
		if t == "i" {
			//Session information
			sdpMessage.SessionInformation = value
			continue
		}
		if t == "u" {
			//URI
			sdpMessage.URIOfDescription = value
			continue
		}
		if t == "e" {
			//Email (can be multiple)
			i--
			sdpMessage.EmailAddresses = append(sdpMessage.EmailAddresses, value)
			continue
		}
		if t == "p" {
			//Phone number (can be multiple)
			i--
			sdpMessage.PhoneNumbers = append(sdpMessage.PhoneNumbers, value)
			continue
		}
		if t == "c" {
			//ConnectionData
			foundConnectionDataInSessionDescription = true
			sdpMessage.ConnectionData, err = unpackSDPMessageConnectionInformationLine(value)
			if err != nil {
				return sdpMessage, err
			}
			continue
		}
		if t == "b" {
			//Bandwidth - process (can be multiple)
			bandwidth, err := unpackSDPMessageBroadcastLine(value)
			if err != nil {
				return sdpMessage, err
			}

			//Add value
			sdpMessage.BandwidthInformations = append(sdpMessage.BandwidthInformations, bandwidth)
			i--
			continue
		}
	}

	//Read Time descriptions
	foundFirstTimeType := false
	for {
		//Get Time type
		timeTypeValue, err := getSDPMessageLineValue(messageLines[lineNumber], "t")
		if err != nil {
			if errors.Is(err, os.ErrInvalid) {
				if !foundFirstTimeType {
					return sdpMessage, errors.New("invalid type - needs t type")
				}
				break
			} else {
				return sdpMessage, err
			}
		}

		//Split Time type
		foundFirstTimeType = true
		timeTypeValueSplit := strings.SplitN(timeTypeValue, " ", 2)
		if len(timeTypeValueSplit) != 2 {
			return sdpMessage, errors.New("invalid time length")
		}
		lineNumber++

		//Try to get RepeatTimes type
		repeatTimes := make([]SDPMessageRepeatTime, 0)
		for {
			repeatTimesTypeValue, err := getSDPMessageLineValue(messageLines[lineNumber], "r")
			if err != nil {
				if !errors.Is(err, os.ErrInvalid) {
					return sdpMessage, err
				}
				break
			} else {
				lineNumber++
			}

			//Split RepeatTime type
			repeatTimesTypeValueSplit := strings.SplitN(repeatTimesTypeValue, " ", 2)
			if len(repeatTimesTypeValueSplit) < 3 {
				return sdpMessage, errors.New("invalid repeat time length")
			}
			repeatTimes = append(repeatTimes, SDPMessageRepeatTime{RepeatInterval: repeatTimesTypeValueSplit[0], ActiveDuration: repeatTimesTypeValueSplit[1], Offsets: repeatTimesTypeValueSplit[2:]})
		}

		//Add Time Description
		sdpMessage.TimeDescriptions = append(sdpMessage.TimeDescriptions, SDPMessageTiming{
			StartTime:   timeTypeValueSplit[0],
			EndTime:     timeTypeValueSplit[1],
			RepeatTimes: repeatTimes,
		})
	}

	//Read optional types 2
	sdpMessage.Attributes = make([]SDPMessageAttribute, 0)
	for i := 0; i < len(constSDPSessionDescriptionOptionalTypes2); i++ {
		t := constSDPSessionDescriptionOptionalTypes2[i]
		//Check line number
		if lineNumber >= len(messageLines) {
			//End of lines
			break
		}

		//Read value
		value, gotEndType, err := getSDPMessageOptionalLineValue(messageLines[lineNumber], t, "m")
		if gotEndType {
			break
		}
		if err != nil {
			if errors.Is(err, os.ErrInvalid) {
				continue
			} else {
				return sdpMessage, err
			}
		}

		//Process loaded value
		lineNumber++
		if t == "z" {
			//Time zones
			timeZonesSplit := strings.Split(value, " ")
			if len(timeZonesSplit)%2 != 0 || len(timeZonesSplit) < 2 {
				return sdpMessage, errors.New("invalid time zones length")
			}

			//Add Time zones information
			sdpMessage.TimeZoneAdjustments = make([]SDPMessageTimeZone, 0)
			for i := 0; i < len(timeZonesSplit); i += 2 {
				sdpMessage.TimeZoneAdjustments = append(sdpMessage.TimeZoneAdjustments, SDPMessageTimeZone{AdjustmentTime: timeZonesSplit[i], Offset: timeZonesSplit[i+1]})
			}
			continue
		}
		if t == "k" {
			//Encryption key
			sdpMessage.EncryptionKey, err = unpackSDPMessageEncryptionKeyLine(value)
			if err != nil {
				return sdpMessage, err
			}
			continue
		}
		if t == "a" {
			//Attribute (can be multiple)
			attribute, err := unpackSDPMessageAttributeLine(value)
			if err != nil {
				return sdpMessage, err
			}
			sdpMessage.Attributes = append(sdpMessage.Attributes, attribute)
			i--
			continue
		}
	}

	//Read optional media types
	for {
		//Check line number
		if lineNumber >= len(messageLines) {
			//End of lines
			break
		}

		//Read value
		value, err := getSDPMessageLineValue(messageLines[lineNumber], "m")
		if err != nil {
			return sdpMessage, err
		}

		//Process loaded value
		mediaSplit := strings.Split(value, " ")
		if len(mediaSplit) < 4 {
			return sdpMessage, errors.New("invalid media length")
		}

		//Process port
		portSplit := strings.SplitN(mediaSplit[1], "/", 2)
		if len(portSplit) < 1 {
			return sdpMessage, errors.New("invalid media - port length")
		}

		//Parse port
		port, err := strconv.ParseUint(portSplit[0], 10, 16)
		if err != nil {
			return sdpMessage, err
		}
		if port == 0 {
			return sdpMessage, errors.New("invalid media - port value")
		}

		//Parse numberOfPorts
		var numberOfPorts uint64 = 1
		if len(portSplit) == 2 {
			numberOfPorts, err = strconv.ParseUint(portSplit[1], 10, 16)
			if err != nil {
				return sdpMessage, err
			}
			if numberOfPorts == 0 {
				return sdpMessage, errors.New("invalid media - numberOfPorts value")
			}
		}

		mediaType := SDPMessageMediaDescription{
			Media:         mediaSplit[0],
			Port:          uint16(port),
			NumberOfPorts: uint16(numberOfPorts),
			Protocol:      mediaSplit[2],
			Formats:       mediaSplit[3:],
			Attributes:    make([]SDPMessageAttribute, 0),
		}
		lineNumber++

		//Read optional values
		foundConnectionInformation := false
		for i := 0; i < len(constSDPMediaDescriptionOptionalTypes); i++ {
			t := constSDPMediaDescriptionOptionalTypes[i]
			//Check line number
			if lineNumber >= len(messageLines) {
				//End of lines
				break
			}

			//Read value
			value, gotEndType, err := getSDPMessageOptionalLineValue(messageLines[lineNumber], t, "m")
			if gotEndType {
				break
			}
			if err != nil {
				if errors.Is(err, os.ErrInvalid) {
					continue
				} else {
					return sdpMessage, err
				}
			}

			//Process loaded value
			lineNumber++
			if t == "i" {
				//Media title
				mediaType.MediaTitle = value
				continue
			}
			if t == "c" {
				//Connection data
				foundConnectionInformation = true
				mediaType.ConnectionData, err = unpackSDPMessageConnectionInformationLine(value)
				if err != nil {
					return sdpMessage, err
				}
				continue
			}
			if t == "b" {
				//Bandwidth - process (can be multiple)
				bandwidth, err := unpackSDPMessageBroadcastLine(value)
				if err != nil {
					return sdpMessage, err
				}

				//Add value
				mediaType.BandwidthInformations = append(sdpMessage.BandwidthInformations, bandwidth)
				i--
				continue
			}
			if t == "k" {
				//Encryption key
				mediaType.EncryptionKey, err = unpackSDPMessageEncryptionKeyLine(value)
				if err != nil {
					return sdpMessage, err
				}
				continue
			}
			if t == "a" {
				//Attribute (can be multiple)
				attribute, err := unpackSDPMessageAttributeLine(value)
				if err != nil {
					return sdpMessage, err
				}
				mediaType.Attributes = append(mediaType.Attributes, attribute)
				i--
				continue
			}
		}
		if !foundConnectionDataInSessionDescription && !foundConnectionInformation {
			//No connection specified
			return sdpMessage, errors.New("no connection specified")
		}
		sdpMessage.MediaDescriptions = append(sdpMessage.MediaDescriptions, mediaType)
	}
	return sdpMessage, nil
}

const sdpLineEnd = "\r\n"

func isInvalidSDPValue(value string) bool {
	return webtools.IsStringEmpty(value) || strings.ContainsAny(value, " \n\r")
}

func packSDPMessageConnectionInformationLine(connectionData SDPMessageConnectionData, errorPrefix string) (string, error) {
	result := ""
	if connectionData.NetworkType != "" && connectionData.AddressType != "" {
		//First same part
		if isInvalidSDPValue(connectionData.AddressType) {
			return "", errors.New(errorPrefix + "connectionData - addressType invalid")
		}
		if isInvalidSDPValue(connectionData.NetworkType) {
			return "", errors.New(errorPrefix + "connectionData - networkType invalid")
		}
		if isInvalidSDPValue(connectionData.ConnectionAddress) {
			return "", errors.New(errorPrefix + "connectionData - networkType invalid")
		}
		if connectionData.NumberOfAddresses < 1 {
			return "", errors.New(errorPrefix + "connectionData - NumberOfAddresses too small")
		}
		result = "c=" + connectionData.AddressType + " " + connectionData.NetworkType + " " + connectionData.ConnectionAddress

		//Write TTL for IPv4
		if connectionData.NetworkType == "IP4" {
			result += "/" + strconv.FormatUint(uint64(connectionData.TimeToLive), 10)
		}

		//Write NumberOfAddresses if needed
		if connectionData.NumberOfAddresses != 1 {
			result += "/" + strconv.Itoa(connectionData.NumberOfAddresses)
		}
		result += sdpLineEnd
	}
	return result, nil
}

func packSDPMessageBandwithInformationLine(bandwidth SDPMessageBandwidth, errorPrefix string) (string, error) {
	if isInvalidSDPValue(bandwidth.BandwidthType) {
		return "", errors.New(errorPrefix + "bandwidth - bandwidthType empty or invalid")
	}
	if isInvalidSDPValue(bandwidth.Bandwidth) {
		return "", errors.New(errorPrefix + "bandwidth - Bandwidth empty or invalid")
	}
	return "b=" + bandwidth.BandwidthType + ":" + bandwidth.Bandwidth + sdpLineEnd, nil
}

func packSDPMessageEncryptionKeyLine(encryptionKey SDPMessageEncyptionKey, errorPrefix string) (string, error) {
	result := ""
	if encryptionKey.Method != "" {
		if isInvalidSDPValue(encryptionKey.Method) {
			return "", errors.New(errorPrefix + "encryptionKey - method invalid")
		}
		result += encryptionKey.Method

		//Put optional encryptionKey
		if encryptionKey.EncryptionKey != "" {
			if strings.ContainsAny(encryptionKey.EncryptionKey, "\n\r") {
				return "", errors.New(errorPrefix + "encryptionKey - encryptionKey invalid")
			}
			result += ":" + encryptionKey.EncryptionKey
		}
		result += sdpLineEnd
	}
	return result, nil
}

func packSDPMessageAttributeLine(attribute SDPMessageAttribute, errorPrefix string) (string, error) {
	if isInvalidSDPValue(attribute.Attribute) {
		return "", errors.New(errorPrefix + "attribute - attribute empty or invalid")
	}
	result := "a=" + attribute.Attribute

	//Put optional Value
	if attribute.Value != "" {
		if strings.ContainsAny(attribute.Value, "\n\r") {
			return "", errors.New(errorPrefix + "attribute - value invalid")
		}
		result += ":" + attribute.Value
	}
	result += sdpLineEnd
	return result, nil
}

/*
PackSDPMessage converts SDPMessage struct to crossplatform string message with error checking
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5
Other specifications on sub-structs
*/
func PackSDPMessage(sdpMessage SDPMessage) (string, error) {
	resultMessage := ""

	//Put ProtocolVersion
	if isInvalidSDPValue(sdpMessage.ProtocolVersion) {
		return "", errors.New("protocolVersion empty or invalid")
	}
	resultMessage += "v=" + sdpMessage.ProtocolVersion + sdpLineEnd

	//Put Origin
	if isInvalidSDPValue(sdpMessage.Origin.Username) {
		return "", errors.New("origin - username empty or invalid")
	}
	if isInvalidSDPValue(sdpMessage.Origin.SessionID) {
		return "", errors.New("origin - sessionID empty or invalid")
	}
	if isInvalidSDPValue(sdpMessage.Origin.SessionVersion) {
		return "", errors.New("origin - sessionVersion empty or invalid")
	}
	if isInvalidSDPValue(sdpMessage.Origin.NetworkType) {
		return "", errors.New("origin - networkType empty or invalid")
	}
	if isInvalidSDPValue(sdpMessage.Origin.AddressType) {
		return "", errors.New("origin - addressType empty or invalid")
	}
	if isInvalidSDPValue(sdpMessage.Origin.UnicastAddress) {
		return "", errors.New("origin - unicastAddress empty or invalid")
	}
	resultMessage += "o=" + sdpMessage.Origin.Username + " " + sdpMessage.Origin.SessionID + " " + sdpMessage.Origin.SessionVersion + " " + sdpMessage.Origin.NetworkType + " " + sdpMessage.Origin.AddressType + " " + sdpMessage.Origin.UnicastAddress + sdpLineEnd

	//Put SessionName
	if sdpMessage.SessionName == "" || strings.ContainsAny(sdpMessage.SessionName, "\n\r") {
		return "", errors.New("sessionName empty or invalid")
	}
	resultMessage += "s=" + sdpMessage.SessionName + sdpLineEnd

	//Put sessionInformation - optional
	if sdpMessage.SessionInformation != "" {
		if strings.ContainsAny(sdpMessage.SessionInformation, "\n\r") {
			return "", errors.New("sessionInformation invalid")
		}
		resultMessage += "i=" + sdpMessage.SessionInformation + sdpLineEnd
	}

	//Put uriOfDescription - optional
	if sdpMessage.URIOfDescription != "" {
		if strings.ContainsAny(sdpMessage.URIOfDescription, "\n\r") {
			return "", errors.New("uriOfDescription invalid")
		}
		resultMessage += "u=" + sdpMessage.URIOfDescription + sdpLineEnd
	}

	//Put EmailAddresses - optional
	for _, email := range sdpMessage.EmailAddresses {
		if strings.ContainsAny(email, "\n\r") {
			return "", errors.New("email invalid")
		}
		resultMessage += "e=" + email + sdpLineEnd
	}

	//Put PhoneNumbers - optional
	for _, phone := range sdpMessage.PhoneNumbers {
		if strings.ContainsAny(phone, "\n\r") {
			return "", errors.New("phone invalid")
		}
		resultMessage += "p=" + phone + sdpLineEnd
	}

	//Put connectionData - optional
	wroteConnectionData := false
	connectionInformationMessage, err := packSDPMessageConnectionInformationLine(sdpMessage.ConnectionData, "")
	if err != nil {
		return "", err
	}
	if len(connectionInformationMessage) != 0 {
		wroteConnectionData = true
		resultMessage += connectionInformationMessage
	}

	//Put Bandwidth - optional
	for _, bandwidth := range sdpMessage.BandwidthInformations {
		bandwidthMessage, err := packSDPMessageBandwithInformationLine(bandwidth, "")
		if err != nil {
			return "", nil
		}
		resultMessage += bandwidthMessage
	}

	//Put Time
	if len(sdpMessage.TimeDescriptions) == 0 {
		return "", errors.New("timeDescriptions must have at least one item")
	}
	for _, time := range sdpMessage.TimeDescriptions {
		//Put required Time
		if isInvalidSDPValue(time.StartTime) {
			return "", errors.New("timeDescriptions - startTime empty or invalid")
		}
		if isInvalidSDPValue(time.EndTime) {
			return "", errors.New("timeDescriptions - endTime empty or invalid")
		}
		resultMessage += "t=" + time.StartTime + " " + time.EndTime + sdpLineEnd

		//Put optional RepeatTimes
		for _, repeatTime := range time.RepeatTimes {
			if isInvalidSDPValue(repeatTime.RepeatInterval) {
				return "", errors.New("timeDescriptions - repeatTime - repeatInterval empty or invalid")
			}
			if isInvalidSDPValue(repeatTime.ActiveDuration) {
				return "", errors.New("timeDescriptions - repeatTime - activeDuration empty or invalid")
			}
			if len(repeatTime.Offsets) == 0 {
				return "", errors.New("timeDescriptions - repeatTime - offsets empty")
			}
			resultMessage += "t=" + repeatTime.RepeatInterval + " " + repeatTime.ActiveDuration

			//Put all offsets
			for _, offset := range repeatTime.Offsets {
				resultMessage += offset + " "
			}
			resultMessage = strings.TrimSuffix(resultMessage, " ")
			resultMessage += sdpLineEnd
		}
	}

	//Put Time Zone - optional
	if len(sdpMessage.TimeZoneAdjustments) != 0 {
		resultMessage += "z="
		for _, timeZone := range sdpMessage.TimeZoneAdjustments {
			if isInvalidSDPValue(timeZone.AdjustmentTime) {
				return "", errors.New("timeZone - adjustmentTime empty or invalid")
			}
			if isInvalidSDPValue(timeZone.Offset) {
				return "", errors.New("timeZone - offset empty or invalid")
			}
			resultMessage += timeZone.AdjustmentTime + " " + timeZone.Offset + " "
		}
		resultMessage = strings.TrimSuffix(resultMessage, " ")
		resultMessage += sdpLineEnd
	}

	//Put Encryption Key - optional
	encryptionKeyMessage, err := packSDPMessageEncryptionKeyLine(sdpMessage.EncryptionKey, "")
	if err != nil {
		return "", err
	}
	if len(encryptionKeyMessage) != 0 {
		resultMessage += encryptionKeyMessage
	}

	//Put Attributes - optional
	for _, attribute := range sdpMessage.Attributes {
		attributeMessage, err := packSDPMessageAttributeLine(attribute, "")
		if err != nil {
			return "", err
		}
		resultMessage += attributeMessage
	}

	//Put MediaDescription - optional
	for _, media := range sdpMessage.MediaDescriptions {
		//Check m type
		if isInvalidSDPValue(media.Media) {
			return "", errors.New("mediaDescriptions - media invalid")
		}
		if media.Port == 0 {
			return "", errors.New("mediaDescriptions - port invalid")
		}
		if media.NumberOfPorts == 0 {
			return "", errors.New("mediaDescriptions - numberOfPorts invalid")
		}
		if isInvalidSDPValue(media.Protocol) {
			return "", errors.New("mediaDescriptions - protocol invalid")
		}
		//Put basic data
		resultMessage += "m=" + media.Media + " " + strconv.FormatUint(uint64(media.Port), 10) + " " + strconv.FormatUint(uint64(media.NumberOfPorts), 10) + " " + media.Protocol

		//Put formats
		for _, format := range media.Formats {
			resultMessage += format + " "
		}
		resultMessage = strings.TrimSuffix(resultMessage, " ")
		resultMessage += sdpLineEnd

		//Put optional MediaTitle
		if media.MediaTitle != "" {
			if strings.ContainsAny(media.MediaTitle, "\n\r") {
				return "", errors.New("media - mediaTitle invalid")
			}
			resultMessage += media.MediaTitle + sdpLineEnd
		}

		//Put optional MediaTitle
		if media.MediaTitle != "" {
			if strings.ContainsAny(media.MediaTitle, "\n\r") {
				return "", errors.New("media - mediaTitle invalid")
			}
			resultMessage += media.MediaTitle + sdpLineEnd
		}

		//Put optional ConnectionInformation
		connectionInformationMessage, err := packSDPMessageConnectionInformationLine(media.ConnectionData, "media - ")
		if err != nil {
			return "", err
		}
		if len(connectionInformationMessage) != 0 {
			resultMessage += connectionInformationMessage
		} else {
			if !wroteConnectionData {
				return "", errors.New("media - connectionData empty")
			}
		}

		//Put optional Bandwidth
		for _, bandwidth := range media.BandwidthInformations {
			bandwidthMessage, err := packSDPMessageBandwithInformationLine(bandwidth, "media - ")
			if err != nil {
				return "", nil
			}
			resultMessage += bandwidthMessage
		}

		//Put optional Encryption Key
		encryptionKeyMessage, err := packSDPMessageEncryptionKeyLine(media.EncryptionKey, "media - ")
		if err != nil {
			return "", err
		}
		if len(encryptionKeyMessage) != 0 {
			resultMessage += encryptionKeyMessage
		}

		//Put optional Attributes
		for _, attribute := range media.Attributes {
			attributeMessage, err := packSDPMessageAttributeLine(attribute, "media - ")
			if err != nil {
				return "", err
			}
			resultMessage += attributeMessage
		}
	}
	resultMessage = strings.TrimSuffix(resultMessage, sdpLineEnd)
	return resultMessage, nil
}
