package udp

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

const framer_keepAliveTimerInSeconds = 15

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
	UseKeepAlive   bool
}

type UDPFramerReadFunc func(address *net.UDPAddr, data []byte, ended bool)
/*
FramerReadFunc is function definition for reading data from Framer
*/
type FramerReadFunc func(address *net.UDPAddr, data []byte, ended bool)

/*
Framer adds basic checking for resending packages and ordering them same as TCP
*/
type UDPFramer struct {
	config             *UDPFramerConfig
	gotResponce        webtools.SafeMap[string, bool]
	readData           webtools.SafeMap[string, time.Time]
	orderList          []webtools.FourValuePair[string, uint64, *net.UDPAddr, []byte]
	orderListMutex     *sync.RWMutex
	onReadFunc         UDPFramerReadFunc
	onFailSendFunc     UDPFramerReadFunc
	isKeepAliveRunning bool
	keepAliveConns     webtools.SafeMap[*net.UDPAddr, *net.UDPConn]
}

/*
NewUDPFramer creates new UDP framer
*/
func NewUDPFramer(readFunc UDPFramerReadFunc, failSendFunc UDPFramerReadFunc, timeoutForResendInMs int64, resendMaxLimit uint, isOrganised bool, organisedTimeoutInMs int64, useKeepAlive bool) *UDPFramer {
	framer := NewUDPFramerSimpleFromConfig(&UDPFramerConfig{TimeoutForResendInMs: timeoutForResendInMs, ResendMaxLimit: resendMaxLimit, IsOrganised: isOrganised, OrganisedTimeoutInMs: organisedTimeoutInMs, UseKeepAlive: useKeepAlive}, failSendFunc)
	framer.onReadFunc = readFunc
	return framer
}

/*
NewUDPFramerSimple creates new UDP framer
*/
func NewUDPFramerSimple(failSendFunc UDPFramerReadFunc, timeoutForResendInMs int64, resendMaxLimit uint, isOrganised bool, organisedTimeoutInMs int64, useKeepAlive bool) *UDPFramer {
	framer := NewUDPFramerSimpleFromConfig(&UDPFramerConfig{TimeoutForResendInMs: timeoutForResendInMs, ResendMaxLimit: resendMaxLimit, IsOrganised: isOrganised, OrganisedTimeoutInMs: organisedTimeoutInMs, UseKeepAlive: useKeepAlive}, failSendFunc)
	framer.onFailSendFunc = failSendFunc
	return framer
}

/*
Creates new UDP framer
*/
func NewUDPFramerSimpleFromConfig(config *UDPFramerConfig, failSendFunc UDPFramerReadFunc) *UDPFramer {
	return &UDPFramer{
		onReadFunc:         nil,
		onFailSendFunc:     failSendFunc,
		config:             config,
		gotResponce:        webtools.MakeSafeMap[string, bool](),
		readData:           webtools.MakeSafeMap[string, time.Time](),
		orderList:          make([]webtools.FourValuePair[string, uint64, *net.UDPAddr, []byte], 0),
		orderListMutex:     &sync.RWMutex{},
		keepAliveConns:     webtools.MakeSafeMap[*net.UDPAddr, *net.UDPConn](),
		isKeepAliveRunning: config.UseKeepAlive,
	}
}
func NewUDPFramerSimple(timeoutForResendInMs int64, resendMaxLimit uint, isOrganised bool, organisedTimeoutInMs int64) *Framer {
	return &Framer{onReadFunc: nil, timeoutForResendInMs: timeoutForResendInMs, resendMaxLimit: resendMaxLimit, isOrganised: isOrganised, organisedTimeoutInMs: organisedTimeoutInMs, gotResponce: webtools.MakeSafeMap[string, bool](), readData: webtools.MakeSafeMap[string, time.Time](), orderList: make([]webtools.FourValuePair[string, uint64, *net.UDPAddr, []byte], 0), orderListMutex: &sync.RWMutex{}}
}

