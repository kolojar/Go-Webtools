package webtools

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
)

const webSocketGuid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

/*
Creates WebSocket key from original security key and magic text
*/
func computeWebSocketKey(webSocketKey string) string {
	//New hasher
	sha1Hasher := sha1.New()
	//Hash SHA combined original key and magic text
	sha1Hasher.Write([]byte(webSocketKey + webSocketGuid))
	//Make sum of SHA key
	acceptValue := sha1Hasher.Sum(nil)
	//Encode byte to string
	return base64.StdEncoding.EncodeToString(acceptValue)
}

/*
Standardized type of function
*HTTPWebSocketServerConn = Connection
String = message
Uint8 = 0 = Open, 1 = Close, 2 = Read text, 3 = Read binary
*/
type HTTPWebSocketServerReadFunc func(*HTTPWebSocketServerConn, []byte, uint8)

/*
HTTP WebSocket server connection object
*/
type HTTPWebSocketServerConn struct {
	origin   *HTTPWebSocketServer
	Conn     *net.TCPConn
	IsBinary bool
}

/*
Sends data to client, it is set by first recieved packed, can be changed using IsBinary property
*/
func (httpConn *HTTPWebSocketServerConn) Send(data []byte) {
	if httpConn.IsBinary {
		httpConn.origin.WriteToClientBinary(httpConn.Conn, data)
	} else {
		httpConn.origin.WriteToClientText(httpConn.Conn, data)
	}
}

/*
Closes connection to client
*/
func (httpConn *HTTPWebSocketServerConn) Close() {
	err := httpConn.Conn.Close()
	if err != nil {
		httpConn.origin.Logger.Log(3, "Error closing connection from: "+httpConn.Conn.RemoteAddr().String()+" connected locally to: "+httpConn.Conn.LocalAddr().String()+" with error: "+err.Error())
	} else {
		httpConn.origin.Logger.Log(0, "Closed connectin on "+httpConn.Conn.RemoteAddr().String()+" connected locally to: "+httpConn.Conn.LocalAddr().String())
	}
}

/*
HTTP WebSocket server for JavaScript with standards
*/
type HTTPWebSocketServer struct {
	httpServer   *HTTPServer
	Logger       *ConsoleLogger
	conns        SafeMap[string, *HTTPWebSocketServerConn]
	readFunc     HTTPWebSocketServerReadFunc
	onAccessFunc HTTPAccessFunc
	websocketURL string
}

/*
Creates new HTTP WebSocket Server but does not starts it
*/
func NewHTTPWebSocketServer(address string, readFunc HTTPWebSocketServerReadFunc, onAccessFunc HTTPAccessFunc, rootPath string, reportTraffic bool) *HTTPWebSocketServer {
	level := uint8(0)
	if !reportTraffic {
		level = 1
	}
	sv := &HTTPWebSocketServer{Logger: NewConsoleLogger("HTTP-WSServer", level), readFunc: readFunc, conns: MakeSafeMap[string, *HTTPWebSocketServerConn](), onAccessFunc: onAccessFunc, websocketURL: "/websocket"}
	sv.httpServer = NewHTTPServer(address, sv.handleHTTPAccess, rootPath, false)
	sv.httpServer.Logger = sv.Logger
	return sv
}

/*
Sets URL of WebSocket
*/
func (sv *HTTPWebSocketServer) SetWebSocketURL(newURL string) error {
	if !strings.HasPrefix(newURL, "/") {
		return errors.New("url must start with /")
	}
	sv.websocketURL = newURL
	return nil
}

/*
Gets HTTP server
*/
func (sv *HTTPWebSocketServer) GetHTTPServer() *HTTPServer {
	return sv.httpServer
}

func (sv *HTTPWebSocketServer) handleHTTPAccess(_ *HTTPServer, w http.ResponseWriter, r *http.Request, params map[string]string) bool {
	if r.Method == http.MethodGet && r.URL.Path == sv.websocketURL {
		//Websocket request - Correct URL and Method
		sv.Logger.Log(1, "Preparing connection from: "+r.RemoteAddr)

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
			sv.Logger.Log(3, "Failed to hijact connection from: "+r.RemoteAddr+" | Error: "+err.Error())
			return true
		}
		sv.Logger.Log(2, "Connection from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String())
		go handleWebSocketFrameRead(conn.(*net.TCPConn), sv.Logger, sv.readFuncLocal)
		return true
	} else {
		//Normal request
		if sv.onAccessFunc != nil {
			return sv.onAccessFunc(sv.httpServer, w, r, params)
		}
	}
	return false
}

