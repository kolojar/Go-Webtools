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
	udpConn.origin.logger.Log(0, "Closed connectin on "+udpConn.address.String())
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
type UDPReadFunc func(*UDPServerConn, []byte, bool)

/*
Basic UDP server
*/
type UDPServer struct {
	listener      *net.UDPConn
	readFunc      UDPReadFunc
	address       *net.UDPAddr
	logger        ConsoleLogger
	requestedStop bool
	isRunning     bool
	conns         map[*net.UDPAddr]*UDPServerConn
}

/*
Creates new UDP Server but does not starts it
*/
func NewUDPServer(address string, readFunc UDPReadFunc, reportTraffic bool) (*UDPServer, error) {
	//Make address
	addressObj, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	level := uint8(0)
	if !reportTraffic {
		level = 1
	}
	return &UDPServer{address: addressObj, readFunc: readFunc, logger: MakeConsoleLogger("UDPServer", level), conns: map[*net.UDPAddr]*UDPServerConn{}}, nil
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
		udp.logger.Log(3, "Error listening to "+udp.address.String()+" with error: "+err.Error())
		return
	}
	udp.isRunning = true
	udp.logger.Log(2, "Started listening on "+udp.address.String())

	//Handle read and connection accept
	udp.handleTCPRead()
	udp.isRunning = false
}

/*
Handles UDP Read
*/
func (udp *UDPServer) handleTCPRead() {
	buffer := make([]byte, BUFFER_SIZE)
	//Listener loop
	for !udp.requestedStop {
		//Get connection and data
		n, addr, err := udp.listener.ReadFromUDP(buffer)
		if err != nil {
			if udp.requestedStop {
				//Ignore all errors
				break
			} else {
				if addr == nil {
					udp.logger.Log(3, "Error getting UDP connection: from: "+err.Error())
				} else {
					udp.logger.Log(3, "Error reading from: "+addr.String()+" | Error: "+err.Error())
				}
			}
		}

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
			data := buffer[:n]
			udp.logger.Log(0, "Reading from: "+addr.String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
			udp.readFunc(udpConn, data, false)
		}

		//Do cleanup
		udp.CleanupConnections(false)
	}
}

/*
Handles TCP Write
*/
func writeToUDP(conn *UDPServerConn, data []byte, logger *ConsoleLogger) {
	if conn == nil {
		logger.Log(1, "Invalid connecting, cancelling write.")
		return
	}

	//Write
	logger.Log(0, "Writing to: "+conn.address.String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
	_, err := conn.origin.listener.WriteToUDP(data, conn.address)
	if err != nil {
		logger.Log(3, "Error writing to: "+conn.address.String()+" | Error: "+err.Error())
	}
}

/*
Writes to Client
*/
func (udp *UDPServer) WriteToClient(conn *UDPServerConn, data []byte) {
	writeToUDP(conn, data, &udp.logger)
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
		udp.logger.Log(3, "Error stopping UDP server: "+err.Error())
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
	udp.logger.Log(0, "Connection cleanup done! Removed connections: "+strconv.Itoa(removed)+" / "+strconv.Itoa(oldCount))
}
