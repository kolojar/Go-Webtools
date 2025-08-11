package webtools

import (
	"encoding/hex"
	"math/rand/v2"
	"strconv"
	"time"
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
	if data != nil {
		frame = append(frame, data...)
	}
	return frame
}

/*
Unpacks Proxy frame, operation 0 means error
*/
func UnpackProxyFrame(frame []byte, logger *ConsoleLogger) (uint8, []byte, []byte) {
	//Check size
	if len(frame) < 2 {
		logger.Log(3, "Frame too short. | Data lenght: "+strconv.Itoa(len(frame))+" | Data in hex: "+hex.EncodeToString(frame))
		return 0, nil, nil
	}

	//Get operation
	operation := frame[0]
	if frame[1] != PROXY_FRAME_SEPARATOR {
		logger.Log(3, "Invalid frame at index 1. | Data lenght: "+strconv.Itoa(len(frame))+" | Data in hex: "+hex.EncodeToString(frame))
		return 0, nil, nil
	}

	//Get id and rest of data
	var id []byte
	var data []byte
	for i := 2; i < len(frame); i++ {
		if frame[i] == PROXY_FRAME_SEPARATOR {
			id = frame[2:i]
			if len(frame) > (i + 1) {
				data = frame[i+1:]
			}
			break
		}
	}
	return operation, id, data
}

/*
HTTP Proxy server for TCP object
*/
type HTTPProxyServerTCP struct {
	idToClient       SafeMap[string, *HTTPProxyServerTCPConn]
	clientToId       SafeMap[*TCPClient, string]
	httpServer       *HTTPWebTransportServer
	tcpServerAddress string
	reportTrafic     bool
}

/*
HTTP Proxy server for TCP connection object
*/
type HTTPProxyServerTCPConn struct {
	tcpClient *TCPClient
	id        []byte
	source    *HTTPWebTransportServerConn
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
Generates random Id based on random and current time
*/
func GenerateRandomId() string {
	return strconv.FormatUint(rand.Uint64(), 36) + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

/*
Creates new HTTP Proxy Server for TCP but does not starts it
*/
func NewHTTPProxyServerTCP(httpProxyAddress string, tcpServerAddress string, reportTraffic bool) *HTTPProxyServerTCP {
	sv := &HTTPProxyServerTCP{tcpServerAddress: tcpServerAddress, clientToId: MakeSafeMap[*TCPClient, string](), idToClient: MakeSafeMap[string, *HTTPProxyServerTCPConn](), reportTrafic: reportTraffic}
	sv.httpServer = NewHTTPWebTransportServer(httpProxyAddress, sv.handleWebTransportReadFunc, reportTraffic)
	sv.httpServer.Logger.Prefix = "HTTPProxyServerTCP - " + sv.httpServer.Logger.Prefix
	return sv
}

func (sv *HTTPProxyServerTCP) handleWebTransportReadFunc(conn *HTTPWebTransportServerConn, frame []byte, ended bool) {
	if ended {
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

	//Unpack frame
	operation, id, data := UnpackProxyFrame(frame, conn.origin.Logger)
	if operation == 0 {
		return
	}

	//Sort connections
	if sv.idToClient.Get(string(id)) == nil {
		if operation == PROXY_FRAME_TYPE_CONNECT {
			//Create new connection
			id = []byte(GenerateRandomId())
			cl, err := NewTCPClient(sv.tcpServerAddress, sv.handleTCPReadFunc, sv.reportTrafic)
			cl.Logger.Prefix = "HTTPProxyServerTCP - " + cl.Logger.Prefix
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
		conn.origin.Logger.Log(3, "Connection with id: "+string(id)+" connected to: "+conn.Conn.RemoteAddr().String()+" connected locally to: "+conn.Conn.LocalAddr().String()+" closed")
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

func (sv *HTTPProxyServerTCP) handleTCPReadFunc(tcp *TCPClient, data []byte, ended bool) {
	//Get HTTP client
	if sv.clientToId.Get(tcp) == "" || sv.idToClient.Get(sv.clientToId.Get(tcp)) == nil {
		//Connection does not exists
		tcp.Logger.Log(3, "Connection connected to: "+tcp.address.String()+" not found")
		return
	}
	id := sv.clientToId.Get(tcp)
	cl := sv.idToClient.Get(id)

	//End other connection
	if ended {
		cl.Close(true)
	}

	//Send to client
	cl.SendToHTTP(PROXY_FRAME_TYPE_DATA, data)
}

/*
Starts HTTP Proxy Server for TCP
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
