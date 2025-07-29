package webtools

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"time"
)

const webSocketGuid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

/*
Standardized type of function
HTTPServerWebSocketConnection = source
String = message
Bool = is ended
*/
type WebSocketReadFunc func(*HTTPServerWebSocketConnection, string, bool)

/*
Standardized type of function
HTTPServerWebSocketConnection = source
*/
type WebSocketConnectFunc func(*HTTPServerWebSocketConnection)

/*
Basic Websocket HTTP server with simple framing with handshake
*/
type WebSocketHTTPServer struct {
	HttpServer              HTTPServer
	httpGetViewsFunc        funcViews
	webSocketReadFunc       WebSocketReadFunc
	webSocketConnectFunc    WebSocketConnectFunc
	Logger                  ConsoleLogger
	webSocketConnections    []*HTTPServerWebSocketConnection
	webSocketDisconnectFunc WebSocketConnectFunc
}

/*
Gets address of HTTP server (AKA Websocket server)
*/
func (wsSv *WebSocketHTTPServer) GetAddress() string {
	return wsSv.HttpServer.GetAddress()
}

/*
Returns if running HTTP server is running (AKA WebSocket server)
*/
func (ws *WebSocketHTTPServer) IsAlive() bool {
	return ws.HttpServer.IsAlive()
}

/*
Constructs new instance of Websocket HTTP Server but does not starts it
*/
func NewWebSocketHTTPServer(address string, dataPathPrefix string, sharedDataPathPrefix string, httpGetViewsFunc funcViews, httpPostViewsFunc funcViews, webSocketReadFunc WebSocketReadFunc, webSocketConnectFunc WebSocketConnectFunc, webSocketDisconnectFunc WebSocketConnectFunc, startWebBrowser bool) *WebSocketHTTPServer {
	ws := &WebSocketHTTPServer{httpGetViewsFunc: httpGetViewsFunc, webSocketReadFunc: webSocketReadFunc, Logger: MakeConsoleLogger("WebSocketServer"), webSocketConnections: make([]*HTTPServerWebSocketConnection, 0), webSocketConnectFunc: webSocketConnectFunc, webSocketDisconnectFunc: webSocketDisconnectFunc}
	ws.HttpServer = MakeHTTPServer(address, dataPathPrefix, sharedDataPathPrefix, ws.getViewsFunc, httpPostViewsFunc, startWebBrowser)
	ws.HttpServer.Logger = ws.Logger
	return ws
}

/*
Basic Get request handler with Websocket sorting
*/
func (ws *WebSocketHTTPServer) getViewsFunc(w http.ResponseWriter, r *http.Request, params map[string]string) {
	if r.URL.Path == "/websocket" {
		//Setup connection
		conn, cookies, err := prepareWebSocketConnection(w, r, &ws.Logger)
		if err != nil {
			ws.Logger.Log(3, err.Error())
		}

		//Generate new ID
		generateNewId := true
		var id uint64
		for generateNewId {
			id = rand.Uint64()
			generateNewId = false
			for i := 0; i < len(ws.webSocketConnections); i++ {
				if ws.webSocketConnections[i].id == id {
					generateNewId = true
					break
				}
			}
		}

		//Generate new WebSocket object
		webSocket := &HTTPServerWebSocketConnection{id: id, userParameters: params, server: ws, connection: conn, exit: false, exited: false, Cookies: cookies}
		ws.webSocketConnections = append(ws.webSocketConnections, webSocket)
		if ws.webSocketConnectFunc != nil {
			ws.webSocketConnectFunc(webSocket)
		}

		//Start read
		if ws != nil {
			HandleWebSocketRead(conn, &ws.Logger, webSocket.onRead)
			if ws.webSocketDisconnectFunc != nil {
				ws.webSocketDisconnectFunc(webSocket)
			}
		}
	} else {
		if ws.httpGetViewsFunc != nil {
			ws.httpGetViewsFunc(w, r, params)
		}
	}
}

