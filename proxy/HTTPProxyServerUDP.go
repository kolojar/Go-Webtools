package proxy

import (
	"net"
	"webtools"
	httptools "webtools/httpTools"
	udptools "webtools/udpTools"
)

/*
HTTPProxyServerUDP is server for proxied UDP traffic over HTTP
*/
type HTTPProxyServerUDP struct {
	idToClient       webtools.SafeMap[string, *HTTPProxyServerUDPConn]
	clientToID       webtools.SafeMap[*udptools.UDPClient, string]
	httpServer       *httptools.WebSocketServer
	udpServerAddress string
	reportTrafic     bool
}

/*
HTTPProxyServerUDPConn is connection object of HTTPProxyServerUDP
*/
type HTTPProxyServerUDPConn struct {
	udpClient *udptools.UDPClient
	id        []byte
	source    *httptools.WebSocketServerConn
	origin    *HTTPProxyServerUDP
}

/*
SendToHTTP creates frame and sends it to HTTP
*/
func (cl *HTTPProxyServerUDPConn) SendToHTTP(operation uint8, data []byte) {
	cl.source.Send(webtools.PackWebtoolsFrame(operation, cl.id, data))
}

/*
SendToUDP sends data to UDP
*/
func (cl *HTTPProxyServerUDPConn) SendToUDP(data []byte) {
	cl.udpClient.Send(data)
}

/*
Close closes connection to client
*/
func (cl *HTTPProxyServerUDPConn) Close(isInitiator bool) {
	cl.udpClient.Stop()
	cl.origin.idToClient.Delete(string(cl.id))
	if isInitiator {
		cl.SendToHTTP(webtools.WEBTOOLS_FRAME_TYPE_CLOSE, nil)
	}
	cl.origin.clientToID.Delete(cl.udpClient)
}

/*
NewHTTPProxyServerUDP creates new HTTP Proxy Server for UDP but does not starts it
*/
func NewHTTPProxyServerUDP(httpProxyAddress string, udpServerAddress string, reportTraffic bool) *HTTPProxyServerUDP {
	sv := &HTTPProxyServerUDP{udpServerAddress: udpServerAddress, clientToID: webtools.MakeSafeMap[*udptools.UDPClient, string](), idToClient: webtools.MakeSafeMap[string, *HTTPProxyServerUDPConn](), reportTrafic: reportTraffic}
	sv.httpServer = httptools.NewHTTPWebSocketServer(httpProxyAddress, sv.handleWebSocketReadFunc, nil, "", reportTraffic)
	sv.httpServer.Logger.Prefix = "HTTPProxyServerUDP - " + sv.httpServer.Logger.Prefix
	return sv
}

func (sv *HTTPProxyServerUDP) handleWebSocketReadFunc(conn *httptools.WebSocketServerConn, frame []byte, status uint8, isBinary bool) {
	_ = isBinary //Get rid of unneded property
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
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, conn.Client.Logger) {
		if frame.Operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(frame.Id)) == nil {
			if frame.Operation == webtools.WEBTOOLS_FRAME_TYPE_CONNECT {
				//Create new connection
				frame.Id = []byte(webtools.GenerateRandomId())
				cl, err := udptools.NewUDPClient(sv.udpServerAddress, sv.handleUDPReadFunc, sv.reportTrafic)
				cl.Logger.Prefix = "HTTPProxyServerUDP - " + cl.Logger.Prefix
				if err != nil {
					conn.Client.Logger.Log(3, "Could not create connection with id: "+string(frame.Id)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.Id), &HTTPProxyServerUDPConn{udpClient: cl, id: frame.Id, source: conn, origin: sv})
				sv.clientToID.Set(cl, string(frame.Id))
				sv.idToClient.Get(string(frame.Id)).SendToHTTP(webtools.WEBTOOLS_FRAME_TYPE_CONNECT, frame.Data)
				return
			}
			conn.Client.Logger.Log(3, "Could not find connection to id: "+string(frame.Id))
			return
		}
		cl := sv.idToClient.Get(string(frame.Id))
		if !cl.udpClient.IsAlive() {
			conn.Client.Logger.Log(3, "Connection with id: "+string(frame.Id)+" connected to: "+conn.GetConn().RemoteAddr().String()+" connected locally to: "+conn.GetConn().LocalAddr().String()+" closed")
			return
		}

		//Sort operations
		switch frame.Operation {
		case webtools.WEBTOOLS_FRAME_TYPE_CLOSE:
			{
				//Close connection
				cl.Close(false)
			}
		case webtools.WEBTOOLS_FRAME_TYPE_DATA:
			{
				//Send to UDP
				cl.SendToUDP(frame.Data)
			}
		}
	}
}

func (sv *HTTPProxyServerUDP) handleUDPReadFunc(udp *udptools.UDPClient, _ *net.UDPAddr, data []byte, ended bool) {
	//Get HTTP client
	if sv.clientToID.Get(udp) == "" || sv.idToClient.Get(sv.clientToID.Get(udp)) == nil {
		//Connection does not exists
		udp.Logger.Log(3, "Connection connected to: "+udp.Conn.RemoteAddr().String()+" not found")
		return
	}
	id := sv.clientToID.Get(udp)
	cl := sv.idToClient.Get(id)

	//End other connection
	if ended {
		cl.Close(true)
	}

	//Send to client
	cl.SendToHTTP(webtools.WEBTOOLS_FRAME_TYPE_DATA, data)
}

/*
Start starts HTTP Proxy Server for UDP. Locks execution thread
*/
func (sv *HTTPProxyServerUDP) Start() {
	sv.httpServer.Start()
}

/*
Stop stops HTTP Proxy Server for UDP
*/
func (sv *HTTPProxyServerUDP) Stop() {
	sv.httpServer.Stop()
}

/*
IsAlive gets if server is alive
*/
func (sv *HTTPProxyServerUDP) IsAlive() bool {
	return sv.httpServer.IsAlive()
}

/*
GetAddress gets address of server
*/
func (sv *HTTPProxyServerUDP) GetAddress() string {
	return sv.httpServer.GetAddress()
}
