package webtools

import (
	"strings"
	"time"
)

/*
Standardized type of function
*HTTPWebTransportClient = Client
String = message
Bool = is ended
*/
type HTTPWebTransportClientReadFunc func(*HTTPWebTransportClient, []byte, bool)

/*
Simple HTTP connection hijack client fo switching from HTTP to TCP.
This is NOT WebSocket HTTP client for JavaScript, it is intended for inner communication between Go server and Go client (this file). It is used for HTTPProxy (TCP and UDP traffic over HTTP)
*/
type HTTPWebTransportClient struct {
	tcpClient      *TCPClient
	Logger         *ConsoleLogger
	readFunc       HTTPWebTransportClientReadFunc
	awaitingReady  bool
	awaitingStatus bool
	address        string
}

func (cl *HTTPWebTransportClient) IsAlive() bool {
	return cl.tcpClient.IsAlive()
}

/*
Creates new HTTP WebTransport Client but does not connects it
*/
func NewHTTPWebTransportClient(address string, readFunc HTTPWebTransportClientReadFunc, reportTraffic bool) (*HTTPWebTransportClient, error) {
	level := uint8(0)
	if !reportTraffic {
		level = 1
	}

	//Create client
	cl := &HTTPWebTransportClient{Logger: NewConsoleLogger("HTTP-WTClient", level), readFunc: readFunc, address: address}
	var err error
	cl.tcpClient, err = NewTCPClient(address, cl.readFuncLocal, reportTraffic, false)
	if err != nil {
		return nil, err
	}
	cl.tcpClient.Logger = cl.Logger
	return cl, nil
}

/*
Connects to HTTP server and start reading loop, does not locks execution thread
*/
func (cl *HTTPWebTransportClient) Connect() {
	if cl.tcpClient.IsAlive() {
		return
	}

	cl.tcpClient.Connect()

	//Reset ready state
	cl.tcpClient.Logger.Log(1, "Upgrading connection with: "+cl.tcpClient.address.String())
	cl.awaitingReady = true

	//Get host
	//host, _ := strings.CutSuffix(cl.tcpClient.address.String(), "/websocket")
	host := strings.SplitN(cl.address, ":", 2)[0]

	//Make handshake GET
	request := "GET /webtransport HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"\r\n"
	cl.Send([]byte(request))
	for cl.awaitingReady {
		time.Sleep(1 * time.Second)
	}
	if cl.awaitingStatus {
		//Successfully connected
		cl.tcpClient.Logger.Log(1, "Upgraded connection with: "+cl.tcpClient.address.String())
	} else {
		cl.tcpClient.Logger.Log(3, "Failed to upgrade connection with: "+cl.tcpClient.address.String())
		cl.tcpClient.Stop()
	}
}

/*
Sends data to server
*/
func (cl *HTTPWebTransportClient) Send(data []byte) {
	writeToTCPFramed(cl.tcpClient.Conn, data, cl.Logger)
}

/*
Local readFunc for local TCP client
*/
func (cl *HTTPWebTransportClient) readFuncLocal(_ *TCPClient, data []byte, ended bool) {
	if !ended && cl.awaitingReady {
		//First request
		cl.awaitingStatus = strings.Contains(string(data), "HTTP/1.1 101 Switching Protocols")
		cl.awaitingReady = false
		return
	}

	//Other requests
	if cl.readFunc != nil {
		cl.readFunc(cl, data, ended)
	}
}

/*
Stops TCP client
*/
func (cl *HTTPWebTransportClient) Stop() {
	cl.tcpClient.Stop()
}