/*
Prepares secured WebSocket connection to meet standards
*/
func prepareWebSocketConnection(w http.ResponseWriter, r *http.Request, logger *ConsoleLogger) (net.Conn, []*http.Cookie, error) {
	logger.Log(1, "Preparing connection from: "+r.RemoteAddr)

	//Verify if connection wants WebSocket
	if !strings.Contains(r.Header.Get("Upgrade"), "websocket") || !strings.Contains(r.Header.Get("Connection"), "Upgrade") {
		http.Error(w, "Invalid WebSocket request", http.StatusBadRequest)
		return nil, nil, errors.New("invalid WebSocket request")
	}

	//Get Websocket key and check it the key is present
	webSocketKey := r.Header.Get("Sec-WebSocket-Key")
	if webSocketKey == "" {
		http.Error(w, "Missing WebSocket Key", http.StatusBadRequest)
		return nil, nil, errors.New("missing WebSocket Key")
	}

	//Compute security WebSocket key
	secureKey := computeWebSocketKey(webSocketKey)

	//Set responce headers
	w.Header().Set("Upgrade", "websocket")
	w.Header().Set("Connection", "Upgrade")
	w.Header().Set("Sec-WebSocket-Accept", secureKey)

	//Request to switch to Websocket keep-alive connection
	w.WriteHeader(http.StatusSwitchingProtocols)

	//Get connection from ResponceWriter using Hijack
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return nil, nil, errors.New("Failed to hijack connection: " + err.Error())
	}

	//Return connection
	logger.Log(1, "WebSocketServer: Prepared connection from: "+r.RemoteAddr)
	return conn, r.Cookies(), nil
}

/*
Creates WebSocket key from original security key and magic text
*/
func computeWebSocketKey(webSocketKey string) string {
	//New hasher
	sha1Hasher := sha1.New()
	//Hash SHA combinet original key and magic text
	sha1Hasher.Write([]byte(webSocketKey + webSocketGuid))
	//Make sum of SHA key
	acceptValue := sha1Hasher.Sum(nil)
	//Encode byte to string
	return base64.StdEncoding.EncodeToString(acceptValue)
}

/*
Handles WebSocket message
Returns if message was ping and message
*/
func HandleWebSocketRead(conn net.Conn, logger *ConsoleLogger, readFunc TCPReadFunc) {
	logger.Log(1, "Connection from: "+(conn).RemoteAddr().String())
	//Read data
	for {
		//Unpack frame
		msg, err := UnpackWebSocketFrame(conn, logger)
		if err != nil {
			if err.Error() != "EOF" && !strings.Contains(err.Error(), "use of closed network connection") {
				logger.Log(3, "Error reading from: "+conn.RemoteAddr().String()+" | Error: "+err.Error())
			}
			break
		}
		logger.Log(1, "Reading from: "+conn.RemoteAddr().String()+" | Data: "+msg)
		if readFunc != nil {
			readFunc(conn, msg, false)
		}
	}

	//Report error
	logger.Log(1, "Disconecting from: "+conn.RemoteAddr().String())
	if readFunc != nil {
		readFunc(conn, "", true)
	}
	defer conn.Close()
}

