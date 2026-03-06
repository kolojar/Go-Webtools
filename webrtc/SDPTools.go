package webrtc

import (
	"errors"
	"os"
	"strings"
)

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

type SDPMessageTimeDescription struct {
	Time        string   //t
	RepeatTimes []string //r*
}

type SDPMessageMediaDescription struct {
	MediaNameAndTransportAddress string   //m
	MediaTitle                   string   //i*
	ConnectionInformation        string   //c*
	BandwidthInformations        []string //b*
	EncryptionKey                string   //k*
	Attributes                   []string //a*
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
	EmailAddress          string                       //e*
	PhoneNumber           string                       //p*
	ConnectionInformation string                       //c*
	BandwidthInformations []string                     //b*
	TimeDescriptions      []SDPMessageTimeDescription  //complex = t + r*
	TimeZoneAdjustments   []string                     //z*
	EncryptionKey         string                       //k*
	Attributes            []string                     //a=*
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

/*
Specification: https://datatracker.ietf.org/doc/html/rfc4566#section-5
*/
func UnpackSDPMessage(message string) (SDPMessage, error) {
	//Split to lines
	messageLines := make([]string, 0)
	for line := range strings.Lines(message) {
		messageLines = append(messageLines, line)
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

	//Read optional types
	lineNumber := 3
	for lineNumber < len(messageLines) {
		//Check for Session Information
		messageLine := messageLines[lineNumber]
		var value string
		value, err = getSDPMessageLineValue(messageLine, "i")
		if err != nil {
			if errors.Is(os.ErrInvalid, err) {
				if value == "t" {
					//Jump to Time Description
					break
				}
				//Skip optional property
			} else {
				//Other error
				return sdpMessage, err
			}
		}

	}
	//return sdpMessage, nil
}
