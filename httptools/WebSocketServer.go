package httptools

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"

	webtools "github.com/kolojar/Go-Webtools"
	"github.com/kolojar/Go-Webtools/helpertools"
	"github.com/kolojar/Go-Webtools/tcp"
)

const webSocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

/*
Creates WebSocket key from original security key and magic text
*/
func computeWebSocketKey(webSocketKey string) string {
	//New hasher
	sha1Hasher := sha1.New()
	//Hash SHA combined original key and magic text
	sha1Hasher.Write([]byte(webSocketKey + webSocketGUID))
	//Make sum of SHA key
	acceptValue := sha1Hasher.Sum(nil)
	//Encode byte to string
	return base64.StdEncoding.EncodeToString(acceptValue)
}

/*
WebSocketServerReadFunc is function definition for reading data from WebSocketServer
*/
type WebSocketServerReadFunc func(conn *WebSocketServerConn, data []byte, status webtools.NetworkStatus, isBinary bool)

/*
WebSocketServerConn is connection object of WebSocketServer
*/
type WebSocketServerConn struct {
	origin    *WebSocketServer
	Client    *tcp.ClientUniversal
	IsBinary  bool
	firstRead bool
	urlParams map[string]string
	sourceURL string
	Cookies   []*http.Cookie
}

/*
GetConn gets raw TCP connection
*/
func (httpConn *WebSocketServerConn) GetConn() *net.TCPConn {
	return httpConn.Client.GetConn()
}

/*
GetCookie gets specific cookie
*/
func (httpConn *WebSocketServerConn) GetCookie(name string) *http.Cookie {
	for _, v := range httpConn.Cookies {
		if v.Name == name {
			return v
		}
	}
	return nil
}

/*
Send sends data to client, it is set by first recieved packed, can be changed using IsBinary property
*/
func (httpConn *WebSocketServerConn) Send(data []byte) {
	httpConn.Client.Send(data, map[string]any{"opcode": helpertools.FormatByBool[uint8](httpConn.IsBinary, 2, 1)})
}

/*
Close closes connection to client
*/
func (httpConn *WebSocketServerConn) Close() {
	httpConn.Client.Stop()
}

/*
GetURLParameter gets URL parameter from original HTTP request
*/
func (httpConn *WebSocketServerConn) GetURLParameter(key string) string {
	return httpConn.urlParams[key]
}

/*
SetURLParameter sets URL parameter from original HTTP request
*/
func (httpConn *WebSocketServerConn) SetURLParameter(key string, value string) {
	httpConn.urlParams[key] = value
}

/*
RemoveURLParameter removes URL parameter from original HTTP request
*/
func (httpConn *WebSocketServerConn) RemoveURLParameter(key string) {
	delete(httpConn.urlParams, key)
}

/*
WebSocketServer is HTTP WebSocket server for JavaScript with standards
*/
type WebSocketServer struct {
	httpServer                *Server
	conns                     helpertools.SafeMap[*tcp.ClientUniversal, *WebSocketServerConn]
	onAccessFunc              AccessFunc
	websocketURLsAndReadFuncs helpertools.SafeMap[string, WebSocketServerReadFunc]
	reportTraffic             bool
}

/*
IsAlive gets if server is alive
*/
func (sv *WebSocketServer) IsAlive() bool {
	return sv.httpServer.IsAlive()
}

/*
GetAddress gets address of server
*/
func (sv *WebSocketServer) GetAddress() string {
	return sv.httpServer.GetAddress()
}

/*
GetLogger gets logger
*/
func (sv *WebSocketServer) GetLogger() *helpertools.ConsoleLogger {
	return sv.httpServer.Logger
}

/*
NewWebSocketServer creates new HTTP WebSocket Server but does not starts it
This readFunc is asociated with "/websocket" url
*/
func NewWebSocketServer(address string, readFunc WebSocketServerReadFunc, onAccessFunc AccessFunc, rootPath string, startWebBrowser bool, reportHTTPTraffic bool, reportWebSocketTraffic bool) *WebSocketServer {
	wsURLAndFuncs := helpertools.MakeSafeMap[string, WebSocketServerReadFunc]()
	wsURLAndFuncs.Set("/websocket", readFunc)
	sv := &WebSocketServer{reportTraffic: reportWebSocketTraffic, conns: helpertools.MakeSafeMap[*tcp.ClientUniversal, *WebSocketServerConn](), onAccessFunc: onAccessFunc, websocketURLsAndReadFuncs: wsURLAndFuncs}
	sv.httpServer = NewServer(address, sv.handleHTTPAccess, rootPath, startWebBrowser, reportHTTPTraffic)
	sv.httpServer.Logger.Prefix = "HTTP-WSServer"
	return sv
}