/*
Return structure: message, error
Unpacks websocket frame
Sources: https://developer.mozilla.org/en-US/docs/Web/API/WebSockets_API/Writing_WebSocket_server
Sources: https://en.wikipedia.org/wiki/WebSocket#Opcodes
Some fixes applied from ChatGPT (big payloads)
*/
func UnpackWebSocketFrame(conn net.Conn, logger *ConsoleLogger) (string, error) {
	//Read header of frame
	header := make([]byte, 2)
	_, err := conn.Read(header)
	if err != nil {
		return "", err
	}

	//Get opcode (operation code) (masked with bitshift operation AND)
	opcode := header[0] & 0xF

	//Sort opcodes
	if opcode == 8 {
		//Close / disconnected
		return "", io.EOF
	}

	//Check if has mask by bitshifting -> returns 0 or 1
	hasMask := header[1] >> 7

	//Size of payload (masked with bitshift operation AND)
	payloadSize := int(header[1] & 0x7F)

	//Can be threaded as offsets
	switch payloadSize {
	case 126:
		//Payload to long -> Longer than 125 characters
		//Get new size of buffer encoded using big Endian - 16 bits
		sizeAdder := make([]byte, 2)
		_, err = conn.Read(sizeAdder)
		if err != nil {
			return "", err
		}

		//Calculate new size
		payloadSize = int(binary.BigEndian.Uint16(sizeAdder))
	case 127:
		//Payload to long -> Longer than 65535 characters
		//Get new size of buffer encoded using big Endian - 64 bits
		sizeAdder := make([]byte, 8)
		_, err = conn.Read(sizeAdder)
		if err != nil {
			return "", err
		}

		//Calculate new size
		payloadSize = int(binary.BigEndian.Uint64(sizeAdder))
	}

	//Read masking key of frame
	maskingKey := make([]byte, 4)
	if hasMask == 1 {
		_, err = conn.Read(maskingKey)
		if err != nil {
			return "", err
		}
	}

	//Payload data (message)
	payload := make([]byte, payloadSize)
	_, err = conn.Read(payload)
	if err != nil {
		return "", err
	}

	//Unmask payload
	if hasMask == 1 {
		for i := 0; i < payloadSize; i++ {
			//Decode payload using masking key and bitshift XOR
			payload[i] = payload[i] ^ maskingKey[i%4]
		}
	}

	//Sort opcodes
	if opcode == 9 {
		//Ping -> Send pong
		logger.Log(1, "Got ping -> IGNORING.")
		SendWebSocketPong(&conn, []byte(string(payload)), logger)
		return "", errors.New("ignoring requested ping")
	}
	if opcode == 10 {
		//Got pong
		logger.Log(1, "Got pong -> IGNORING.")
		return "", errors.New("ignoring requested ping")
	}

	//Normal message
	return string(payload), nil
}

/*
Pack websocket pong frame
*/
func SendWebSocketPong(conn *net.Conn, data []byte, logger *ConsoleLogger) {
	//Create pong (10) frame and copy payload
	frame := []byte{byte(128 + 10)}
	frame = append(frame, byte(len(data)))
	frame = append(frame, data...)

	//Send frame
	_, err := (*conn).Write(frame)
	logger.Log(1, "Sending pong to: "+(*conn).RemoteAddr().String())
	if err != nil {
		logger.Log(3, "Error sending pong to: "+(*conn).RemoteAddr().String()+" | Error: "+err.Error())
	}
}

/*
Pack websocket frame
Sources: https://developer.mozilla.org/en-US/docs/Web/API/WebSockets_API/Writing_WebSocket_server
Sources: https://en.wikipedia.org/wiki/WebSocket#Opcodes
Some fixes applied from ChatGPT (big payloads)
OpCode must be in range form 0 to 16 (from Wikipedia) in hex format
*/
func PackWebSocketFrame(message string, opcode uint8) []byte {
	//Create frame and payload (message array) -> Example for messages: 0x81
	frame := []byte{byte(128 + opcode)}

	//frame := []byte{0x81}
	payload := []byte(message)

	//Calculate payload size
	payloadSize := len(payload)
	if payloadSize <= 125 {
		//Small frame, no problem
		frame = append(frame, byte(payloadSize))
	} else if payloadSize <= 65535 {
		//Payload smaller than 65535 characters - Must convert to support 16 bit endian -> Bitshift 2 bytes
		frame = append(frame, 126) //126 = type for this size of message
		frame = append(frame, byte(payloadSize>>8), byte(payloadSize))
	} else {
		//Payload larger than 65535 characters - Must convert to support 64 bit endian -> Bitshift 8 bytes
		frame = append(frame, 127) //127 = type for this size of message
		frame = append(frame, byte(payloadSize>>56), byte(payloadSize>>48), byte(payloadSize>>40), byte(payloadSize>>32), byte(payloadSize>>24), byte(payloadSize>>16), byte(payloadSize>>8), byte(payloadSize))
	}

	//Append payload at the end
	frame = append(frame, payload...)
	return frame
}

/*
Starts WebSocket under HTTP server
*/
func (ws *WebSocketHTTPServer) Start() {
	ws.Logger.Log(2, "Started WebSocket server at "+ws.HttpServer.address)
	ws.HttpServer.Start()
}

func (ws *WebSocketHTTPServer) Stop() {
	ws.HttpServer.Stop()
}

