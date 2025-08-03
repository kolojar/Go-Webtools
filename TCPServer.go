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
	Conn   *net.TCPConn
}

/*
Sends data to client
*/
func (tcpConn *TCPServerConn) Send(data []byte) {
	tcpConn.origin.WriteToClient(tcpConn.Conn, data)
}

/*
Closes connection to client
*/
func (tcpConn *TCPServerConn) Close() {
	err := tcpConn.Conn.Close()
	if err != nil {
		tcpConn.origin.Logger.Log(3, "Error closing connection from: "+tcpConn.Conn.RemoteAddr().String()+" connected locally to: "+tcpConn.Conn.LocalAddr().String()+" with error: "+err.Error())
	} else {
		tcpConn.origin.Logger.Log(0, "Closed connectin on "+tcpConn.Conn.RemoteAddr().String()+" connected locally to: "+tcpConn.Conn.LocalAddr().String())
	}
}

/*
Standardized type of function
*TCPServerConn = Connection
String = message
Bool = is ended
*/
type TCPServerReadFunc func(*TCPServerConn, []byte, bool)

/*
Basic TCP server
*/
type TCPServer struct {
	listener      *net.TCPListener
	readFunc      TCPServerReadFunc
	address       *net.TCPAddr
	Logger        *ConsoleLogger
	requestedStop bool
	isRunning     bool
	conns         map[*net.TCPConn]*TCPServerConn
}

/*
Creates new TCP Server but does not starts it
*/
func NewTCPServer(address string, readFunc TCPServerReadFunc, reportTraffic bool) (*TCPServer, error) {
	//Make address
	addressObj, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}
	level := uint8(0)
	if !reportTraffic {
		level = 1
	}
	return &TCPServer{address: addressObj, readFunc: readFunc, Logger: NewConsoleLogger("TCPServer", level), conns: map[*net.TCPConn]*TCPServerConn{}}, nil
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
		tcp.Logger.Log(3, "Error listening to "+tcp.address.String()+" with error: "+err.Error())
		return
	}
	tcp.isRunning = true
	tcp.Logger.Log(2, "Started listening on "+tcp.address.String())

	//Listener loop
	for !tcp.requestedStop {
		conn, err2 := tcp.listener.AcceptTCP()
		if err2 != nil {
			if tcp.requestedStop {
				//Ignore all errors
				break
			} else {
				tcp.Logger.Log(3, "Error accepting connection: "+err2.Error())
			}
		}

		//Handle connection
		tcp.Logger.Log(2, "Connection from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String())
		go handleTCPRead(conn, tcp.Logger, tcp.readFuncLocal)
	}
	tcp.isRunning = false
}

/*
Handles TCP Read
*/
func handleTCPRead(conn *net.TCPConn, logger *ConsoleLogger, readFunc func(*net.TCPConn, []byte, bool)) {
	buffer := make([]byte, BUFFER_SIZE)
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
			readFunc(conn, data, false)
		}
	}

	//Finished reading
	logger.Log(2, "Disconneted from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String())
	readFunc(conn, nil, true)
	defer conn.Close()
}

func (tcp *TCPServer) readFuncLocal(conn *net.TCPConn, data []byte, ended bool) {
	var tcpConn *TCPServerConn = tcp.conns[conn]
	if tcpConn == nil {
		tcpConn = &TCPServerConn{origin: tcp, Conn: conn}
		tcp.conns[conn] = tcpConn
	}
	//Process read
	if tcp.readFunc != nil {
		if !ended {
			tcp.Logger.Log(0, "Reading from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
		}
		tcp.readFunc(tcpConn, data, ended)
	}
}

/*
Handles TCP Write
*/
func writeToTCP(conn *net.TCPConn, data []byte, logger *ConsoleLogger) {
	if conn == nil {
		logger.Log(1, "Invalid connection, cancelling write.")
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
	writeToTCP(conn, data, tcp.Logger)
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
		tcp.Logger.Log(3, "Error stopping TCP server: "+err.Error())
	}
}
