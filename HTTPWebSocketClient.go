package webtools

import (
	"encoding/base64"
	"strings"
	"time"
)

/*
Standardized type of function
*HTTPWebSocketClient = Client
String = message
Uint8 = 0 = Open, 1 = Close, 2 = Read text, 3 = Read binary
*/
type HTTPWebSocketClientReadFunc func(*HTTPWebSocketClient, []byte, uint8)

type HTTPWebSocketClient struct {
	tcpClient      *TCPClient
	Logger         *ConsoleLogger
	readFunc       HTTPWebSocketClientReadFunc
	awaitingReady  bool
	awaitingStatus bool
	address        string
	hijacked       bool
	pathForHTTP    string
}

func (cl *HTTPWebSocketClient) IsAlive() bool {
	return cl.tcpClient.IsAlive()
}

/*
Creates new HTTP WebSocket Client but does not connects it, if you want to use default connection endpoint, add /websocket to end of address
*/
func NewHTTPWebSocketClient(address string, readFunc HTTPWebSocketClientReadFunc, reportTraffic bool) (*HTTPWebSocketClient, error) {
	level := uint8(0)
	if !reportTraffic {
		level = 1
	}

	//Create client
	cl := &HTTPWebSocketClient{Logger: NewConsoleLogger("HTTP-WSClient", level), readFunc: readFunc, address: address}
	var err error
	var tcpAddress string
	tcpAddress, cl.pathForHTTP = HTTPWebTransportGetAddressAndTarget(address)
	cl.tcpClient, err = NewTCPClient(tcpAddress, cl.readFuncLocal, reportTraffic, false)
	if err != nil {
		return nil, err
	}
	cl.tcpClient.Logger = cl.Logger
	return cl, nil
}

/*
Connects to HTTP server and start reading loop, does not locks execution thread
*/
func (cl *HTTPWebSocketClient) Connect() {
	if cl.tcpClient.IsAlive() {
		return
	}

	cl.tcpClient.Connect()

	//Reset ready state
	cl.tcpClient.Logger.Log(1, "Upgrading connection with: "+cl.tcpClient.address.String())
	cl.awaitingReady = true
	cl.hijacked = false

	//Get host
	host := strings.SplitN(cl.address, ":", 2)[0]

	//Generate random key
	key := base64.StdEncoding.EncodeToString([]byte(GenerateRandomString(24)))

	//Make handshake GET
	request := "GET " + cl.pathForHTTP + " HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"\r\n"
	cl.Send([]byte(request), 0)
	for cl.awaitingReady {
		time.Sleep(1 * time.Second)
	}
	if cl.awaitingStatus {
		//Successfully connected
		cl.hijacked = true
		cl.tcpClient.Logger.Log(1, "Upgraded connection with: "+cl.tcpClient.address.String())
	} else {
		cl.tcpClient.Logger.Log(3, "Failed to upgrade connection with: "+cl.tcpClient.address.String())
		cl.tcpClient.Stop()
	}
}

/*
Sends data to server
*/
func (cl *HTTPWebSocketClient) Send(data []byte, opcode uint8) {
	if cl.hijacked {
		writeToWebSocketFrame(cl.tcpClient.Conn, data, opcode, cl.Logger)
	} else {
		writeToTCP(cl.tcpClient.Conn, data, cl.Logger)
	}
}
