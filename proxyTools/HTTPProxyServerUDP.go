package proxytools

import (
	"webtools"
	httptools "webtools/httpTools"
)

/*
HTTP Proxy server for UDP object
*/
type HTTPProxyServerUDP struct {
	idToClient       webtools.SafeMap[string, *HTTPProxyServerUDPConn]
	clientToId       webtools.SafeMap[*UDPClient, string]
	httpServer       *httptools.WebSocketServer
	udpServerAddress string
	reportTrafic     bool
}

/*
HTTP Proxy server for UDP connection object
*/
type HTTPProxyServerUDPConn struct {
	udpClient *UDPClient
	id        []byte
	source    *httptools.WebSocketServerConn
	origin    *HTTPProxyServerUDP
}

/*
Creates frame and sends it to HTTP
*/
func (cl *HTTPProxyServerUDPConn) SendToHTTP(operation uint8, data []byte) {
	cl.source.Send(PackProxyFrame(operation, cl.id, data))
}

/*
Creates frame and sends it to UDP
*/
func (cl *HTTPProxyServerUDPConn) SendToUDP(data []byte) {
	cl.udpClient.Send(data)
}

/*
Closes connection to client
*/
func (cl *HTTPProxyServerUDPConn) Close(isInitiator bool) {
	cl.udpClient.Stop()
	cl.origin.idToClient.Delete(string(cl.id))
	if isInitiator {
		cl.SendToHTTP(PROXY_FRAME_TYPE_CLOSE, nil)
	}
	cl.origin.clientToId.Delete(cl.udpClient)
}

/*
Creates new HTTP Proxy Server for UDP but does not starts it
*/
func NewHTTPProxyServerUDP(httpProxyAddress string, udpServerAddress string, reportTraffic bool) *HTTPProxyServerUDP {
	sv := &HTTPProxyServerUDP{udpServerAddress: udpServerAddress, clientToId: webtools.MakeSafeMap[*UDPClient, string](), idToClient: webtools.MakeSafeMap[string, *HTTPProxyServerUDPConn](), reportTrafic: reportTraffic}
	sv.httpServer = httptools.NewHTTPWebSocketServer(httpProxyAddress, sv.handleWebSocketReadFunc, nil, "", reportTraffic)
	sv.httpServer.Logger.Prefix = "HTTPProxyServerUDP - " + sv.httpServer.Logger.Prefix
	return sv
}

func (sv *HTTPProxyServerUDP) handleWebSocketReadFunc(conn *httptools.WebSocketServerConn, frame []byte, status uint8, isBinary bool) {
	if status == webtools.TCP_CONNECT_STATUS {
		conn.IsBinary = true
		return
	}
	if status == webtools.TCP_DISCONNECT_STATUS {
		//Close all connections with this HTTP WebTransport Conn
		for _, v := range sv.idToClient.GetValues() {
			if v == nil {
				continue
			}
			if v.source == conn {
				v.Close(true)
			}
		}
		return
	}
	if status != webtools.TCP_READ_DATA_STATUS {
		return
	}

	//Unpack frame
	for _, frame := range UnpackProxyFrame(frame, conn.origin.Logger) {
		if frame.Operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(frame.Id)) == nil {
			if frame.Operation == PROXY_FRAME_TYPE_CONNECT {
				//Create new connection
				frame.Id = []byte(webtools.GenerateRandomId())
				cl, err := NewUDPClient(sv.udpServerAddress, sv.handleUDPReadFunc, sv.reportTrafic)
				cl.Logger.Prefix = "HTTPProxyServerUDP - " + cl.Logger.Prefix
				if err != nil {
					conn.origin.Logger.Log(3, "Could not create connection with id: "+string(frame.Id)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.Id), &HTTPProxyServerUDPConn{udpClient: cl, id: frame.Id, source: conn, origin: sv})
				sv.clientToId.Set(cl, string(frame.Id))
				sv.idToClient.Get(string(frame.Id)).SendToHTTP(PROXY_FRAME_TYPE_CONNECT, frame.Data)
				return
			} else {
				conn.origin.Logger.Log(3, "Could not find connection to id: "+string(frame.Id))
				return
			}
		}
		cl := sv.idToClient.Get(string(frame.Id))
		if !cl.udpClient.IsAlive() {
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
				//Send to UDP
				cl.SendToUDP(frame.Data)
			}
		}
	}
}

func (sv *HTTPProxyServerUDP) handleUDPReadFunc(udp *UDPClient, data []byte, ended bool) {
	//Get HTTP client
	if sv.clientToId.Get(udp) == "" || sv.idToClient.Get(sv.clientToId.Get(udp)) == nil {
		//Connection does not exists
		udp.Logger.Log(3, "Connection connected to: "+udp.address.String()+" not found")
		return
	}
	id := sv.clientToId.Get(udp)
	cl := sv.idToClient.Get(id)

	//End other connection
	if ended {
		cl.Close(true)
	}

	//Send to client
	cl.SendToHTTP(PROXY_FRAME_TYPE_DATA, data)
}

/*
Starts HTTP Proxy Server for UDP. Locks execution thread
*/
func (sv *HTTPProxyServerUDP) Start() {
	sv.httpServer.Start()
}

/*
Stops HTTP Proxy Server for UDP
*/
func (sv *HTTPProxyServerUDP) Stop() {
	sv.httpServer.Stop()
}
func (sv *HTTPProxyServerUDP) IsAlive() bool {
	return sv.httpServer.IsAlive()
}
func (sv *HTTPProxyServerUDP) GetAddress() string {
	return sv.httpServer.GetAddress()
}