/*
AddWebSocketURL adds URL of WebSocket
*/
func (sv *WebSocketServer) AddWebSocketURL(url string, readFunc WebSocketServerReadFunc) error {
	if !strings.HasPrefix(url, "/") {
		return errors.New("url must start with /")
	}
	sv.websocketURLsAndReadFuncs.Set(url, readFunc)
	return nil
}

/*
RemoveWebSocketURL removes URL of WebSocket
*/
func (sv *WebSocketServer) RemoveWebSocketURL(url string) {
	sv.websocketURLsAndReadFuncs.Delete(url)
}

/*
GetHTTPServer gets HTTP server
*/
func (sv *WebSocketServer) GetHTTPServer() *Server {
	return sv.httpServer
}

func (sv *WebSocketServer) handleHTTPAccess(_ *Server, w http.ResponseWriter, r *http.Request, params map[string]string) bool {
	if r.Method == http.MethodGet && slices.Contains(sv.websocketURLsAndReadFuncs.GetKeys(), r.URL.Path) {
		//Websocket request - Correct URL and Method
		sv.httpServer.Logger.Log(1, "Preparing connection from: "+r.RemoteAddr)

		//Verify if connection wants WebSocket
		if !strings.Contains(r.Header.Get("Upgrade"), "websocket") || !strings.Contains(r.Header.Get("Connection"), "Upgrade") {
			http.Error(w, "Invalid WebSocket request", http.StatusBadRequest)
			return false
		}

		//Get Websocket key and check it the key is present
		webSocketKey := r.Header.Get("Sec-WebSocket-Key")
		if webSocketKey == "" {
			http.Error(w, "Missing WebSocket Key", http.StatusBadRequest)
			return false
		}

		//Compute security WebSocket key
		secureKey := computeWebSocketKey(webSocketKey)

		//Valid connection
		w.Header().Set("Upgrade", "websocket")
		w.Header().Set("Connection", "Upgrade")
		w.Header().Set("Sec-WebSocket-Accept", secureKey)

		//Request to switch to Webtransport keep-alive connection
		w.WriteHeader(http.StatusSwitchingProtocols)

		//Hijack connection
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			sv.httpServer.Logger.Log(3, "Failed to hijact connection from: "+r.RemoteAddr+" | Error: "+err.Error())
			return true
		}

		//Make client
		cl := tcp.NewTCPClientUniversalFromConnection(conn.(*net.TCPConn), sv.reportTraffic)
		cl.Logger = sv.httpServer.Logger
		cl.HandlerFuncs = append(cl.HandlerFuncs,
			tcp.ClientUniversalHanderFuncs{
				UseCount:               -1,
				ReadHandler:            HandleWebSocketFrameRead,
				ReadFunc:               sv.readFuncLocal,
				WriteHandler:           WriteToWebSocketFrameHandler,
				CanOneWriteAfterSwitch: false,
			})
		sv.conns.Set(cl, &WebSocketServerConn{origin: sv, Client: cl, urlParams: params, IsBinary: false, firstRead: true, sourceURL: r.URL.Path, Cookies: r.Cookies()})
		cl.Connect()

		//sv.Logger.Log(2, "Connection from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String())
		//go handleWebSocketFrameRead(conn.(*net.TCPConn), sv.Logger, sv.readFuncLocal)
		return true
	}

	//Normal request
	if sv.onAccessFunc != nil {
		return sv.onAccessFunc(sv.httpServer, w, r, params)
	}
	return false
}

