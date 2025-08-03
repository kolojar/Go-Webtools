package webtools

import (
	"encoding/hex"
	"net"
	"strconv"
	"time"
)

// Cleanup timeout in seconds
const CLEANUP_TIMEOUT = 30

/*
UDP server connection object
*/
type UDPServerConn struct {
	origin   *UDPServer
	address  *net.UDPAddr
	lastSeen time.Time
}

/*
Sends data to client
*/
func (udpConn *UDPServerConn) Send(data []byte) {
	udpConn.origin.WriteToClient(udpConn, data)
}

/*
Closes connection to client
*/
func (udpConn *UDPServerConn) Close() {
	delete(udpConn.origin.conns, udpConn.address)
	udpConn.origin.Logger.Log(0, "Closed connection on "+udpConn.address.String())
	if udpConn.origin.readFunc != nil {
		udpConn.origin.readFunc(udpConn, nil, true)
	}
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
	Logger        *ConsoleLogger
	requestedStop bool
	isRunning     bool
	conns         map[*net.UDPAddr]*UDPServerConn
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
	level := uint8(0)
	if !reportTraffic {
		level = 1
	}
	return &UDPServer{address: addressObj, readFunc: readFunc, Logger: NewConsoleLogger("UDPServer", level), conns: map[*net.UDPAddr]*UDPServerConn{}}, nil
}

/*
Starts UDP Server
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
		handleUDPRead(udp.listener, udp.Logger, udp.readFuncLocal)
	}
	udp.isRunning = false
}

/*
Handles UDP Read
*/
func handleUDPRead(listener *net.UDPConn, logger *ConsoleLogger, readFunc func(*net.UDPAddr, []byte, bool)) bool {
	buffer := make([]byte, BUFFER_SIZE)
	//Get connection and data
	n, addr, err := listener.ReadFromUDP(buffer)
	if err != nil {
		if addr == nil {
			logger.Log(3, "Error getting UDP connection from: "+err.Error())
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
Handles UDP Read for server
*/
func (udp *UDPServer) readFuncLocal(addr *net.UDPAddr, data []byte, ended bool) {
	//Get connection association
	var udpConn *UDPServerConn = udp.conns[addr]
	if udpConn == nil {
		//No connection, create new
		udpConn = &UDPServerConn{origin: udp, address: addr, lastSeen: time.Now()}
		udpConn.origin.conns[addr] = udpConn
	}
	udpConn.lastSeen = time.Now()

	//Process read
	if udp.readFunc != nil {
		udp.Logger.Log(0, "Reading from: "+addr.String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
		udp.readFunc(udpConn, data, false)
	}

	//Do cleanup
	udp.CleanupConnections(false)
}

/*
Handles TCP Write
*/
func writeToUDP(isServer bool, listener *net.UDPConn, addr *net.UDPAddr, data []byte, logger *ConsoleLogger) {
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
Writes to Client
*/
func (udp *UDPServer) WriteToClient(conn *UDPServerConn, data []byte) {
	writeToUDP(true, conn.origin.listener, conn.address, data, udp.Logger)
}

/*
Stops UDP server
*/
func (udp *UDPServer) Stop() {
	if !udp.isRunning {
		return
	}

	//Request stop
	udp.requestedStop = true
	err := udp.listener.Close()
	time.Sleep(1 * time.Second)
	if err != nil {
		udp.Logger.Log(3, "Error stopping UDP server: "+err.Error())
	}
}

/*
Removes old not used UDP connections
*/
func (udp *UDPServer) CleanupConnections(forceAll bool) {
	oldCount := len(udp.conns)
	for k, v := range udp.conns {
		if v == nil {
			//Remove non existing connection addresses
			delete(udp.conns, k)
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
	current := len(udp.conns)
	removed := oldCount - current
	udp.Logger.Log(0, "Connection cleanup done! Removed connections: "+strconv.Itoa(removed)+" / "+strconv.Itoa(oldCount))
}
