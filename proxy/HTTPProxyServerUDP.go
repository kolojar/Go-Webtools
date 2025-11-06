package proxy

import (
	"net"
	"webtools"
	"webtools/httptools"
	"webtools/udp"
)

/*
HTTPProxyServerUDP is server for proxied UDP traffic over HTTP
*/
type HTTPProxyServerUDP struct {
	idToClient       webtools.SafeMap[string, *HTTPProxyServerUDPConn]
	clientToID       webtools.SafeMap[*udp.Client, string]
	httpServer       *httptools.WebSocketServer
	udpServerAddress string
	reportTrafic     bool
}

/*
HTTPProxyServerUDPConn is connection object of HTTPProxyServerUDP
*/
type HTTPProxyServerUDPConn struct {
	udpClient *udp.Client
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
		cl.SendToHTTP(webtools.FrameTypeClose, nil)
	}
	cl.origin.clientToID.Delete(cl.udpClient)
}

/*
NewHTTPProxyServerUDP creates new HTTP Proxy Server for UDP but does not starts it
*/
func NewHTTPProxyServerUDP(httpProxyAddress string, udpServerAddress string, reportTraffic bool) *HTTPProxyServerUDP {
	sv := &HTTPProxyServerUDP{udpServerAddress: udpServerAddress, clientToID: webtools.MakeSafeMap[*udp.Client, string](), idToClient: webtools.MakeSafeMap[string, *HTTPProxyServerUDPConn](), reportTrafic: reportTraffic}
	sv.httpServer = httptools.NewWebSocketServer(httpProxyAddress, sv.handleWebSocketReadFunc, nil, "", reportTraffic)
	sv.httpServer.GetLogger().Prefix = "HTTPProxyServerUDP - " + sv.httpServer.GetLogger().Prefix
	return sv
}

func (sv *HTTPProxyServerUDP) handleWebSocketReadFunc(conn *httptools.WebSocketServerConn, frame []byte, status uint8, isBinary bool) {
	_ = isBinary //Get rid of unneded property
	if status == webtools.ConnectStatus {
		conn.IsBinary = true
		return
	}
	if status == webtools.DisconnectStatus {
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
	if status != webtools.ReadDataStatus {
		return
	}

	//Unpack frame
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, conn.Client.Logger) {
		if frame.Operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(frame.ID)) == nil {
			if frame.Operation == webtools.FrameTypeConnect {
				//Create new connection
				frame.ID = []byte(webtools.GenerateRandomID())
				cl, err := udp.NewClient(sv.udpServerAddress, sv.handleUDPReadFunc, sv.reportTrafic)
				cl.Logger.Prefix = "HTTPProxyServerUDP - " + cl.Logger.Prefix
				if err != nil {
					conn.Client.Logger.Log(3, "Could not create connection with id: "+string(frame.ID)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.ID), &HTTPProxyServerUDPConn{udpClient: cl, id: frame.ID, source: conn, origin: sv})
				sv.clientToID.Set(cl, string(frame.ID))
				sv.idToClient.Get(string(frame.ID)).SendToHTTP(webtools.FrameTypeConnect, frame.Data)
				return
			}
			conn.Client.Logger.Log(3, "Could not find connection to id: "+string(frame.ID))
			return
		}
		cl := sv.idToClient.Get(string(frame.ID))
		if !cl.udpClient.IsAlive() {
			conn.Client.Logger.Log(3, "Connection with id: "+string(frame.ID)+" connected to: "+conn.GetConn().RemoteAddr().String()+" connected locally to: "+conn.GetConn().LocalAddr().String()+" closed")
			return
		}

		//Sort operations
		switch frame.Operation {
		case webtools.FrameTypeClose:
			{
				//Close connection
				cl.Close(false)
			}
		case webtools.FrameTypeData:
			{
				//Send to UDP
				cl.SendToUDP(frame.Data)
			}
		}
	}
}

func (sv *HTTPProxyServerUDP) handleUDPReadFunc(udp *udp.Client, _ *net.UDPAddr, data []byte, ended bool) {
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
	cl.SendToHTTP(webtools.FrameTypeData, data)
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
