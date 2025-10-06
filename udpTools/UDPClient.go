package udpTools

import (
	"encoding/binary"
	"encoding/hex"
	"net"
	"strconv"
	"strings"
	"time"
	"webtools"
)

/*
Standardized type of function
*UDPClient = Client
String = message
Bool = is ended
*/
type UDPClientReadFunc func(client *UDPClient, sourceAddress *net.UDPAddr, data []byte, ended bool)

/*
Basic TCP Client
*/
type UDPClient struct {
	readFunc UDPClientReadFunc
	Logger   *webtools.ConsoleLogger
	Conn     *net.UDPConn
	address  *net.UDPAddr
	isAlive  bool

	//Use TCP like informing and organisation
	isFramed bool
	//Organise packets in order as they were send
	isOrganised bool
	//How long to wait for other packets to arrive to do the sorting
	organisedTimeoutInMs uint
	//How long to wait for resending the packet if no responce arrive
	timeoutForResendInMs uint
	gotResponce          webtools.SafeMap[string, bool]
	readData             webtools.SafeMap[string, time.Time]
	resendMaxLimit       uint
}

func (udp *UDPClient) IsAlive() bool {
	return udp.isAlive
}

/*
Creates new UDP Client but does not starts it
*/
func NewUDPClient(address string, readFunc UDPClientReadFunc, reportTraffic bool) (*UDPClient, error) {
	//Make address
	addressObj, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	//Make client
	return &UDPClient{address: addressObj, Logger: webtools.NewConsoleLoggerForTraffic("UDPClient", reportTraffic), readFunc: readFunc}, nil
}

/*
Connects to TCP server and start reading loop, does not locks execution thread
*/
func (udp *UDPClient) Connect() {
	//Dial
	var err error
	udp.Conn, err = net.DialUDP("udp", nil, udp.address)
	if err != nil {
		udp.Logger.Log(3, "Error connecting to: "+udp.address.String()+" | Error: "+err.Error())
		return
	}
	udp.startRead()
}

func (udp *UDPClient) startRead() {
	udp.isAlive = true
	//Handle read
	go func() {
		var ok bool = true
		for ok {
			ok = handleUDPRead(udp.Conn, udp.Logger, udp.readFuncLocal)
		}
		udp.isAlive = false
		udp.readFuncLocal(nil, nil, true)
	}()
}

