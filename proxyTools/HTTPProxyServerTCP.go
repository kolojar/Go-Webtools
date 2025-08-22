package proxytools

import (
	"encoding/hex"
	"strconv"
	"webtools"
	httptools "webtools/httpTools"
	tcptools "webtools/tcpTools"
)

const PROXY_FRAME_SEPARATOR = byte(rune(';'))
const PROXY_FRAME_TYPE_CONNECT = uint8(1)
const PROXY_FRAME_TYPE_CLOSE = uint8(2)
const PROXY_FRAME_TYPE_DATA = uint8(3)

type UnpackedProxyFrame struct {
	Operation uint8
	Id        []byte
	Data      []byte
}

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
func UnpackProxyFrame(frame []byte, logger *webtools.ConsoleLogger) []UnpackedProxyFrame {
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
	var subframes []UnpackedProxyFrame
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
	result := make([]UnpackedProxyFrame, 0)
	result = append(result, UnpackedProxyFrame{Operation: operation, Id: id, Data: data})
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
	idToClient       webtools.SafeMap[string, *HTTPProxyServerTCPConn]
	clientToId       webtools.SafeMap[*tcptools.TCPClientSimple, string]
	httpServer       *httptools.WebSocketServer
	tcpServerAddress string
	reportTrafic     bool
}

/*
HTTP Proxy server for TCP connection object
*/
type HTTPProxyServerTCPConn struct {
	tcpClient *TCPClientSimple
	id        []byte
	source    *WebSocketServerConn
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
	sv := &HTTPProxyServerTCP{tcpServerAddress: tcpServerAddress, clientToId: webtools.MakeSafeMap[*tcptools.TCPClientSimple, string](), idToClient: webtools.MakeSafeMap[string, *HTTPProxyServerTCPConn](), reportTrafic: reportTraffic}
	sv.httpServer = httptools.NewHTTPWebSocketServer(httpProxyAddress, sv.handleWebSocketReadFunc, nil, "", reportTraffic)
	sv.httpServer.Logger.Prefix = "HTTPProxyServerTCP - " + sv.httpServer.Logger.Prefix
	return sv
}

func (sv *HTTPProxyServerTCP) handleWebSocketReadFunc(conn *httptools.WebSocketServerConn, frame []byte, status uint8, isBinary bool) {
	if status == webtools.TCP_CONNECT_STATUS {
		conn.IsBinary = true
		return
	}
	if status == webtools.TCP_DISCONNECT_STATUS {
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
	if status != webtools.TCP_READ_DATA_STATUS {
		return
	}

	//Unpack frame
	for _, frame := range UnpackProxyFrame(frame, sv.httpServer.Logger) {
		if frame.Operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(frame.Id)) == nil {
			if frame.Operation == PROXY_FRAME_TYPE_CONNECT {
				//Create new connection
				frame.Id = []byte(webtools.GenerateRandomId())
				cl, err := tcptools.NewTCPClientSimple(sv.tcpServerAddress, -1, false, sv.handleTCPReadFunc, sv.reportTrafic)
				cl.GetLogger().Prefix = "HTTPProxyServerTCP - " + cl.GetLogger().Prefix
				if err != nil {
					conn.origin.Logger.Log(3, "Could not create connection with id: "+string(frame.Id)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.Id), &HTTPProxyServerTCPConn{tcpClient: cl, id: frame.Id, source: conn, origin: sv})
				sv.clientToId.Set(cl, string(frame.Id))
				sv.idToClient.Get(string(frame.Id)).SendToHTTP(PROXY_FRAME_TYPE_CONNECT, frame.Data)
				return
			} else {
				conn.origin.Logger.Log(3, "Could not find connection to id: "+string(frame.Id))
				return
			}
		}
		cl := sv.idToClient.Get(string(frame.Id))
		if !cl.tcpClient.IsAlive() {
			conn.origin.Logger.Log(3, "Connection with id: "+string(frame.Id)+" connected to: "+conn.GetConn().RemoteAddr().String()+" connected locally to: "+conn.GetConn().LocalAddr().String()+" closed")
			return
		}

		//Sort operations
		switch frame.Operation {
		case PROXY_FRAME_TYPE_CLOSE:
			{
				//Close connection
				cl.Close(false)
			}
		case PROXY_FRAME_TYPE_DATA:
			{
				//Send to TCP
				cl.SendToTCP(frame.Data)
			}
		}
	}
}

func (sv *HTTPProxyServerTCP) handleTCPReadFunc(tcp *tcptools.TCPClientSimple, data []byte, status uint8) {
	if status == webtools.TCP_CONNECT_STATUS {
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
	if status == webtools.TCP_DISCONNECT_STATUS {
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
