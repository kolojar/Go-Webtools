package webtools

import (
	"encoding/hex"
	"strconv"
)

/*
FrameSeparatorChar is separator character in WebTools frames
*/
const FrameSeparatorChar = byte(rune(';'))

/*
FrameTypeConnect is signal of connect in WebTools frames
*/
const FrameTypeConnect = uint8(1)

/*
FrameTypeClose is signal of close in WebTools frames
*/
const FrameTypeClose = uint8(2)

/*
FrameTypeData is signal of data in WebTools frames
*/
const FrameTypeData = uint8(3)

/*
UnpackedWebtoolsFrame is object of unpacked frame
*/
type UnpackedWebtoolsFrame struct {
	Operation uint8
	ID        []byte
	Data      []byte
}

/*
PackWebtoolsFrame packs webtools frame
*/
func PackWebtoolsFrame(operation uint8, id []byte, data []byte) []byte {
	frame := make([]byte, 0)
	frame = append(frame, operation)
	frame = append(frame, FrameSeparatorChar)
	frame = append(frame, id...)
	frame = append(frame, FrameSeparatorChar)
	frame = append(frame, []byte(strconv.Itoa(len(data)))...)
	frame = append(frame, FrameSeparatorChar)
	if data != nil {
		frame = append(frame, data...)
	}
	return frame
}

/*
UnpackWebtoolsFrame unpacks webtools frame, operation 0 means error
*/
func UnpackWebtoolsFrame(frame []byte, logger *ConsoleLogger) []UnpackedWebtoolsFrame {
	//Invalid frame
	if len(frame) == 0 {
		return nil
	}

	//Check size
	if len(frame) < 2 {
		logger.Log(3, "Frame too short. | Data lenght: "+strconv.Itoa(len(frame))+" | Data in hex: "+hex.EncodeToString(frame))
		return nil
	}

	//Get operation
	operation := frame[0]
	if frame[1] != FrameSeparatorChar {
		logger.Log(3, "Invalid frame at index 1. | Data lenght: "+strconv.Itoa(len(frame))+" | Data in hex: "+hex.EncodeToString(frame))
		return nil
	}

	//Get id and len of rest of frame
	var id []byte
	var idEndIndex = -1
	var data []byte
	var subframes []UnpackedWebtoolsFrame
	for i := 2; i < len(frame); i++ {
		if frame[i] == FrameSeparatorChar {
			if idEndIndex == -1 {
				//Get id
				id = frame[2:i]
				idEndIndex = i
			} else {
				//Get len of data
				lenOfDataStr := frame[idEndIndex+1 : i]
				lenOfData, err := strconv.Atoi(string(lenOfDataStr))
				if lenOfData > 0 {
					lenOfData = lenOfData - 1
				}
				if err != nil {
					logger.Log(3, "Invalid frame lenght. | Data lenght: "+strconv.Itoa(len(frame))+" | Data in hex: "+hex.EncodeToString(frame)+" | Error: "+err.Error())
					return nil
				}

				//Get data
				if len(frame) > (i + lenOfData + 1) {
					data = frame[i+1 : i+2+lenOfData]
				}

				//Get rest of data
				if len(frame) > (i + lenOfData + 1) {
					subframes = UnpackWebtoolsFrame(frame[i+2+lenOfData:], logger)
				}
				break
			}
		}

	}

	//Make result
	result := make([]UnpackedWebtoolsFrame, 0)
	result = append(result, UnpackedWebtoolsFrame{Operation: operation, ID: id, Data: data})
	if subframes != nil {
		result = append(result, subframes...)
	}
	//fmt.Println(len(result))
	return result
}
