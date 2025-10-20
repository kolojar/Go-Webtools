package udpTools

import (
	"encoding/binary"
	"encoding/hex"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"
	"webtools"
)

/*
Config settings for UDP framer
*/
type UDPFramerConfig struct {
	//Organise packets in order as they were send
	IsOrganised bool
	//How long to wait for other packets to arrive to do the sorting
	OrganisedTimeoutInMs int64
	//How long to wait for resending the packet if no responce arrive
	TimeoutForResendInMs int64
	//Retry count
	ResendMaxLimit uint
}

type UDPFramerReadFunc func(address *net.UDPAddr, data []byte, ended bool)

/*
Adds basic checking for resending packages and ordering them same as TCP
*/
type UDPFramer struct {
	config         *UDPFramerConfig
	gotResponce    webtools.SafeMap[string, bool]
	readData       webtools.SafeMap[string, time.Time]
	orderList      []webtools.FourValuePair[string, uint64, *net.UDPAddr, []byte]
	orderListMutex *sync.RWMutex
	onReadFunc     UDPFramerReadFunc
}

/*
Creates new UDP framer
*/
func NewUDPFramer(readFunc UDPFramerReadFunc, timeoutForResendInMs int64, resendMaxLimit uint, isOrganised bool, organisedTimeoutInMs int64) *UDPFramer {
	return &UDPFramer{onReadFunc: readFunc, config: &UDPFramerConfig{TimeoutForResendInMs: timeoutForResendInMs, ResendMaxLimit: resendMaxLimit, IsOrganised: isOrganised, OrganisedTimeoutInMs: organisedTimeoutInMs}, gotResponce: webtools.MakeSafeMap[string, bool](), readData: webtools.MakeSafeMap[string, time.Time](), orderList: make([]webtools.FourValuePair[string, uint64, *net.UDPAddr, []byte], 0), orderListMutex: &sync.RWMutex{}}
}

/*
Creates new UDP framer
*/
func NewUDPFramerSimple(timeoutForResendInMs int64, resendMaxLimit uint, isOrganised bool, organisedTimeoutInMs int64) *UDPFramer {
	return NewUDPFramerSimpleFromConfig(&UDPFramerConfig{TimeoutForResendInMs: timeoutForResendInMs, ResendMaxLimit: resendMaxLimit, IsOrganised: isOrganised, OrganisedTimeoutInMs: organisedTimeoutInMs})
}

/*
Creates new UDP framer
*/
func NewUDPFramerSimpleFromConfig(config *UDPFramerConfig) *UDPFramer {
	return &UDPFramer{onReadFunc: nil, config: config, gotResponce: webtools.MakeSafeMap[string, bool](), readData: webtools.MakeSafeMap[string, time.Time](), orderList: make([]webtools.FourValuePair[string, uint64, *net.UDPAddr, []byte], 0), orderListMutex: &sync.RWMutex{}}
}

