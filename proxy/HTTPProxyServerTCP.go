package proxy

import (
	webtools "github.com/kolojar/Go-Webtools"
	"github.com/kolojar/Go-Webtools/helpertools"
	"github.com/kolojar/Go-Webtools/httptools"
	"github.com/kolojar/Go-Webtools/tcp"
)

/*
HTTPProxyServerTCP is server for proxied TCP traffic over HTTP
*/
type HTTPProxyServerTCP struct {
	idToClient       helpertools.SafeMap[string, *HTTPProxyServerTCPConn]
	clientToID       helpertools.SafeMap[*tcp.ClientSimple, string]
	httpServer       *httptools.WebSocketServer
	tcpServerAddress string
	reportTrafic     bool
}

/*
HTTPProxyServerTCPConn is connection object of HTTPProxyServerTCP
*/
type HTTPProxyServerTCPConn struct {
	tcpClient *tcp.ClientSimple
	id        []byte
	source    *httptools.WebSocketServerConn
	origin    *HTTPProxyServerTCP
}

/*
SendToHTTP creates frame and sends it to HTTP
*/
func (cl *HTTPProxyServerTCPConn) SendToHTTP(operation uint8, data []byte) {
	cl.source.Send(PackWebtoolsFrame(operation, cl.id, data))
}

/*
SendToTCP sends data to TCP
*/
func (cl *HTTPProxyServerTCPConn) SendToTCP(data []byte) {
	cl.tcpClient.Send(data)
}

/*
Close closes connection to client
*/
func (cl *HTTPProxyServerTCPConn) Close(isInitiator bool) {
	if cl == nil || cl.tcpClient == nil {
		return
	}
	cl.tcpClient.Stop()
	cl.origin.idToClient.Delete(string(cl.id))
	if isInitiator {
		cl.SendToHTTP(FrameTypeClose, nil)
	}
	cl.origin.clientToID.Delete(cl.tcpClient)
}

/*
NewHTTPProxyServerTCP creates new HTTP Proxy Server for TCP but does not starts it
*/
func NewHTTPProxyServerTCP(httpProxyAddress string, tcpServerAddress string, reportTraffic bool) *HTTPProxyServerTCP {
	sv := &HTTPProxyServerTCP{tcpServerAddress: tcpServerAddress, clientToID: helpertools.MakeSafeMap[*tcp.ClientSimple, string](), idToClient: helpertools.MakeSafeMap[string, *HTTPProxyServerTCPConn](), reportTrafic: reportTraffic}
	sv.httpServer = httptools.NewWebSocketServer(httpProxyAddress, sv.handleWebSocketReadFunc, nil, "", false, false, reportTraffic)
	sv.httpServer.GetLogger().Prefix = "HTTPProxyServerTCP - " + sv.httpServer.GetLogger().Prefix
	return sv
}

func (sv *HTTPProxyServerTCP) handleWebSocketReadFunc(conn *httptools.WebSocketServerConn, frame []byte, status webtools.NetworkStatus, isBinary bool) {
	_ = isBinary //Get rid of unneded property
	if status == webtools.ConnectStatus {
		conn.IsBinary = true
		return
	}
	if status == webtools.DisconnectStatus {
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
	if status != webtools.ReadDataStatus {
		return
	}

	//Unpack frame
	for _, frame := range UnpackWebtoolsFrame(frame, sv.httpServer.GetLogger()) {
		if frame.Operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(frame.ID)) == nil {
			if frame.Operation == FrameTypeConnect {
				//Create new connection
				frame.ID = []byte(helpertools.GenerateRandomID())
				cl, err := tcp.NewClientSimple(sv.tcpServerAddress, -1, false, sv.handleTCPReadFunc, sv.reportTrafic)
				cl.GetLogger().Prefix = "HTTPProxyServerTCP - " + cl.GetLogger().Prefix
				if err != nil {
					conn.Client.Logger.Log(3, "Could not create connection with id: "+string(frame.ID)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.ID), &HTTPProxyServerTCPConn{tcpClient: cl, id: frame.ID, source: conn, origin: sv})
				sv.clientToID.Set(cl, string(frame.ID))
				sv.idToClient.Get(string(frame.ID)).SendToHTTP(FrameTypeConnect, frame.Data)
				return
			}
			conn.Client.Logger.Log(3, "Could not find connection to id: "+string(frame.ID))
			return
		}
		cl := sv.idToClient.Get(string(frame.ID))
		if !cl.tcpClient.IsAlive() {
			conn.Client.Logger.Log(3, "Connection with id: "+string(frame.ID)+" connected to: "+conn.GetConn().RemoteAddr().String()+" connected locally to: "+conn.GetConn().LocalAddr().String()+" closed")
			return
		}

		//Sort operations
		switch frame.Operation {
		case FrameTypeClose:
			{
				//Close connection
				cl.Close(false)
			}
		case FrameTypeData:
			{
				//Send to TCP
				cl.SendToTCP(frame.Data)
			}
		}
	}
}

func (sv *HTTPProxyServerTCP) handleTCPReadFunc(tcp *tcp.ClientSimple, data []byte, status webtools.NetworkStatus) {
	if status == webtools.ConnectStatus {
		return
	}

	//Get HTTP client
	if sv.clientToID.Get(tcp) == "" || sv.idToClient.Get(sv.clientToID.Get(tcp)) == nil {
		//Connection does not exists
		tcp.GetLogger().Log(3, "Connection connected to: "+tcp.GetConn().RemoteAddr().String()+" not found")
		return
	}
	id := sv.clientToID.Get(tcp)
	cl := sv.idToClient.Get(id)

	//End other connection
	if status == webtools.DisconnectStatus {
		cl.Close(true)
	}

	//Send to client
	cl.SendToHTTP(FrameTypeData, data)
}

/*
Start starts HTTP Proxy Server for TCP. Locks execution thread
*/
func (sv *HTTPProxyServerTCP) Start() {
	sv.httpServer.Start()
}

/*
Stop stops HTTP Proxy Server for TCP
*/
func (sv *HTTPProxyServerTCP) Stop() {
	sv.httpServer.Stop()
}

/*
IsAlive gets if server is alive
*/
func (sv *HTTPProxyServerTCP) IsAlive() bool {
	return sv.httpServer.IsAlive()
}

/*
GetAddress gets address of server
*/
func (sv *HTTPProxyServerTCP) GetAddress() string {
	return sv.httpServer.GetAddress()
}
