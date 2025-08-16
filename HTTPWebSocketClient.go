package webtools

import (
	"encoding/base64"
	"math/rand/v2"
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
	tcpClient      *TCPClientUniversal
	Logger         *ConsoleLogger
	readFunc       HTTPWebSocketClientReadFunc
	awaitingReady  bool
	awaitingStatus bool
	address        string
	hijacked       bool
	pathForHTTP    string
	webSocketKey   string
}

func (cl *HTTPWebSocketClient) IsAlive() bool {
	return cl.tcpClient.IsAlive()
}

/*
Generates random string
*/
func GenerateRandomString(lenght int) string {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	result := ""
	for i := 0; i < lenght; i++ {
		result += string(letters[rand.IntN(len(letters))])
	}
	return result
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
	cl.tcpClient, err = NewTCPClientUniversal(tcpAddress, reportTraffic)
	cl.tcpClient.Logger = cl.Logger
	cl.tcpClient.HandlerFuncs = append(cl.tcpClient.HandlerFuncs,
		FiveValuePair[int, TCPClientUniversalReadHandlerFunc, TCPClientUniversalOnReadFunc, TCPClientUniversalOnWriteHandlerFunc, bool]{
			A: 1,
			B: handleTCPRead,
			C: cl.readFuncLocalRaw,
			D: writeToTCPHandler,
			E: false,
		})
	cl.tcpClient.HandlerFuncs = append(cl.tcpClient.HandlerFuncs,
		FiveValuePair[int, TCPClientUniversalReadHandlerFunc, TCPClientUniversalOnReadFunc, TCPClientUniversalOnWriteHandlerFunc, bool]{
			A: -1,
			B: handleWebSocketFrameRead,
			C: cl.readFuncLocalWS,
			D: writeToWebSocketFrameHandler,
			E: false,
		})
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
	cl.webSocketKey = base64.StdEncoding.EncodeToString([]byte(GenerateRandomString(24)))

	//Make handshake GET
	request := "GET " + cl.pathForHTTP + " HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + cl.webSocketKey + "\r\n" +
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
	cl.tcpClient.Send(data, map[string]any{"opcode": opcode})
}

/*
Local readFunc for local TCP client
*/
func (cl *HTTPWebSocketClient) readFuncLocalRaw(_ *TCPClientUniversal, data []byte, status uint8, otherData map[string]any) {
	if status == TCP_READ_DATA_STATUS && cl.awaitingReady {
		//First request
		if !strings.Contains(string(data), "HTTP/1.1 101 Switching Protocols") {
			//Invalid switch
			cl.awaitingStatus = false
			cl.awaitingReady = false
			return
		}

		//Check if handshake key is correct
		wsKey := computeWebSocketKey(cl.webSocketKey)
		cl.awaitingStatus = strings.Contains(string(data), "Sec-Websocket-Accept: "+wsKey)
		cl.awaitingReady = false
		return
	} else {
		//Other requests
		cl.Logger.Log(3, "Invalid read func called for other requests! Ignoring but inform author of this error.")
		if cl.readFunc != nil {
			cl.readFunc(cl, nil, 1)
		}
	}
}

/*
Local readFunc for local TCP client with WebSocket frame
*/
func (cl *HTTPWebSocketClient) readFuncLocalWS(_ *TCPClientUniversal, data []byte, status uint8, otherData map[string]any) {
	//Get opcode
	isBinaryRaw := otherData["isBinary"]
	if isBinaryRaw == nil || isBinaryRaw == "" {
		//Invalid opcode
		cl.Logger.Log(3, "No property 'opcode' found in otherData")
		return
	}
	isBinary := isBinaryRaw.(bool)

	//Other request
	if cl.readFunc != nil {
		cl.readFunc(cl, data, FormatByBool[uint8](isBinary, 3, 2))
	}
}
