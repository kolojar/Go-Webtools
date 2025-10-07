package udpTools

import (
	"encoding/hex"
	"net"
	"strconv"
	"strings"
	"time"
	"webtools"
)

// Cleanup timeout in seconds
const CLEANUP_TIMEOUT = 10

/*
UDP server connection object
*/
type UDPServerConn struct {
	origin *UDPServer
	Address  *net.UDPAddr
	lastSeen time.Time
}

/*
Sends data to client
*/
func (udpConn *UDPServerConn) Send(data []byte) {
	udpConn.Client.Send(data)
}

/*
Closes connection to client
*/
func (udpConn *UDPServerConn) Close() {
	udpConn.origin.conns.Delete(udpConn.Client.address.String())
	udpConn.origin.Logger.Log(0, "Closed connection on "+udpConn.Client.address.String())
	//if udpConn.origin.readFunc != nil {
	//	udpConn.origin.readFunc(udpConn, nil, true)
	//}
	udpConn.Client.Stop()
}

/*
Standardized type of function
*UDPServerConn = Connection
String = message
Bool = is ended
*/
type UDPServerReadFunc func(*UDPServerConn, []byte, bool)

/*
Basic UDP server
*/
type UDPServer struct {
	listener      *net.UDPConn
	readFunc      UDPServerReadFunc
	address       *net.UDPAddr
	Logger        *webtools.ConsoleLogger
	requestedStop bool
	isRunning     bool
	conns         webtools.SafeMap[string, *UDPServerConn]
	udpFramer     *UDPFramer
}

func (udp *UDPServer) GetAddress() *net.UDPAddr {
	return udp.address
}

/*
Creates new UDP Server but does not starts it
*/
func NewUDPServer(address string, readFunc UDPServerReadFunc, reportTraffic bool) (*UDPServer, error) {
	//Make address
	addressObj, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	//Make UDP sv
	return &UDPServer{address: addressObj, readFunc: readFunc, Logger: webtools.NewConsoleLoggerForTraffic("UDPServer", reportTraffic), conns: webtools.MakeSafeMap[string, *UDPServerConn]()}, nil
}

/*
Starts UDP Server, locks execution thread
*/
func (udp *UDPServer) Start() {
	//Check if already running
	if udp.isRunning {
		return
	}

	//Reset stop request
	udp.requestedStop = false

	//Open listener
	var err error
	udp.listener, err = net.ListenUDP("udp", udp.address)
	if err != nil {
		udp.Logger.Log(3, "Error listening to "+udp.address.String()+" with error: "+err.Error())
		return
	}
	udp.isRunning = true
	udp.Logger.Log(2, "Started listening on "+udp.address.String())

	//Listener loop
	for !udp.requestedStop {
		//Handle read and connection accept
		//udp.Client.startRead()
		handleUDPRead(udp.listener, udp.Logger, udp.readFuncLocal)
	}
	udp.isRunning = false
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
	data := buffer[:n]
	logger.Log(0, "Reading from: "+addr.String()+" connected locally to: "+listener.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
	if readFunc != nil {
		readFunc(addr, data, false)
	}
	return true
}


/*
Handles UDP Read for server
*/
func (udp *UDPServer) readFuncLocal(addr *net.UDPAddr, data []byte, ended bool) {
	if !ended {
		//Get connection association
		var udpConn *UDPServerConn = udp.conns.Get(addr.String())
		if udpConn == nil {
			//No connection, create new
			udpConn = &UDPServerConn{origin: udp, lastSeen: time.Now()}
			udpConn.origin.conns.Set(addr.String(), udpConn)
		}
		udpConn.lastSeen = time.Now()
		if udp.udpFramer != nil {
			udp.udpFramer.
		}

		//Process read
		if udp.readFunc != nil {
			udp.Logger.Log(0, "Reading from: "+addr.String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
			udp.readFunc(udpConn, data, false)
		}
	}

	//Do cleanup
	udp.CleanupConnections(false)
}

/*
Writes to Client
*/
func (udp *UDPServer) WriteToClient(conn *UDPServerConn, data []byte) {
	//writeToUDP(true, conn.origin.listener, conn.Address, data, udp.Logger)
	udp.Client.Send(data)
}


/*
Handles UDP Write
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


/*
Stops UDP server
*/
func (udp *UDPServer) Stop() {
	if !udp.isRunning {
		return
	}
	//
	////Request stop
	udp.requestedStop = true
	udp.Client.Stop()
	//err := udp.listener.Close()
	time.Sleep(1 * time.Second)
	//if err != nil {
	//	udp.Logger.Log(3, "Error stopping UDP server: "+err.Error())
	//}
}

/*
Removes old not used UDP connections
*/
func (udp *UDPServer) CleanupConnections(forceAll bool) {
	oldCount := udp.conns.Len()
	for _, d := range udp.conns.GetData() {
		k := d.Key
		v := d.Value
		if v == nil {
			//Remove non existing connection addresses
			udp.conns.Delete(k)
			continue
		}
		if forceAll {
			//Forced
			v.Close()
			continue
		}
		if time.Since(v.lastSeen).Seconds() >= CLEANUP_TIMEOUT {
			//Remove not used connection
			v.Close()
			continue
		}
	}
	current := udp.conns.Len()
	removed := oldCount - current
	udp.Logger.Log(0, "Connection cleanup done! Removed connections: "+strconv.Itoa(removed)+" / "+strconv.Itoa(oldCount))
}

/*
Setups framing for UDP connection and simulates basic TCP properties - mainly checks delivery of all packets and optionly can organise them in orger they got sent
*/
func (udp *UDPServer) SetupFraming(isFramed bool, timeoutForResendInMs uint, resendMaxLimit uint, isOrganised bool, organisedTimeoutInMs uint) {
	udp.Client.SetupFraming(isFramed, timeoutForResendInMs, resendMaxLimit, isOrganised, organisedTimeoutInMs)
}
