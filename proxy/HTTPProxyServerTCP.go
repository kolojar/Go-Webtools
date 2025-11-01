package proxy

import (
	"webtools"
	httptools "webtools/httpTools"
	tcptools "webtools/tcpTools"
)

/*
HTTPProxyServerTCP is server for proxied TCP traffic over HTTP
*/
type HTTPProxyServerTCP struct {
	idToClient       webtools.SafeMap[string, *HTTPProxyServerTCPConn]
	clientToID       webtools.SafeMap[*tcptools.TCPClientSimple, string]
	httpServer       *httptools.WebSocketServer
	tcpServerAddress string
	reportTrafic     bool
}

/*
HTTPProxyServerTCPConn is connection object of HTTPProxyServerTCP
*/
type HTTPProxyServerTCPConn struct {
	tcpClient *tcptools.TCPClientSimple
	id        []byte
	source    *httptools.WebSocketServerConn
	origin    *HTTPProxyServerTCP
}

/*
SendToHTTP creates frame and sends it to HTTP
*/
func (cl *HTTPProxyServerTCPConn) SendToHTTP(operation uint8, data []byte) {
	cl.source.Send(webtools.PackWebtoolsFrame(operation, cl.id, data))
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
		cl.SendToHTTP(webtools.WEBTOOLS_FRAME_TYPE_CLOSE, nil)
	}
	cl.origin.clientToID.Delete(cl.tcpClient)
}

/*
NewHTTPProxyServerTCP creates new HTTP Proxy Server for TCP but does not starts it
*/
func NewHTTPProxyServerTCP(httpProxyAddress string, tcpServerAddress string, reportTraffic bool) *HTTPProxyServerTCP {
	sv := &HTTPProxyServerTCP{tcpServerAddress: tcpServerAddress, clientToID: webtools.MakeSafeMap[*tcptools.TCPClientSimple, string](), idToClient: webtools.MakeSafeMap[string, *HTTPProxyServerTCPConn](), reportTrafic: reportTraffic}
	sv.httpServer = httptools.NewHTTPWebSocketServer(httpProxyAddress, sv.handleWebSocketReadFunc, nil, "", reportTraffic)
	sv.httpServer.Logger.Prefix = "HTTPProxyServerTCP - " + sv.httpServer.Logger.Prefix
	return sv
}

func (sv *HTTPProxyServerTCP) handleWebSocketReadFunc(conn *httptools.WebSocketServerConn, frame []byte, status uint8, isBinary bool) {
	_ = isBinary //Get rid of unneded property
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
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, sv.httpServer.Logger) {
		if frame.Operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(frame.Id)) == nil {
			if frame.Operation == webtools.WEBTOOLS_FRAME_TYPE_CONNECT {
				//Create new connection
				frame.Id = []byte(webtools.GenerateRandomId())
				cl, err := tcptools.NewTCPClientSimple(sv.tcpServerAddress, -1, false, sv.handleTCPReadFunc, sv.reportTrafic)
				cl.GetLogger().Prefix = "HTTPProxyServerTCP - " + cl.GetLogger().Prefix
				if err != nil {
					conn.Client.Logger.Log(3, "Could not create connection with id: "+string(frame.Id)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.Id), &HTTPProxyServerTCPConn{tcpClient: cl, id: frame.Id, source: conn, origin: sv})
				sv.clientToID.Set(cl, string(frame.Id))
				sv.idToClient.Get(string(frame.Id)).SendToHTTP(webtools.WEBTOOLS_FRAME_TYPE_CONNECT, frame.Data)
				return
			}
			conn.Client.Logger.Log(3, "Could not find connection to id: "+string(frame.Id))
			return
		}
		cl := sv.idToClient.Get(string(frame.Id))
		if !cl.tcpClient.IsAlive() {
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
	if sv.clientToID.Get(tcp) == "" || sv.idToClient.Get(sv.clientToID.Get(tcp)) == nil {
		//Connection does not exists
		tcp.GetLogger().Log(3, "Connection connected to: "+tcp.GetConn().RemoteAddr().String()+" not found")
		return
	}
	id := sv.clientToID.Get(tcp)
	cl := sv.idToClient.Get(id)

	//End other connection
	if status == webtools.TCP_DISCONNECT_STATUS {
		cl.Close(true)
	}

	//Send to client
	cl.SendToHTTP(webtools.WEBTOOLS_FRAME_TYPE_DATA, data)
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
