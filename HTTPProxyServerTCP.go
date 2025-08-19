package webtools

import (
	"encoding/hex"
	"strconv"
)

const PROXY_FRAME_SEPARATOR = byte(rune(';'))
const PROXY_FRAME_TYPE_CONNECT = uint8(1)
const PROXY_FRAME_TYPE_CLOSE = uint8(2)
const PROXY_FRAME_TYPE_DATA = uint8(3)

/*
Packs Proxy frame
*/
func PackProxyFrame(operation uint8, id []byte, data []byte) []byte {
	frame := make([]byte, 0)
	frame = append(frame, operation)
	frame = append(frame, PROXY_FRAME_SEPARATOR)
	frame = append(frame, id...)
	frame = append(frame, PROXY_FRAME_SEPARATOR)
	frame = append(frame, []byte(strconv.Itoa(len(data)))...)
	frame = append(frame, PROXY_FRAME_SEPARATOR)
	if data != nil {
		frame = append(frame, data...)
	}
	return frame
}

/*
Unpacks Proxy frame, operation 0 means error
*/
func UnpackProxyFrame(frame []byte, logger *ConsoleLogger) []ThreeValuePair[uint8, []byte, []byte] {
	//Invalid frame
	if len(frame) == 0 {
		return nil
	}

	//Check size
	if len(frame) < 2 {
		logger.Log(3, "Frame too short. | Data lenght: "+strconv.Itoa(len(frame))+" | Data in hex: "+hex.EncodeToString(frame))
		return nil
	}

	//Get operation
	operation := frame[0]
	if frame[1] != PROXY_FRAME_SEPARATOR {
		logger.Log(3, "Invalid frame at index 1. | Data lenght: "+strconv.Itoa(len(frame))+" | Data in hex: "+hex.EncodeToString(frame))
		return nil
	}

	//Get id and len of rest of frame
	var id []byte
	var idEndIndex int = -1
	var data []byte
	var subframes []ThreeValuePair[uint8, []byte, []byte]
	for i := 2; i < len(frame); i++ {
		if frame[i] == PROXY_FRAME_SEPARATOR {
			if idEndIndex == -1 {
				//Get id
				id = frame[2:i]
				idEndIndex = i
			} else {
				//Get len of data
				lenOfDataStr := frame[idEndIndex+1 : i]
				lenOfData, err := strconv.Atoi(string(lenOfDataStr))
				if lenOfData > 0 {
					lenOfData = lenOfData - 1
				}
				if err != nil {
					logger.Log(3, "Invalid frame lenght. | Data lenght: "+strconv.Itoa(len(frame))+" | Data in hex: "+hex.EncodeToString(frame)+" | Error: "+err.Error())
					return nil
				}

				//Get data
				if len(frame) > (i + lenOfData + 1) {
					data = frame[i+1 : i+2+lenOfData]
				}

				//Get rest of data
				if len(frame) > (i + lenOfData + 1) {
					subframes = UnpackProxyFrame(frame[i+2+lenOfData:], logger)
				}
				break
			}
		}

	}

	//Make result
	result := make([]ThreeValuePair[uint8, []byte, []byte], 0)
	result = append(result, ThreeValuePair[uint8, []byte, []byte]{A: operation, B: id, C: data})
	if subframes != nil {
		result = append(result, subframes...)
	}
	//fmt.Println(len(result))
	return result
}

/*
HTTP Proxy server for TCP object
*/
type HTTPProxyServerTCP struct {
	idToClient       SafeMap[string, *HTTPProxyServerTCPConn]
	clientToId       SafeMap[*TCPClientSimple, string]
	httpServer       *HTTPWebSocketServer
	tcpServerAddress string
	reportTrafic     bool
}

/*
HTTP Proxy server for TCP connection object
*/
type HTTPProxyServerTCPConn struct {
	tcpClient *TCPClientSimple
	id        []byte
	source    *HTTPWebSocketServerConn
	origin    *HTTPProxyServerTCP
}

/*
Creates frame and sends it to HTTP
*/
func (cl *HTTPProxyServerTCPConn) SendToHTTP(operation uint8, data []byte) {
	cl.source.Send(PackProxyFrame(operation, cl.id, data))
}

/*
Creates frame and sends it to TCP
*/
func (cl *HTTPProxyServerTCPConn) SendToTCP(data []byte) {
	cl.tcpClient.Send(data)
}

/*
Closes connection to client
*/
func (cl *HTTPProxyServerTCPConn) Close(isInitiator bool) {
	if cl == nil || cl.tcpClient == nil {
		return
	}
	cl.tcpClient.Stop()
	cl.origin.idToClient.Delete(string(cl.id))
	if isInitiator {
		cl.SendToHTTP(PROXY_FRAME_TYPE_CLOSE, nil)
	}
	cl.origin.clientToId.Delete(cl.tcpClient)
}

