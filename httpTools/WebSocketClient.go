package httpTools

import (
	"encoding/base64"
	"strings"
	"time"
	"webtools"
	tcptools "webtools/tcp"
)

/*
Returns address for TCP and path to HTTP request
*/
func HTTPWebSocketGetAddressAndTarget(completeURL string) (string, string) {
	//Separate protocol and URL
	splitUrl := strings.SplitN(completeURL, "://", 2)
	protocol := ""
	url := splitUrl[len(splitUrl)-1]
	if len(splitUrl) > 1 {
		protocol = splitUrl[0]
	}

	//Separate Web address and path
	urlSplit := strings.SplitN(url, "/", 2)
	webAddress := urlSplit[0]
	path := "/"
	if len(urlSplit) > 1 {
		path += urlSplit[1]
	}

	//Get port by protocol
	portByProtocol := ""
	switch protocol {
	case "http":
		{
			portByProtocol = "80"
		}
	case "https":
		{
			portByProtocol = "443"
		}
	case "ws":
		{
			portByProtocol = "80"
		}
	case "wss":
		{
			portByProtocol = "443"
		}
	}

	//Check if webAddress has protocol + add port if needed
	tcpAddress := ""
	if len(strings.SplitN(webAddress, ":", 2)) == 1 {
		//No port, add from port protocol
		tcpAddress = webAddress + ":" + portByProtocol
	} else {
		tcpAddress = webAddress
	}

	return tcpAddress, path
}

/*
Standardized type of function
*WebSocketClient = Client
Uint8 = status
Bool = isBinary
*/
type WebSocketClientReadFunc func(client *WebSocketClient, data []byte, status uint8, isBinary bool)

type WebSocketClient struct {
	tcpClient      *tcptools.ClientUniversal
	Logger         *webtools.ConsoleLogger
	readFunc       WebSocketClientReadFunc
	awaitingReady  bool
	awaitingStatus bool
	address        string
	hijacked       bool
	pathForHTTP    string
	webSocketKey   string
}

func (cl *WebSocketClient) IsAlive() bool {
	return cl.tcpClient.IsAlive()
}

/*
Creates new HTTP WebSocket Client but does not connects it, if you want to use default connection endpoint, add /websocket to end of address
*/
func NewWebSocketClient(address string, readFunc WebSocketClientReadFunc, reportTraffic bool) (*WebSocketClient, error) {
	//Create client
	cl := &WebSocketClient{Logger: webtools.NewConsoleLoggerForTraffic("HTTP-WSClient", reportTraffic), readFunc: readFunc, address: address}
	var err error
	var tcpAddress string
	tcpAddress, cl.pathForHTTP = HTTPWebSocketGetAddressAndTarget(address)
	cl.tcpClient, err = tcptools.NewTCPClientUniversal(tcpAddress, reportTraffic)
	cl.tcpClient.Logger = cl.Logger
	cl.tcpClient.HandlerFuncs = append(cl.tcpClient.HandlerFuncs,
		tcptools.ClientUniversalHanderFuncs{
			UseCount:               1,
			ReadHandler:            tcptools.HandleTCPRead,
			ReadFunc:               cl.readFuncLocalRaw,
			WriteHandler:           tcptools.WriteToTCPHandler,
			CanOneWriteAfterSwitch: false,
		})
	cl.tcpClient.HandlerFuncs = append(cl.tcpClient.HandlerFuncs,
		tcptools.ClientUniversalHanderFuncs{
			UseCount:               -1,
			ReadHandler:            HandleWebSocketFrameRead,
			ReadFunc:               cl.readFuncLocalWS,
			WriteHandler:           WriteToWebSocketFrameHandler,
			CanOneWriteAfterSwitch: false,
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
func (cl *WebSocketClient) Connect() {
	if cl.tcpClient.IsAlive() {
		return
	}

	cl.tcpClient.Connect()

	//Reset ready state
	cl.tcpClient.Logger.Log(1, "Upgrading connection with: "+cl.tcpClient.GetAddress().String())
	cl.awaitingReady = true
	cl.hijacked = false

	//Get host
	host := strings.SplitN(cl.address, ":", 2)[0]

	//Generate random key
	cl.webSocketKey = base64.StdEncoding.EncodeToString([]byte(webtools.GenerateRandomString(24)))

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
		cl.tcpClient.Logger.Log(1, "Upgraded connection with: "+cl.tcpClient.GetAddress().String())
	} else {
		cl.tcpClient.Logger.Log(3, "Failed to upgrade connection with: "+cl.tcpClient.GetAddress().String())
		cl.tcpClient.Stop()
	}
}

/*
Sends data to server
*/
func (cl *WebSocketClient) Send(data []byte, opcode uint8) {
	cl.tcpClient.Send(data, map[string]any{"opcode": opcode})
}

/*
Local readFunc for local TCP client
*/
func (cl *WebSocketClient) readFuncLocalRaw(_ *tcptools.ClientUniversal, data []byte, status uint8, otherData map[string]any) {
	if status == webtools.ReadDataStatus && cl.awaitingReady {
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
		if status != webtools.ReadDataStatus {
			return
		}
		//Other requests
		cl.Logger.Log(3, "Invalid read func called for other requests! Ignoring but inform author of this error.")
		if cl.readFunc != nil {
			cl.readFunc(cl, nil, status, false)
		}
	}
}

/*
Local readFunc for local TCP client with WebSocket frame
*/
func (cl *WebSocketClient) readFuncLocalWS(_ *tcptools.ClientUniversal, data []byte, status uint8, otherData map[string]any) {
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
		cl.readFunc(cl, data, status, isBinary)
	}
}

/*
Stops HTTP WebSocket client
*/
func (ws *WebSocketClient) Stop() {
	ws.tcpClient.Stop()
}
