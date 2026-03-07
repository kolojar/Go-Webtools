package webrtc

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

var constSDPSessionDescriptionOptionalTypes1 = []string{"i", "u", "e", "p", "c", "b"}
var constSDPSessionDescriptionOptionalTypes2 = []string{"z", "k", "a"}
var constSDPMediaDescriptionOptionalTypes = []string{"i", "c", "b", "k", "a"}

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
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.10
*/
type SDPMessageRepeatTime struct {
	RepeatInterval string
	ActiveDuration string
	Offsets        []string
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
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.12
*/
type SDPMessageEncyptionKey struct {
	Method        string //method
	EncryptionKey string //encryption key
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.14
*/
type SDPMessageMediaDescription struct {
	Media                 string                   //m - media
	Port                  string                   //m - port
	NumberOfPorts         string                   //m - number of ports
	Protocol              string                   //m - proto
	Formats               []string                 //m - fmt
	MediaTitle            string                   //i*
	ConnectionData        SDPMessageConnectionData //c*
	BandwidthInformations []SDPMessageBandwidth    //b*
	EncryptionKey         SDPMessageEncyptionKey   //k*
	Attributes            []SDPMessageAttribute    //a*
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.7
*/
type SDPMessageConnectionData struct {
	NetworkType         string   //nettype
	AddressType         string   //addrtype
	ConnectionAddresses []string //connection-address
	TimeToLive          uint8    //ttl
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.8
*/
type SDPMessageBandwidth struct {
	BandwidthType string //bwtype
	Bandwidth     string //bandwidth
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5.11
*/
type SDPMessageTimeZone struct {
	AdjustmentTime string //adjustment time
	Offset         string //offset
}

type SDPMessageAttribute struct {
	Attribute string //attribute
	Value     string //value
}

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
	connectionData := SDPMessageConnectionData{ConnectionAddresses: make([]string, 0)}

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
	repeatCount := 0

	//Sort IP types
	if connectionData.AddressType == "IP4" {
		//IPv4 - check
		if !ip.Is4() {
			return connectionData, errors.New("invalid connectionData-connectionAddress - wants ipv4 and got ipv6")
		}

		//Check length
		if len(connectionAddressSplit) == 1 {
			fmt.Println("Warning: connectionData-connectionAddress - No TTL specified")
			connectionData.ConnectionAddresses = append(connectionData.ConnectionAddresses, ip.String())
		}
		if len(connectionAddressSplit) >= 2 {
			//Format: IP/TTL
			connectionData.ConnectionAddresses = append(connectionData.ConnectionAddresses, ip.String())

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
	} else if connectionData.AddressType == "IP6" {
		//IPv6 - check
		if !ip.Is6() {
			return connectionData, errors.New("invalid connectionData-connectionAddress - wants ipv6 and got ipv4")
		}

		//Check length
		if len(connectionAddressSplit) == 1 {
			//Format: IP
			connectionData.ConnectionAddresses = append(connectionData.ConnectionAddresses, ip.String())
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
	for i := 1; i < repeatCount; i++ {
		ip = ip.Next()
		connectionData.ConnectionAddresses = append(connectionData.ConnectionAddresses, ip.String())
	}
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
	attributeSplit := strings.SplitN(value, " ", 2)
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
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5
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
		mediaType := SDPMessageMediaDescription{
			Media:         mediaSplit[0],
			Port:          mediaSplit[1],
			NumberOfPorts: mediaSplit[2],
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
				//ConnectionData
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
				mediaType.Attributes = append(sdpMessage.Attributes, attribute)
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
