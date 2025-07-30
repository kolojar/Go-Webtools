package webtools

import (
	"encoding/hex"
	"net"
	"strconv"
	"time"
)

// Cleanup timeout in seconds
const CLEANUP_TIMEOUT = 30

type ServerConn interface {
	Send([]byte)
	Close()
}

/*
UDP server connection object
*/
type UDPServerConn struct {
	origin   *UDPServer
	conn     *net.TCPConn
	address  *net.UDPAddr
	lastSeen time.Time
}

/*
Sends data to client
*/
func (udpConn *UDPServerConn) Send(data []byte) {
	udpConn.origin.WriteToClient(udpConn.conn, data)
}

/*
Closes connection to client
*/
func (udpConn *UDPServerConn) Close() {
	err := udpConn.conn.Close()
	if err != nil {
		udpConn.origin.logger.Log(3, "Error closing connection from: "+udpConn.conn.RemoteAddr().String()+" connected locally to: "+tcpConn.conn.LocalAddr().String()+" with error: "+err.Error())
	} else {
		delete(udpConn.origin.conns, udpConn.address)
		udpConn.origin.logger.Log(0, "Closed connectin on "+udpConn.conn.RemoteAddr().String()+" connected locally to: "+tcpConn.conn.LocalAddr().String())
	}
}

/*
Standardized type of function
*UDPReadFunc = Connection
String = message
Bool = is ended
*/
type UDPReadFunc func(*UDPReadFunc, []byte, bool)

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
	return &UDPServer{address: addressObj, readFunc: readFunc, logger: MakeConsoleLogger("UDPServer", level)}, nil
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
	udp.listener, err = net.ListenUDP("tcp", udp.address)
	if err != nil {
		udp.logger.Log(3, "Error listening to "+udp.address.String()+" with error: "+err.Error())
		return
	}
	udp.isRunning = true
	udp.logger.Log(2, "Started listening on "+udp.address.String())

	//Handle read and connection accept
	udp.handleTCPRead()

	//Listener loop
	for !udp.requestedStop {
		conn, err2 := udp.listener.AcceptTCP()
		if err2 != nil {
			if udp.requestedStop {
				//Ignore all errors
				break
			} else {
				udp.logger.Log(3, "Error accepting connection: "+err2.Error())
			}
		}

		//Handle connection
		udp.logger.Log(2, "Connection from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String())

	}
}

/*
Handles UDP Read
*/
func (udp *UDPServer) handleTCPRead() {
	buffer := make([]byte, BUFFER_SIZE)
	udpConn := &UDPServerConn{origin: udp, conn: conn}
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			//Exit on errors
			//if err.Error() != "EOF" && !strings.Contains(err.Error(), "use of closed network connection") {
			logger.Log(3, "Error reading from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String()+" | Error: "+err.Error())
			//}
			break
		}

		//Process read
		if readFunc != nil {
			data := buffer[:n]
			logger.Log(0, "Reading from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
			readFunc(udpConn, data, false)
		}
	}

	//Finished reading
	logger.Log(2, "Disconneted from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String())
	readFunc(udpConn, nil, true)
	defer conn.Close()
}

/*
Handles TCP Write
*/
func writeToTCP(conn *net.TCPConn, data []byte, logger *ConsoleLogger) {
	if conn == nil {
		logger.Log(1, "Invalid connecting, cancelling write.")
		return
	}

	//Write
	logger.Log(0, "Writing to: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
	_, err := conn.Write(data)
	if err != nil {
		logger.Log(3, "Error writing to: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String()+" | Error: "+err.Error())
	}
}

/*
Writes to Client
*/
func (tcp *TCPServer) WriteToClient(conn *net.TCPConn, data []byte) {
	writeToTCP(conn, data, &tcp.logger)
}

/*
Stops TCP server
*/
func (tcp *TCPServer) Stop() {
	if !tcp.isRunning {
		return
	}

	//Request stop
	tcp.requestedStop = true
	err := tcp.listener.Close()
	time.Sleep(1 * time.Second)
	if err != nil {
		tcp.logger.Log(3, "Error stopping TCP server: "+err.Error())
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
	removed := current - oldCount
	udp.logger.Log(0, "Connection cleanup done! Removed connections: "+strconv.Itoa(removed)+" / "+strconv.Itoa(oldCount))
}