/*
HandleWebSocketFrameRead handles reading of WebSocket frame, is used in TCPClientUniversal
*/
func HandleWebSocketFrameRead(cl *tcp.ClientUniversal, limit int, logger *helpertools.ConsoleLogger, readFunc tcp.ClientUniversalOnReadFuncIntenal) (bool, error) {
	for i := 0; i < limit || limit < 0; i++ {
		//Read header of frame
		header := make([]byte, 2)
		_, err := io.ReadFull(cl.GetConn(), header)
		if err != nil {
			return true, errors.New("error reading frame header | Error: " + err.Error())
		}

		//Get opcode (operation code) (masked with bitshift operation AND)
		opcode := header[0] & 0xF

		//Sort opcodes
		if opcode == 8 {
			//Close / disconnected
			if readFunc != nil {
				readFunc(nil, map[string]any{"disconected": true})
			}
			return true, nil
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
			_, err = io.ReadFull(cl.GetConn(), sizeAdder)
			if err != nil {
				return true, errors.New("error reading additional size of frame: " + err.Error())
			}

			//Calculate new size
			payloadSize = int(binary.BigEndian.Uint16(sizeAdder))
		case 127:
			//Payload to long -> Longer than 65535 characters
			//Get new size of buffer encoded using big Endian - 64 bits
			sizeAdder := make([]byte, 8)
			_, err = cl.GetConn().Read(sizeAdder)
			if err != nil {
				return true, errors.New("error reading additional size of frame: " + err.Error())
			}

			//Calculate new size
			payloadSize = int(binary.BigEndian.Uint64(sizeAdder))
		}

		//Read masking key of frame
		maskingKey := make([]byte, 4)
		if hasMask == 1 {
			_, err = io.ReadFull(cl.GetConn(), maskingKey)
			if err != nil {
				return true, errors.New("error reading mask of frame: " + err.Error())
			}
		}

		//Payload data (message)
		payload := make([]byte, payloadSize)
		_, err = io.ReadFull(cl.GetConn(), payload)
		if err != nil {
			return true, errors.New("error reading data of frame: " + err.Error())
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
			logger.Log(1, "Got ping - Sending pong responce...")
			err := WriteToWebSocketFrameHandler(cl, payload, map[string]any{"opcode": uint8(10)})
			if err != nil {
				logger.Log(3, "Error sending pong: "+err.Error())
			}
			continue
		}
		if opcode == 10 {
			//Got pong
			logger.Log(1, "Got pong - Ignoring read")
			continue
		}

		//Normal message
		if readFunc != nil {
			readFunc(payload, map[string]any{"isBinary": opcode == 2})
		}
	}

	//Connection ended
	return false, nil
}