/*
WebSocket Connection object holder
*/
type HTTPServerWebSocketConnection struct {
	connection     net.Conn
	userParameters map[string]string
	server         *WebSocketHTTPServer
	id             uint64
	exit           bool
	exited         bool
	Cookies        []*http.Cookie
	//gotPing        bool
	//pingMessage    string
}

/*
Writes to WebSocket
*/
func (ws *HTTPServerWebSocketConnection) SendMessage(message string) {
	frame := PackWebSocketFrame(message, 1)
	ws.server.Logger.Log(1, "Sending to: "+ws.connection.RemoteAddr().String()+" | Data: "+message)
	_, err := ws.connection.Write(frame)
	if err != nil {
		ws.server.Logger.Log(3, "Error sending to: "+ws.connection.RemoteAddr().String()+" | Error: "+err.Error())
	}
}

/*
Function for reading (do not use)
*/
func (ws *HTTPServerWebSocketConnection) onRead(_ net.Conn, msg string, hasEnded bool) {
	if ws.server.webSocketReadFunc != nil {
		ws.server.webSocketReadFunc(ws, msg, hasEnded)
	}
	if hasEnded {
		ws.exited = true
		ws.Close()
	}
}

/*
Closes WebSocket connection
*/
func (ws *HTTPServerWebSocketConnection) Close() {
	//Close
	if ws.connection != nil {
		ws.connection.Close()
	}

	//Wait for exit
	for !ws.exited {
		time.Sleep(100 * time.Millisecond)
	}

	//Remove from list
	resultList := make([]*HTTPServerWebSocketConnection, 0)
	for i := 0; i < len(ws.server.webSocketConnections); i++ {
		if ws.id != ws.server.webSocketConnections[i].id {
			resultList = append(resultList, ws.server.webSocketConnections[i])
		}
	}
	ws.server.webSocketConnections = resultList
	//ws.server.Logger.Log(1, "WebSocket connection ended!")
}

/*
Get parameter of WebSocket Connection
*/
func (ws *HTTPServerWebSocketConnection) GetParameter(parameterName string) string {
	return ws.userParameters[parameterName]
}

/*
Find WebSocket connections by parameters
*/
func (webSocketServer *WebSocketHTTPServer) GetWebSocketConnections(params map[string]string) []*HTTPServerWebSocketConnection {
	if webSocketServer == nil {
		return nil
	}
	return FilterWebSocketConnections(webSocketServer.webSocketConnections, params)
}

/*
Filters WebSocket connections by parameters
*/
func FilterWebSocketConnections(wsConns []*HTTPServerWebSocketConnection, params map[string]string) []*HTTPServerWebSocketConnection {
	result := make([]*HTTPServerWebSocketConnection, 0)
	for i := 0; i < len(wsConns); i++ {
		isValid := true
		for k, v := range params {
			if wsConns[i].userParameters[k] != v {
				isValid = false
				break
			}
		}
		if isValid {
			result = append(result, wsConns[i])
		}
	}
	return result
}

/*
Sends WebSocket message to all connections in list
*/
func SendWebSocketMessage(wsConns []*HTTPServerWebSocketConnection, message string) {
	for i := 0; i < len(wsConns); i++ {
		wsConns[i].SendMessage(message)
	}
}

//func (ws *HTTPServerWebSocketConnection) readPing(_ net.Conn, msg string, hasEnded bool) {
//	ws.gotPing = true
//	ws.pingMessage = msg
//}

///*
//Pings client and waits for 10 seconds for responce -> Does not work with javascript clients, for now, do not know why
//*/
//func (ws *HTTPServerWebSocketConnection) Ping() bool {
//	//Send
//	ws.gotPing = false
//	ws.pingMessage = ""
//	frame := PackWebSocketFrame("PING", 9)
//	ws.server.Logger.Log(1, "Sending ping to client.")
//	_, err := ws.connection.Write(frame)
//	if err != nil {
//		ws.server.Logger.Log(3, "Error sending ping to server | Error: "+err.Error())
//	}
//
//	for i := 0; i < 10; i++ {
//		//Await for ping
//		if ws.gotPing {
//			break
//		}
//		time.Sleep(1 * time.Second)
//	}
//
//	return ws.gotPing && ws.pingMessage == "PING"
//}
