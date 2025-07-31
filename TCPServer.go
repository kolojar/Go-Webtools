package webtools

import (
	"encoding/hex"
	"net"
	"strconv"
	"time"
)

const BUFFER_SIZE = 2048

/*
Server connection interface
*/
type ServerConn interface {
	Send([]byte)
	Close()
}

/*
TCP server connection object
*/
type TCPServerConn struct {
	origin *TCPServer
	conn   *net.TCPConn
}

/*
Sends data to client
*/
func (tcpConn *TCPServerConn) Send(data []byte) {
	tcpConn.origin.WriteToClient(tcpConn.conn, data)
}

/*
Closes connection to client
*/
func (tcpConn *TCPServerConn) Close() {
	err := tcpConn.conn.Close()
	if err != nil {
		tcpConn.origin.logger.Log(3, "Error closing connection from: "+tcpConn.conn.RemoteAddr().String()+" connected locally to: "+tcpConn.conn.LocalAddr().String()+" with error: "+err.Error())
	} else {
		tcpConn.origin.logger.Log(0, "Closed connectin on "+tcpConn.conn.RemoteAddr().String()+" connected locally to: "+tcpConn.conn.LocalAddr().String())
	}
}

/*
Standardized type of function
*TCPServerConn = Connection
String = message
Bool = is ended
*/
type TCPReadFunc func(*TCPServerConn, []byte, bool)

/*
Basic TCP server
*/
type TCPServer struct {
	listener      *net.TCPListener
	readFunc      TCPReadFunc
	address       *net.TCPAddr
	logger        ConsoleLogger
	requestedStop bool
	isRunning     bool
}

/*
Creates new TCP Server but does not starts it
*/
func NewTCPServer(address string, readFunc TCPReadFunc, reportTraffic bool) (*TCPServer, error) {
	//Make address
	addressObj, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}
	level := uint8(0)
	if !reportTraffic {
		level = 1
	}
	return &TCPServer{address: addressObj, readFunc: readFunc, logger: MakeConsoleLogger("TCPServer", level)}, nil
}

/*
Starts TCP Server
*/
func (tcp *TCPServer) Start() {
	//Check if already running
	if tcp.isRunning {
		return
	}

	//Reset stop request
	tcp.requestedStop = false

	//Open listener
	var err error
	tcp.listener, err = net.ListenTCP("tcp", tcp.address)
	if err != nil {
		tcp.logger.Log(3, "Error listening to "+tcp.address.String()+" with error: "+err.Error())
		return
	}
	tcp.isRunning = true
	tcp.logger.Log(2, "Started listening on "+tcp.address.String())

	//Listener loop
	for !tcp.requestedStop {
		conn, err2 := tcp.listener.AcceptTCP()
		if err2 != nil {
			if tcp.requestedStop {
				//Ignore all errors
				break
			} else {
				tcp.logger.Log(3, "Error accepting connection: "+err2.Error())
			}
		}

		//Handle connection
		tcp.logger.Log(2, "Connection from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String())
		go tcp.handleTCPRead(conn)
	}
	tcp.isRunning = false
}

/*
Handles TCP Read
*/
func (tcp *TCPServer) handleTCPRead(conn *net.TCPConn) {
	buffer := make([]byte, BUFFER_SIZE)
	tcpConn := &TCPServerConn{origin: tcp, conn: conn}
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			//Exit on errors
			//if err.Error() != "EOF" && !strings.Contains(err.Error(), "use of closed network connection") {
			tcp.logger.Log(3, "Error reading from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String()+" | Error: "+err.Error())
			//}
			break
		}

		//Process read
		if tcp.readFunc != nil {
			data := buffer[:n]
			tcp.logger.Log(0, "Reading from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
			tcp.readFunc(tcpConn, data, false)
		}
	}

	//Finished reading
	tcp.logger.Log(2, "Disconneted from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String())
	tcp.readFunc(tcpConn, nil, true)
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