/*
Resolves UDP frames
Returns ACK frame
*/
func (framer *UDPFramer) Resolve(address *net.UDPAddr, data []byte, logger *webtools.ConsoleLogger) []byte {
	//Invalid frame
	if len(data) == 0 {
		return nil
	}

	//Check size
	if len(data) < 2 {
		logger.Log(3, "Frame too short. | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
		return nil
	}
	typeOfFrame := data[0]
	if data[1] != webtools.WEBTOOLS_FRAME_SEPARATOR {
		logger.Log(3, "Invalid frame at index 1. | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
		return nil
	}

	//Get id part
	var idEndIndex int = -1
	var id []byte
	for i := 2; i < len(data); i++ {
		if data[i] == webtools.WEBTOOLS_FRAME_SEPARATOR {
			if idEndIndex == -1 {
				//Get id
				id = data[2:i]
				idEndIndex = i
			}
		}
	}

	switch typeOfFrame {
	case '0':
		{
			//Data
			//Get sequence number
			if len(data) == idEndIndex {
				logger.Log(3, "Frame too short. | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
				return nil
			}
			var timeStamp uint64
			if framer.config.IsOrganised {
				timeStamp = binary.BigEndian.Uint64(data[idEndIndex+1 : idEndIndex+9])
				if data[idEndIndex+9] != webtools.WEBTOOLS_FRAME_SEPARATOR {
					logger.Log(3, "Invalid frame at index "+strconv.Itoa(idEndIndex+9)+". | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
					return nil
				}
				logger.Log(0, "Latency: "+strconv.FormatInt(time.Since(time.Unix(0, int64(timeStamp))).Milliseconds(), 10)+" ms")
			}
			logger.Log(0, "Got data frame.")

			//Send ACK
			frame := make([]byte, 0)
			frame = append(frame, byte('1')) //1 for ACK
			frame = append(frame, webtools.WEBTOOLS_FRAME_SEPARATOR)
			frame = append(frame, id...)
			frame = append(frame, webtools.WEBTOOLS_FRAME_SEPARATOR)
			//writeToUDP(udp.IsServer, udp.Conn, addrFrom, frame, udp.Logger)

			//Process read
			if !framer.readData.Has(string(id)) {
				framer.readData.Set(string(id), time.Now())
				if framer.config.IsOrganised {
					go framer.ProcessOrdered(string(id), timeStamp, address, data[idEndIndex+10:])
				} else {
					if framer.onReadFunc != nil {
						framer.onReadFunc(address, data[idEndIndex+1:], false)
					}
				}
			}
			framer.readData.Set(string(id), time.Now())
			framer.CleanupData(logger, false)
			return frame
		}
	case '1':
		{
			//ACK
			logger.Log(0, "Got ACK frame.")
			framer.gotResponce.Set(string(id), true)
			framer.CleanupData(logger, false)
			return nil
		}
	default:
		logger.Log(3, "Dropping frame with invalid frame.")
	}
	return nil
}

/*
Adds item to ordered list and starts timer and resolutes returned messages
*/
func (framer *UDPFramer) ProcessOrdered(id string, timeData uint64, address *net.UDPAddr, data []byte) {
	//Find and store data in list
	pair := webtools.FourValuePair[string, uint64, *net.UDPAddr, []byte]{A: id, B: timeData, C: address, D: data}
	framer.orderListMutex.Lock()
	stored := false
	for i := 0; i < len(framer.orderList); i++ {
		if framer.orderList[i].B > timeData {
			if i == 0 {
				framer.orderList = slices.Insert(framer.orderList, 0, pair)
			} else {
				framer.orderList = slices.Insert(framer.orderList, i-1, pair)
			}
			stored = true
			break
		}
	}
	if !stored {
		framer.orderList = append(framer.orderList, pair)
	}
	framer.orderListMutex.Unlock()

	//Wait timeout
	time.Sleep(time.Millisecond * time.Duration(framer.config.OrganisedTimeoutInMs))

	//Return all order than this
	framer.orderListMutex.Lock()
	var i int = 0
	for i = 0; i < len(framer.orderList); i++ {
		if framer.onReadFunc != nil {
			framer.onReadFunc(framer.orderList[i].C, framer.orderList[i].D, false)
		}
		if framer.orderList[i].A == id {
			break
		}
	}

	//Remove not needed values
	if len(framer.orderList) > i+1 {
		framer.orderList = framer.orderList[i+1:]
	} else {
		framer.orderList = nil
		framer.orderList = make([]webtools.FourValuePair[string, uint64, *net.UDPAddr, []byte], 0)
	}
	framer.orderListMutex.Unlock()
}

/*
After double timeout exports all reading stored in framer
*/
func (framer *UDPFramer) ExportAllOrdered() {
	//Wait timeout
	time.Sleep(time.Millisecond * 2 * time.Duration(framer.config.OrganisedTimeoutInMs))

	//Return all order than this
	framer.orderListMutex.Lock()
	var i int = 0
	for i = 0; i < len(framer.orderList); i++ {
		if framer.onReadFunc != nil {
			framer.onReadFunc(framer.orderList[i].C, framer.orderList[i].D, false)
		}
	}

	//Remove not needed values
	framer.orderList = nil
	framer.orderList = make([]webtools.FourValuePair[string, uint64, *net.UDPAddr, []byte], 0)
	framer.orderListMutex.Unlock()
}

/*
Removes old not used UDP read data
*/
func (framer *UDPFramer) CleanupData(logger *webtools.ConsoleLogger, forceAll bool) {
	oldCount := framer.readData.Len()
	if forceAll {
		//Forced
		framer.readData.Clear()
	} else {
		for _, d := range framer.readData.GetData() {
			k := d.Key
			v := d.Value
			if time.Since(v).Seconds() >= CLEANUP_TIMEOUT {
				//Remove not used connection
				framer.readData.Delete(k)
				continue
			}
		}
	}
	current := framer.readData.Len()
	removed := oldCount - current
	logger.Log(0, "Data cleanup done! Removed data: "+strconv.Itoa(removed)+" / "+strconv.Itoa(oldCount))
}

/*
Processes all data incoming to this function and results are returned in readFunc
*/
func processDataForUDP(address *net.UDPAddr, data []byte, ended bool, readFunc UDPFramerReadFunc, logger *webtools.ConsoleLogger, framer *UDPFramer, isServer bool, listener *net.UDPConn) {
	if framer == nil {
		//No framing
		if readFunc != nil {
			readFunc(address, data, ended)
		}
		return
	} else {
		framer.onReadFunc = readFunc
		if ended {
			//Ended - clear framed data
			if framer.config.IsOrganised {
				framer.ExportAllOrdered()
			}
			if readFunc != nil {
				readFunc(address, data, ended)
			}
		} else {
			//Process framed data
			frame := framer.Resolve(address, data, logger)
			writeToUDP(isServer, listener, address, frame, logger)
		}
	}
}

/*
Sends data frame for UDP frame protocol, blocks execution thread
*/
func (framer *UDPFramer) SendFrame(isServer bool, listener *net.UDPConn, addr *net.UDPAddr, id string, sequenceNum uint, data []byte, logger *webtools.ConsoleLogger) {
	var resend bool = true
	for resend {
		//Build frame
		frame := make([]byte, 0)
		frame = append(frame, byte('0')) //0 for data
		frame = append(frame, webtools.WEBTOOLS_FRAME_SEPARATOR)

		//Put ID
		frame = append(frame, []byte(id)...)
		frame = append(frame, webtools.WEBTOOLS_FRAME_SEPARATOR)

		if framer.config.IsOrganised {
			//Put timestamp
			timeStamp := make([]byte, 8)
			binary.BigEndian.PutUint64(timeStamp, uint64(time.Now().UnixNano()))
			frame = append(frame, timeStamp...)
			frame = append(frame, webtools.WEBTOOLS_FRAME_SEPARATOR)
		}

		//Insert data
		frame = append(frame, data...)

		//Log send
		logger.Log(0, "Sending frame: "+id+" with sequence number: "+strconv.FormatUint(uint64(sequenceNum), 10))

		//Send
		writeToUDP(isServer, listener, addr, frame, logger)
		framer.gotResponce.Set(id, false)

		//Check responce
		time.Sleep(time.Millisecond * time.Duration(framer.config.TimeoutForResendInMs))
		if !framer.gotResponce.Get(id) {
			//If no responce, resend
			if framer.config.ResendMaxLimit <= sequenceNum {
				resend = false
			}
		} else {
			//Got responce
			resend = false
		}
		framer.gotResponce.Delete(id)
		sequenceNum++
	}
}

func processSendForUDP(isServer bool, listener *net.UDPConn, addr *net.UDPAddr, data []byte, logger *webtools.ConsoleLogger, framer *UDPFramer) {
	if framer == nil {
		//No framing
		writeToUDP(isServer, listener, addr, data, logger)
	} else {
		//Framing
		go framer.SendFrame(isServer, listener, addr, webtools.GenerateRandomId(), 1, data, logger)
	}
}