/*
Creates new HTTP Proxy Server for TCP but does not starts it
*/
func NewHTTPProxyServerTCP(httpProxyAddress string, tcpServerAddress string, reportTraffic bool) *HTTPProxyServerTCP {
	sv := &HTTPProxyServerTCP{tcpServerAddress: tcpServerAddress, clientToId: MakeSafeMap[*TCPClientSimple, string](), idToClient: MakeSafeMap[string, *HTTPProxyServerTCPConn](), reportTrafic: reportTraffic}
	sv.httpServer = NewHTTPWebSocketServer(httpProxyAddress, sv.handleWebSocketReadFunc, nil, "", reportTraffic)
	sv.httpServer.Logger.Prefix = "HTTPProxyServerTCP - " + sv.httpServer.Logger.Prefix
	return sv
}

func (sv *HTTPProxyServerTCP) handleWebSocketReadFunc(conn *HTTPWebSocketServerConn, frame []byte, status uint8, isBinary bool) {
	if status == TCP_CONNECT_STATUS {
		conn.IsBinary = true
		return
	}
	if status == TCP_DISCONNECT_STATUS {
		//Close all connections with this HTTP WebTransport Conn
		for _, d := range sv.idToClient.GetData() {
			if d.Value == nil {
				continue
			}
			if d.Value.source == conn {
				d.Value.Close(true)
			}
		}
		return
	}
	if status != TCP_READ_DATA_STATUS {
		return
	}

	//Unpack frame
	for _, frame := range UnpackProxyFrame(frame, sv.httpServer.Logger) {
		operation, id, data := frame.A, frame.B, frame.C
		if operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(id)) == nil {
			if operation == PROXY_FRAME_TYPE_CONNECT {
				//Create new connection
				id = []byte(GenerateRandomId())
				cl, err := NewTCPClientSimple(sv.tcpServerAddress, -1, false, sv.handleTCPReadFunc, sv.reportTrafic)
				cl.GetLogger().Prefix = "HTTPProxyServerTCP - " + cl.GetLogger().Prefix
				if err != nil {
					conn.origin.Logger.Log(3, "Could not create connection with id: "+string(id)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(id), &HTTPProxyServerTCPConn{tcpClient: cl, id: id, source: conn, origin: sv})
				sv.clientToId.Set(cl, string(id))
				sv.idToClient.Get(string(id)).SendToHTTP(PROXY_FRAME_TYPE_CONNECT, data)
				return
			} else {
				conn.origin.Logger.Log(3, "Could not find connection to id: "+string(id))
				return
			}
		}
		cl := sv.idToClient.Get(string(id))
		if !cl.tcpClient.IsAlive() {
			conn.origin.Logger.Log(3, "Connection with id: "+string(id)+" connected to: "+conn.GetConn().RemoteAddr().String()+" connected locally to: "+conn.GetConn().LocalAddr().String()+" closed")
			return
		}

		//Sort operations
		switch operation {
		case PROXY_FRAME_TYPE_CLOSE:
			{
				//Close connection
				cl.Close(false)
			}
		case PROXY_FRAME_TYPE_DATA:
			{
				//Send to TCP
				cl.SendToTCP(data)
			}
		}
	}
}

func (sv *HTTPProxyServerTCP) handleTCPReadFunc(tcp *TCPClientSimple, data []byte, status uint8) {
	if status == TCP_CONNECT_STATUS {
		return
	}

	//Get HTTP client
	if sv.clientToId.Get(tcp) == "" || sv.idToClient.Get(sv.clientToId.Get(tcp)) == nil {
		//Connection does not exists
		tcp.GetLogger().Log(3, "Connection connected to: "+tcp.universalClient.address.String()+" not found")
		return
	}
	id := sv.clientToId.Get(tcp)
	cl := sv.idToClient.Get(id)

	//End other connection
	if status == TCP_DISCONNECT_STATUS {
		cl.Close(true)
	}

	//Send to client
	cl.SendToHTTP(PROXY_FRAME_TYPE_DATA, data)
}

/*
Starts HTTP Proxy Server for TCP. Locks execution thread
*/
func (sv *HTTPProxyServerTCP) Start() {
	sv.httpServer.Start()
}

/*
Stops HTTP Proxy Server for TCP
*/
func (sv *HTTPProxyServerTCP) Stop() {
	sv.httpServer.Stop()
}

func (sv *HTTPProxyServerTCP) IsAlive() bool {
	return sv.httpServer.IsAlive()
}
func (sv *HTTPProxyServerTCP) GetAddress() string {
	return sv.httpServer.GetAddress()
}
