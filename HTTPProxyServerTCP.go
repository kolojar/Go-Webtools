package webtools

import (
	"encoding/hex"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"
)

const HTTP_PROXY_FRAME_SEPARATOR = byte(rune(';'))
const HTTP_PROXY_FRAME_TYPE_CONNECT = uint8(1)
const HTTP_PROXY_FRAME_TYPE_CLOSE = uint8(2)
const HTTP_PROXY_FRAME_TYPE_DATA = uint8(3)

/*
Copies sync.Map
*/
//func CopySyncMap(src sync.Map) sync.Map {
//	var dest sync.Map = sync.Map{}
//	src.Range(func(k, v any) bool {
//		vMap, ok := v.(sync.Map)
//		if ok {
//			dest.Store(k, CopySyncMap(vMap))
//		} else {
//			dest.Store(k, v)
//		}
//		return true
//	})
//	return dest
//}

/*
Packs HTTP Proxy frame
*/
func PackHTTPProxyFrame(operation uint8, id []byte, data []byte) []byte {
	frame := make([]byte, 0)
	frame = append(frame, operation)
	frame = append(frame, HTTP_PROXY_FRAME_SEPARATOR)
	frame = append(frame, id...)
	frame = append(frame, HTTP_PROXY_FRAME_SEPARATOR)
	if data != nil {
		frame = append(frame, data...)
	}
	return frame
}

/*
Unpacks HTTP Proxy frame, operation 0 means error
*/
func UnpackHTTPProxyFrame(frame []byte, logger *ConsoleLogger) (uint8, []byte, []byte) {
	//Check size
	if len(frame) < 2 {
		logger.Log(3, "Frame too short. | Data lenght: "+strconv.Itoa(len(frame))+" | Data in hex: "+hex.EncodeToString(frame))
		return 0, nil, nil
	}

	//Get operation
	operation := frame[0]
	if frame[1] != HTTP_PROXY_FRAME_SEPARATOR {
		logger.Log(3, "Invalid frame at index 1. | Data lenght: "+strconv.Itoa(len(frame))+" | Data in hex: "+hex.EncodeToString(frame))
		return 0, nil, nil
	}

	//Get id and rest of data
	var id []byte
	var data []byte
	for i := 2; i < len(frame); i++ {
		if frame[i] == HTTP_PROXY_FRAME_SEPARATOR {
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
	//idToClient       map[string]*HTTPProxyServerTCPConn
	//clientToId       map[*TCPClient]string
	idToClient       sync.Map
	clientToId       sync.Map
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
	cl.source.Send(PackHTTPProxyFrame(operation, cl.id, data))
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
	cl.tcpClient.Stop()
	cl.origin.idToClient.Delete(string(cl.id))
	if isInitiator {
		cl.SendToHTTP(HTTP_PROXY_FRAME_TYPE_CLOSE, nil)
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
	sv := &HTTPProxyServerTCP{tcpServerAddress: tcpServerAddress, clientToId: sync.Map{}, idToClient: sync.Map{}, reportTrafic: reportTraffic}
	sv.httpServer = NewHTTPWebTransportServer(httpProxyAddress, sv.handleWebTransportReadFunc, reportTraffic)
	sv.httpServer.Logger.Prefix = "HTTPProxyServerTCP - " + sv.httpServer.Logger.Prefix
	return sv
}

func (sv *HTTPProxyServerTCP) handleWebTransportReadFunc(conn *HTTPWebTransportServerConn, frame []byte, ended bool) {
	if ended {
		//Close all connections with this HTTP WebTransport Conn
		sv.idToClient.Range(func(_, val any) bool {
			v := val.(*HTTPProxyServerTCPConn)
			if v == nil {
				return true
			}
			if v.source == conn {
				v.Close(true)
			}
			return true
		})
	}

	//Unpack frame
	operation, id, data := UnpackHTTPProxyFrame(frame, conn.origin.Logger)
	if operation == 0 {
		return
	}

	//Sort connections
	gcl, _ := sv.idToClient.Load(string(id))
	if gcl == nil {
		if operation == HTTP_PROXY_FRAME_TYPE_CONNECT {
			//Create new connection
			id = []byte(GenerateRandomId())
			cl, err := NewTCPClient(sv.tcpServerAddress, sv.handleTCPReadFunc, sv.reportTrafic)
			cl.Logger.Prefix = "HTTPProxyServerTCP - " + cl.Logger.Prefix
			if err != nil {
				conn.origin.Logger.Log(3, "Could not create connection with id: "+string(id)+" to server.")
				return
			}
			cl.Connect()
			sv.idToClient.Store(string(id), &HTTPProxyServerTCPConn{tcpClient: cl, id: id, source: conn, origin: sv})
			sv.clientToId.Store(cl, string(id))
			gcl, _ = sv.idToClient.Load(string(id))
			gcl.(*HTTPProxyServerTCPConn).SendToHTTP(HTTP_PROXY_FRAME_TYPE_CONNECT, data)
			return
		} else {
			conn.origin.Logger.Log(3, "Could not find connection to id: "+string(id))
			return
		}
	}
	cl := gcl.(*HTTPProxyServerTCPConn)
	if !cl.tcpClient.IsAlive() {
		conn.origin.Logger.Log(3, "Connection with id: "+string(id)+" connected to: "+conn.Conn.RemoteAddr().String()+" connected locally to: "+conn.Conn.LocalAddr().String()+" closed")
		return
	}

	//Sort operations
	switch operation {
	case HTTP_PROXY_FRAME_TYPE_CLOSE:
		{
			//Close connection
			cl.Close(false)
		}
	case HTTP_PROXY_FRAME_TYPE_DATA:
		{
			//Send to TCP
			cl.SendToTCP(data)
		}
	}
}

func (sv *HTTPProxyServerTCP) handleTCPReadFunc(tcp *TCPClient, data []byte, ended bool) {
	//Get HTTP client
	id, _ := sv.clientToId.Load(tcp)
	if id == "" {
		//Connection does not exists
		tcp.Logger.Log(3, "Connection connected to: "+tcp.address.String()+" not found")
		return
	}
	gcl, _ := sv.idToClient.Load(id)
	if gcl == nil {
		//Connection does not exists
		tcp.Logger.Log(3, "Connection connected to: "+tcp.address.String()+" not found")
		return
	}
	cl := gcl.(*HTTPProxyServerTCPConn)

	//End other connection
	if ended {
		cl.Close(true)
	}

	//Send to client
	cl.SendToHTTP(HTTP_PROXY_FRAME_TYPE_DATA, data)
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