/*
Resolve resolves UDP frames
Returns ACK frame
*/
func (framer *Framer) Resolve(address *net.UDPAddr, data []byte, logger *webtools.ConsoleLogger) []byte {
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
	if data[1] != webtools.FrameSeparatorChar {
		logger.Log(3, "Invalid frame at index 1. | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
		return nil
	}

	//Get id part
	var idEndIndex = -1
	var id []byte
	for i := 2; i < len(data); i++ {
		if data[i] == webtools.FrameSeparatorChar {
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
				if data[idEndIndex+9] != webtools.FrameSeparatorChar {
					logger.Log(3, "Invalid frame at index "+strconv.Itoa(idEndIndex+9)+". | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
					return nil
				}
				logger.Log(0, "Latency: "+strconv.FormatInt(time.Since(time.Unix(0, int64(timeStamp))).Milliseconds(), 10)+" ms")
			}
			logger.Log(0, "Got data frame.")

			//Send ACK
			frame := make([]byte, 0)
			frame = append(frame, byte('1')) //1 for ACK
			frame = append(frame, webtools.FrameSeparatorChar)
			frame = append(frame, id...)
			frame = append(frame, webtools.FrameSeparatorChar)
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
	case '2':
		{
			//Keep alive
			logger.Log(0, "Got keep alive frame.")
			return nil
		}
	default:
		logger.Log(3, "Dropping frame with invalid frame.")
	}
	return nil
}

/*
ProcessOrdered adds item to ordered list and starts timer and resolutes returned messages, recommended to run in go routine
*/
func (framer *Framer) ProcessOrdered(id string, timeData uint64, address *net.UDPAddr, data []byte) {
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
	var i = 0
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
ExportAllOrdered after double timeout exports all reading stored in framer
*/
func (framer *Framer) ExportAllOrdered() {
	//Wait timeout
	time.Sleep(time.Millisecond * 2 * time.Duration(framer.config.OrganisedTimeoutInMs))

	//Return all order than this
	framer.orderListMutex.Lock()
	var i = 0
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
CleanupData removes old not used UDP read data
*/
func (framer *Framer) CleanupData(logger *webtools.ConsoleLogger, forceAll bool) {
	oldCount := framer.readData.Len()
	if forceAll {
		//Forced
		framer.readData.Clear()
	} else {
		for _, d := range framer.readData.GetData() {
			k := d.Key
			v := d.Value
			if time.Since(v).Seconds() >= cleanupTimeout {
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
func processDataForUDP(address *net.UDPAddr, data []byte, ended bool, readFunc FramerReadFunc, logger *webtools.ConsoleLogger, framer *Framer, isServer bool, listener *net.UDPConn) {
	if framer == nil {
		//No framing
		if readFunc != nil {
			readFunc(address, data, ended)
		}
		return
	}
		//Framed
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

/*
SendFrame sends data frame for UDP frame protocol, blocks execution thread
*/
func (framer *UDPFramer) SendFrame(isServer bool, listener *net.UDPConn, addr *net.UDPAddr, id string, sequenceNum uint, data []byte, logger *webtools.ConsoleLogger) {
	framer.AddListenerToKeepAlive(isServer, listener, addr, logger)
	var resend bool = true
	for resend {
		//Build frame
		frame := make([]byte, 0)
		frame = append(frame, byte('0')) //0 for data
		frame = append(frame, webtools.FrameSeparatorChar)

		//Put ID
		frame = append(frame, []byte(id)...)
		frame = append(frame, webtools.FrameSeparatorChar)

		if framer.config.IsOrganised {
			//Put timestamp
			timeStamp := make([]byte, 8)
			binary.BigEndian.PutUint64(timeStamp, uint64(time.Now().UnixNano()))
			frame = append(frame, timeStamp...)
			frame = append(frame, webtools.FrameSeparatorChar)
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
				//Failed sending
				if framer.onFailSendFunc != nil {
					framer.onFailSendFunc(addr, data, false)
				}
			}
		} else {
			//Got responce
			resend = false
		}
		framer.gotResponce.Delete(id)
		sequenceNum++
	}
}

func processSendForUDP(isServer bool, listener *net.UDPConn, addr *net.UDPAddr, data []byte, logger *webtools.ConsoleLogger, framer *Framer) {
	if framer == nil {
		//No framing
		writeToUDP(isServer, listener, addr, data, logger)
	} else {
		//Framing
		go framer.SendFrame(isServer, listener, addr, webtools.GenerateRandomId(), 1, data, logger)

	}
}

/*
AddListenerToKeepAlive adds listener to keep alive, listeners used in framer are added automatically on first use
*/
func (framer *UDPFramer) AddListenerToKeepAlive(isServer bool, listener *net.UDPConn, addr *net.UDPAddr, logger *webtools.ConsoleLogger) {
	if framer.keepAliveConns.Has(addr) {
		return
	}

	//Add listener
	framer.keepAliveConns.Set(addr, listener)
	logger.Log(1, "KeepAlive for: "+addr.String()+" added.")
	go func() {
		//Start time loop
		lastUpdate := time.Unix(0, 0)
		for framer.isKeepAliveRunning {
			for time.Since(lastUpdate).Seconds() < framer_keepAliveTimerInSeconds {
				time.Sleep(time.Second)
			}

			//Send frame to keep alive - build frame
			frame := make([]byte, 0)
			frame = append(frame, byte('2')) //2 for keepAlive
			frame = append(frame, webtools.WEBTOOLS_FRAME_SEPARATOR)

			//Put ID
			frame = append(frame, byte('0'))
			frame = append(frame, webtools.WEBTOOLS_FRAME_SEPARATOR)

			//Send frame
			logger.Log(0, "Sending keepAlive frame to: "+addr.String())
			writeToUDP(isServer, listener, addr, frame, logger)
			lastUpdate = time.Now()
		}
		logger.Log(1, "KeepAlive for: "+addr.String()+" ended.")
	}()
}

/*
StopKeepAlive requests stop of keepAlive
*/
func (framer *UDPFramer) StopKeepAlive() {
	framer.isKeepAliveRunning = false
}