/*
PackWebSocketFrame packs websocket frame
Sources: https://developer.mozilla.org/en-US/docs/Web/API/WebSockets_API/Writing_WebSocket_server
Sources: https://en.wikipedia.org/wiki/WebSocket#Opcodes
Some fixes applied from ChatGPT (big payloads)
OpCode must be in range form 0 to 16 (from Wikipedia) in hex format
*/
func PackWebSocketFrame(payload []byte, opcode uint8, logger *helpertools.ConsoleLogger) []byte {
	//Check opcode size
	if opcode >= 16 {
		logger.Log(3, "Opcode must be in range from 0 to 15 (less than 16), ignoring...")
		return nil
	}

	//Create frame and payload (message array) -> Example for messages: 0x81
	frame := []byte{byte(128 + opcode)}

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
WriteToWebSocketFrameHandler handles packing frame and wrtiting it to WebSocket, is used in TCPClientUniversal
*/
func WriteToWebSocketFrameHandler(cl *tcp.ClientUniversal, data []byte, otherData map[string]any) error {
	//Get opcode
	opcode := otherData["opcode"]
	if opcode == nil || opcode == "" {
		//Invalid opcode
		return errors.New("no property 'opcode' found in otherData")
	}

	//Send
	//	writeToTCP(conn, PackWebSocketFrame(payload, opcode, logger), logger)
	return tcp.WriteToTCPHandler(cl, PackWebSocketFrame(data, opcode.(uint8), cl.Logger), otherData)
}

func (sv *WebSocketServer) readFuncLocal(cl *tcp.ClientUniversal, data []byte, status webtools.NetworkStatus, otherData map[string]any) {
	if otherData["disconected"] == true {
		status = webtools.DisconnectStatus
	}
	if status != webtools.ReadDataStatus && status != webtools.ConnectStatus && status != webtools.DisconnectStatus {
		//Non data requests
		return
	}

	//Get connection
	var httpConn *WebSocketServerConn = sv.conns.Get(cl)
	if httpConn == nil {
		if status != webtools.DisconnectStatus {
			sv.httpServer.Logger.Log(3, "Connection for client connected from: "+cl.GetConn().RemoteAddr().String()+" connected locally to: "+cl.GetConn().LocalAddr().String()+" not found!")
		}
		return
	}

	//Get read
	readFunc := sv.websocketURLsAndReadFuncs.Get(httpConn.sourceURL)
	if status == webtools.ConnectStatus || status == webtools.DisconnectStatus {
		if status == webtools.DisconnectStatus {
			fmt.Println("Disconecting: " + cl.GetAddress().String())
			cl.Stop()
			httpConn.Close()
			sv.conns.Delete(cl)
		}
		if readFunc != nil {
			readFunc(httpConn, data, status, httpConn.IsBinary)
		}
	}

	//Get isBinary
	isBinaryRaw := otherData["isBinary"]
	if isBinaryRaw == nil || isBinaryRaw == "" {
		sv.httpServer.Logger.Log(3, "No property 'isBinary' found in otherData")
		return
	}
	isBinary := isBinaryRaw.(bool)

	//Sort if first read
	if httpConn.firstRead {
		httpConn.firstRead = false
		httpConn.IsBinary = isBinary
	}

	// Check type
	if isBinary != httpConn.IsBinary {
		sv.httpServer.Logger.Log(2, "Connection from: "+cl.GetConn().RemoteAddr().String()+" connected locally to: "+cl.GetConn().LocalAddr().String()+" has got data that are marked as "+helpertools.FormatByBool(isBinary, "binary", "text")+" but this connection is marked as "+helpertools.FormatByBool(httpConn.IsBinary, "binary", "text")+". Consilider changing properties of websocketConnection.")
	}

	//Process read
	if readFunc != nil {
		readFunc(httpConn, data, status, isBinary)
	}
}

/*
Writes to Client with binary formating
*/
//func (sv *HTTPWebSocketServer) WriteToClientBinary(conn *net.TCPConn, data []byte) {
//	sv.WriteToClientOpcode(conn, data, 2)
//}

/*
Writes to Client with text formating
*/
//func (sv *HTTPWebSocketServer) WriteToClientText(conn *net.TCPConn, data []byte) {
//	sv.WriteToClientOpcode(conn, data, 1)
//}

/*
Writes to Client with opcode
*/
//func (sv *HTTPWebSocketServer) WriteToClientOpcode(conn *net.TCPConn, data []byte, opcode uint8) {
//	writeToWebSocketFrame(conn, data, opcode, sv.Logger)
//}

/*
Start starts WebSocket HTTP Server. Locks execution thread
*/
func (sv *WebSocketServer) Start() {
	sv.httpServer.Start()
}

/*
Stop stops WebSocket HTTP Server
*/
func (sv *WebSocketServer) Stop() {
	sv.httpServer.Stop()
}

/*
BroadcastToClients broadcasts data to clients with specific url parameter/s supplied in filter, set filter to nil for all connections
*/
func (sv *WebSocketServer) BroadcastToClients(filterURLParams map[string]string, data []byte) {
	if sv != nil {
		BroadcastToWebSocketClients(sv.conns.GetValues(), filterURLParams, data)
	}
}

/*
FilterClients filters WebSocket connections matching URL parameters
*/
func (sv *WebSocketServer) FilterClients(filterURLParams map[string]string) []*WebSocketServerConn {
	return FilterWebSocketClients(sv.conns.GetValues(), filterURLParams)
}

/*
FilterWebSocketClients filters WebSocket connections matching URL parameters
*/
func FilterWebSocketClients(clients []*WebSocketServerConn, filterURLParams map[string]string) []*WebSocketServerConn {
	result := make([]*WebSocketServerConn, 0)
	for _, v := range clients {
		if filterURLParams != nil {
			//Check parameters
			var invalid = false
			for k, v2 := range filterURLParams {
				if v.urlParams[k] != v2 {
					invalid = true
					break
				}
			}
			if invalid {
				//Some do not match, skip connection
				continue
			}
		}
		result = append(result, v)
	}
	return result
}

/*
BroadcastToWebSocketClients broadcasts data to clients with specific url parameter/s supplied in filter, set filter to nil for all connections
*/
func BroadcastToWebSocketClients(clients []*WebSocketServerConn, filterURLParams map[string]string, data []byte) {
	for _, v := range FilterWebSocketClients(clients, filterURLParams) {
		v.Send(data)
	}
}