func handleWebSocketFrameRead(conn *net.TCPConn, logger *ConsoleLogger, readFunc func(*net.TCPConn, []byte, bool, bool)) {
	for {
		//Read header of frame
		header := make([]byte, 2)
		_, err := conn.Read(header)
		if err != nil {
			logger.Log(3, "Error reading header: "+err.Error())
			break
		}

		//Get opcode (operation code) (masked with bitshift operation AND)
		opcode := header[0] & 0xF

		//Sort opcodes
		if opcode == 8 {
			//Close / disconnected
			break
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
				logger.Log(3, "Error reading additional size of frame: "+err.Error())
				break
			}

			//Calculate new size
			payloadSize = int(binary.BigEndian.Uint16(sizeAdder))
		case 127:
			//Payload to long -> Longer than 65535 characters
			//Get new size of buffer encoded using big Endian - 64 bits
			sizeAdder := make([]byte, 8)
			_, err = conn.Read(sizeAdder)
			if err != nil {
				logger.Log(3, "Error reading additional size of frame: "+err.Error())
				break
			}

			//Calculate new size
			payloadSize = int(binary.BigEndian.Uint64(sizeAdder))
		}

		//Read masking key of frame
		maskingKey := make([]byte, 4)
		if hasMask == 1 {
			_, err = conn.Read(maskingKey)
			if err != nil {
				logger.Log(3, "Error reading mask of frame: "+err.Error())
				break
			}
		}

		//Payload data (message)
		payload := make([]byte, payloadSize)
		_, err = conn.Read(payload)
		if err != nil {
			logger.Log(3, "Error reading data of frame: "+err.Error())
			break
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
			writeToWebSocketFrame(conn, payload, 10, logger)
			continue
		}
		if opcode == 10 {
			//Got pong
			logger.Log(1, "Got pong - Ignoring read")
			continue
		}

		//Normal message
		if readFunc != nil {
			readFunc(conn, payload, opcode == 1, false)
		}
	}

	//Connection ended or error occured
	if readFunc != nil {
		readFunc(conn, nil, false, true)
	}
}

/*
Pack websocket frame
Sources: https://developer.mozilla.org/en-US/docs/Web/API/WebSockets_API/Writing_WebSocket_server
Sources: https://en.wikipedia.org/wiki/WebSocket#Opcodes
Some fixes applied from ChatGPT (big payloads)
OpCode must be in range form 0 to 16 (from Wikipedia) in hex format
*/
func writeToWebSocketFrame(conn *net.TCPConn, payload []byte, opcode uint8, logger *ConsoleLogger) {
	//Check opcode size
	if opcode >= 16 {
		logger.Log(3, "Opcode must be in range from 0 to 15 (less than 16), ignoring...")
		return
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

	//Send
	writeToTCP(conn, frame, logger)
}

func (sv *HTTPWebSocketServer) readFuncLocal(conn *net.TCPConn, data []byte, isBinary bool, ended bool) {
	var httpConn *HTTPWebSocketServerConn = sv.conns.Get(conn.RemoteAddr().String())
	if httpConn == nil {
		httpConn = &HTTPWebSocketServerConn{origin: sv, Conn: conn, IsBinary: isBinary}
		sv.conns.Set(conn.RemoteAddr().String(), httpConn)
	}
	// Check type
	if isBinary != httpConn.IsBinary {
		sv.Logger.Log(2, "Connection from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String()+" has got data that are marked as "+FormatByBool(isBinary, "binary", "text")+" but this connection is marked as "+FormatByBool(httpConn.IsBinary, "binary", "text")+". Consilider changing properties of websocketConnection.")
	}

	//Process read
	if sv.readFunc != nil {
		if !ended {
			sv.Logger.Log(0, "Reading from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
			sv.readFunc(httpConn, data, FormatByBool[uint8](isBinary, 3, 2))
		} else {
			sv.readFunc(httpConn, data, 1)
		}
	}
}

/*
Writes to Client with binary formating
*/
func (sv *HTTPWebSocketServer) WriteToClientBinary(conn *net.TCPConn, data []byte) {
	sv.WriteToClientOpcode(conn, data, 2)
}

/*
Writes to Client with text formating
*/
func (sv *HTTPWebSocketServer) WriteToClientText(conn *net.TCPConn, data []byte) {
	sv.WriteToClientOpcode(conn, data, 1)
}

/*
Writes to Client with opcode
*/
func (sv *HTTPWebSocketServer) WriteToClientOpcode(conn *net.TCPConn, data []byte, opcode uint8) {
	writeToWebSocketFrame(conn, data, opcode, sv.Logger)
}
