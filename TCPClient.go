package webtools

import (
	"encoding/hex"
	"net"
	"strconv"
)

/*
Standardized type of function
*TCPServerConn = Connection
String = message
Bool = is ended
*/
type TCPClientReadFunc func(*TCPClient, []byte, bool)

/*
Basic TCP Client
*/
type TCPClient struct {
	readFunc   TCPClientReadFunc
	logger     ConsoleLogger
	connection *net.TCPConn
	address    *net.TCPAddr
	isAlive    bool
}

func (tcp *TCPClient) IsAlive() bool {
	return tcp.isAlive
}

/*
Creates new TCP Client but does not starts it
*/
func NewTCPClient(address string, readFunc TCPClientReadFunc, reportTraffic bool) (*TCPClient, error) {
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
	return &TCPClient{address: addressObj, logger: MakeConsoleLogger("TCPClient", level), readFunc: readFunc}, nil
}

/*
Connects to TCP server and start reading loop, does not locks execution thread
*/
func (tcp *TCPClient) Connect() {
	//Dial
	var err error
	tcp.connection, err = net.DialTCP("tcp", nil, tcp.address)
	if err != nil {
		tcp.logger.Log(3, "Error connecting to: "+tcp.address.String()+" | Error: "+err.Error())
		return
	}

	tcp.isAlive = true
	//Handle read
	go func() {
		handleTCPRead(tcp.connection, &tcp.logger, tcp.readFuncLocal)
		tcp.logger.Log(2, "Disconnected from: "+tcp.address.String())
		tcp.isAlive = false
	}()
}

func (tcp *TCPClient) readFuncLocal(conn *net.TCPConn, data []byte, ended bool) {
	//Process read
	if tcp.readFunc != nil {
		if !ended {
			tcp.logger.Log(0, "Reading from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
		}
		tcp.readFunc(tcp, data, ended)
	}
}

/*
Sends data to server
*/
func (tcp *TCPClient) Send(data []byte) {
	writeToTCP(tcp.connection, data, &tcp.logger)
}

/*
Stops TCP client
*/
func (tcp *TCPClient) Stop() {
	if tcp.connection == nil {
		//Invalid connection
		return
	}

	//Close
	tcp.logger.Log(1, "Requested disconnect from: "+tcp.address.String())
	err := tcp.connection.Close()
	if err != nil {
		tcp.logger.Log(3, "Error disconnecting from: "+tcp.address.String()+" | Error: "+err.Error())
	}
}