func (udp *UDPClient) readFuncLocal(addrFrom *net.UDPAddr, data []byte, ended bool) {
	if ended {
		//If ended
		if udp.readFunc != nil {
			udp.readFunc(udp, addrFrom, data, ended)
		}
	}

	//Sort if framed
	if udp.isFramed {
		//Check size
		if len(data) < 2 {
			udp.Logger.Log(3, "Frame too short. | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
			return
		}
		typeOfFrame := data[0]
		if data[1] != webtools.WEBTOOLS_FRAME_SEPARATOR {
			udp.Logger.Log(3, "Invalid frame at index 1. | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
			return
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
					udp.Logger.Log(3, "Frame too short. | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
					return
				}
				var timeStamp uint64
				if udp.isOrganised {
					timeStamp = binary.BigEndian.Uint64(data[idEndIndex+1 : idEndIndex+9])
					if data[idEndIndex+10] != webtools.WEBTOOLS_FRAME_SEPARATOR {
						udp.Logger.Log(3, "Invalid frame at index "+strconv.Itoa(idEndIndex+10)+". | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
						return
					}
				}
				_ = timeStamp

				//Send ACK
				frame := make([]byte, 0)
				frame = append(frame, byte('1')) //1 for ACK
				frame = append(frame, webtools.WEBTOOLS_FRAME_SEPARATOR)
				frame = append(frame, id...)
				frame = append(frame, webtools.WEBTOOLS_FRAME_SEPARATOR)
				writeToUDP(false, udp.Conn, udp.address, frame, udp.Logger)

				//Process read
				if udp.isOrganised {
					panic("NOT IMPLEMENTED")
				}
				if udp.readFunc != nil && !udp.readData.Has(string(id)) {
					udp.readData.Set(string(id), time.Now())
					udp.readFunc(udp, addrFrom, data[idEndIndex+webtools.FormatByBool(udp.isOrganised, 11, 1):], ended)
				}
				return
			}
		case '1':
			{
				//ACK
				udp.gotResponce.Set(string(id), true)
				return
			}
		default:
			udp.Logger.Log(3, "Dropping frame with invalid frame.")
		}
	}

	//Process read
	//if udp.readFunc != nil {
	//	if !ended {
	//		udp.Logger.Log(0, "Reading from: "+addr.String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
	//	}
	//	udp.readFunc(udp, data, ended)
	//}
}

/*
Sends data to server
*/
func (udp *UDPClient) Send(data []byte) {
	udp.SendSpecificAddress(data, udp.address)
}

/*
Sends data to specific address
*/
func SendSpecificAddress(data []byte, address *net.UDPAddr, isFramed bool, conn *net.UDPConn, logger *webtools.ConsoleLogger) {
	if isFramed {
		sendUDPFrame(webtools.GenerateRandomId(), 1, data)
		return
	}
	writeToUDP(false, conn, address, data, logger)
}

func sendUDPFrame(id string, sequenceNum uint, data []byte, isOrganised bool, conn *net.UDPConn, logger *webtools.ConsoleLogger) {
	//Build frame
	frame := make([]byte, 0)
	frame = append(frame, byte('0')) //0 for data
	frame = append(frame, webtools.WEBTOOLS_FRAME_SEPARATOR)

	//Put ID
	frame = append(frame, []byte(id)...)
	frame = append(frame, webtools.WEBTOOLS_FRAME_SEPARATOR)

	if isOrganised {
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
	writeToUDP(false, conn, udp.address, frame, udp.Logger)
	udp.gotResponce.Set(id, false)
	go udp.checkResponce(id, sequenceNum, data)
}

func (udp *UDPClient) checkResponce(id string, sequenceNum uint, data []byte) {
	time.Sleep(time.Duration(udp.timeoutForResendInMs) * time.Millisecond)
	if !udp.gotResponce.Get(id) {
		//If no responce, resend
		if udp.resendMaxLimit > sequenceNum {
			sendFrame(id, sequenceNum+1, data)
		}
	}
	udp.gotResponce.Delete(id)
}

/*
Stops TCP client
*/
func (udp *UDPClient) Stop() {
	if udp.Conn == nil || !udp.isAlive {
		//Invalid connection
		return
	}

	//Close
	udp.Logger.Log(1, "Requested disconnect from: "+udp.address.String())
	err := udp.Conn.Close()
	if err != nil {
		udp.Logger.Log(3, "Error disconnecting from: "+udp.address.String()+" | Error: "+err.Error())
	}
}

/*
Setups framing for UDP connection and simulates basic TCP properties - mainly checks delivery of all packets and optionly can organise them in orger they got sent
*/
func (udp *UDPClient) SetupFraming(isFramed bool, timeoutForResendInMs uint, resendMaxLimit uint, isOrganised bool, organisedTimeoutInMs uint) {
	udp.isFramed = isFramed
	udp.isOrganised = isOrganised
	udp.timeoutForResendInMs = timeoutForResendInMs
	udp.organisedTimeoutInMs = organisedTimeoutInMs
	udp.resendMaxLimit = resendMaxLimit
}

/*
Handles UDP Read
*/
func handleUDPRead(listener *net.UDPConn, logger *webtools.ConsoleLogger, readFunc func(*net.UDPAddr, []byte, bool)) bool {
	buffer := make([]byte, webtools.BUFFER_SIZE)
	//Get connection and data
	n, addr, err := listener.ReadFromUDP(buffer)
	if err != nil {
		if addr == nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				logger.Log(3, "Error getting UDP connection from: "+err.Error())
			}
		} else {
			logger.Log(3, "Error reading from: "+addr.String()+" | Error: "+err.Error())
		}
		return false
	}

	//Process read
	if readFunc != nil {
		data := buffer[:n]
		readFunc(addr, data, false)
	}
	return true
}

/*
Handles TCP Write
*/
func writeToUDP(isServer bool, listener *net.UDPConn, addr *net.UDPAddr, data []byte, logger *webtools.ConsoleLogger) {
	if addr == nil {
		logger.Log(1, "Invalid connecting, cancelling write.")
		return
	}

	//Write
	logger.Log(0, "Writing to: "+addr.String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
	var err error
	if isServer {
		_, err = listener.WriteToUDP(data, addr)
	} else {
		_, err = listener.Write(data)
	}
	if err != nil {
		logger.Log(3, "Error writing to: "+addr.String()+" | Error: "+err.Error())
	}
}
