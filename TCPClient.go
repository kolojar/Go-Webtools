package webtools

import (
	"encoding/hex"
	"net"
	"strconv"
)

/*
Standardized type of function
*TCPClient = Client
String = message
Bool = is ended
*/
type TCPClientReadFunc func(*TCPClient, []byte, bool)

/*
Basic TCP Client
*/
type TCPClient struct {
	readFunc TCPClientReadFunc
	Logger   *ConsoleLogger
	Conn     *net.TCPConn
	address  *net.TCPAddr
	isAlive  bool
	framed   bool
}

func (tcp *TCPClient) IsAlive() bool {
	return tcp.isAlive
}

/*
Creates new TCP Client but does not starts it
*/
func NewTCPClient(address string, readFunc TCPClientReadFunc, reportTraffic bool, framed bool) (*TCPClient, error) {
	//Make address
	addressObj, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}

	//Make client
	level := uint8(0)
	if !reportTraffic {
		level = 1
	}
	return &TCPClient{address: addressObj, Logger: NewConsoleLogger("TCPClient", level), readFunc: readFunc, framed: framed}, nil
}

/*
Connects to TCP server and start reading loop, does not locks execution thread
*/
func (tcp *TCPClient) Connect() {
	//Dial
	var err error
	tcp.Conn, err = net.DialTCP("tcp", nil, tcp.address)
	if err != nil {
		tcp.Logger.Log(3, "Error connecting to: "+tcp.address.String()+" | Error: "+err.Error())
		return
	}

	tcp.Logger.Log(2, "Connected to: "+tcp.address.String())
	tcp.isAlive = true
	//Handle read
	go func() {
		if tcp.framed {
			handleTCPReadFramed(tcp.Conn, tcp.Logger, tcp.readFuncLocal)
		} else {
			handleTCPRead(tcp.Conn, tcp.Logger, tcp.readFuncLocal)
		}
		tcp.isAlive = false
	}()
}

func (tcp *TCPClient) readFuncLocal(conn *net.TCPConn, data []byte, ended bool) {
	//Process read
	if tcp.readFunc != nil {
		if !ended {
			tcp.Logger.Log(0, "Reading from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
		}
		tcp.readFunc(tcp, data, ended)
	}
}

/*
Sends data to server
*/
func (tcp *TCPClient) Send(data []byte) {
	if tcp.framed {
		writeToTCPFramed(tcp.Conn, data, tcp.Logger)
	} else {
		writeToTCP(tcp.Conn, data, tcp.Logger)
	}
}

/*
Stops TCP client
*/
func (tcp *TCPClient) Stop() {
	if tcp.Conn == nil || !tcp.isAlive {
		//Invalid connection
		return
	}

	//Close
	tcp.Logger.Log(1, "Requested disconnect from: "+tcp.address.String())
	err := tcp.Conn.Close()
	if err != nil {
		tcp.Logger.Log(3, "Error disconnecting from: "+tcp.address.String()+" | Error: "+err.Error())
	}
}
