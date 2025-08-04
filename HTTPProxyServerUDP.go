package webtools

import (
	"maps"
)

/*
HTTP Proxy server for UDP object
*/
type HTTPProxyServerUDP struct {
	idToClient       map[string]*HTTPProxyServerUDPConn
	clientToId       map[*UDPClient]string
	httpServer       *HTTPWebTransportServer
	udpServerAddress string
	reportTrafic     bool
}

/*
HTTP Proxy server for UDP connection object
*/
type HTTPProxyServerUDPConn struct {
	udpClient *UDPClient
	id        []byte
	source    *HTTPWebTransportServerConn
	origin    *HTTPProxyServerUDP
}

/*
Creates frame and sends it to HTTP
*/
func (cl *HTTPProxyServerUDPConn) SendToHTTP(operation uint8, data []byte) {
	cl.source.Send(PackHTTPProxyFrame(operation, cl.id, data))
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
	delete(cl.origin.idToClient, string(cl.id))
	if isInitiator {
		cl.SendToHTTP(HTTP_PROXY_FRAME_TYPE_CLOSE, nil)
	}
	delete(cl.origin.clientToId, cl.udpClient)
}

/*
Creates new HTTP Proxy Server for UDP but does not starts it
*/
func NewHTTPProxyServerUDP(httpProxyAddress string, udpServerAddress string, reportTraffic bool) *HTTPProxyServerUDP {
	sv := &HTTPProxyServerUDP{udpServerAddress: udpServerAddress, clientToId: map[*UDPClient]string{}, idToClient: map[string]*HTTPProxyServerUDPConn{}, reportTrafic: reportTraffic}
	sv.httpServer = NewHTTPWebTransportServer(httpProxyAddress, sv.handleWebTransportReadFunc, reportTraffic)
	sv.httpServer.Logger.Prefix = "HTTPProxyServerUDP - " + sv.httpServer.Logger.Prefix
	return sv
}

func (sv *HTTPProxyServerUDP) handleWebTransportReadFunc(conn *HTTPWebTransportServerConn, frame []byte, ended bool) {
	if ended {
		//Close all connections with this HTTP WebTransport Conn
		var cp map[string]*HTTPProxyServerUDPConn = map[string]*HTTPProxyServerUDPConn{}
		maps.Copy(cp, sv.idToClient)
		for _, v := range cp {
			if v == nil {
				continue
			}
			if v.source == conn {
				v.Close(true)
			}
		}
		return
	}

	//Unpack frame
	operation, id, data := UnpackHTTPProxyFrame(frame, conn.origin.Logger)
	if operation == 0 {
		return
	}

	//Sort connections
	if sv.idToClient[string(id)] == nil {
		if operation == HTTP_PROXY_FRAME_TYPE_CONNECT {
			//Create new connection
			id = []byte(GenerateRandomId())
			cl, err := NewUDPClient(sv.udpServerAddress, sv.handleUDPReadFunc, sv.reportTrafic)
			cl.Logger.Prefix = "HTTPProxyServerUDP - " + cl.Logger.Prefix
			if err != nil {
				conn.origin.Logger.Log(3, "Could not create connection with id: "+string(id)+" to server.")
				return
			}
			cl.Connect()
			sv.idToClient[string(id)] = &HTTPProxyServerUDPConn{udpClient: cl, id: id, source: conn, origin: sv}
			sv.clientToId[cl] = string(id)
			sv.idToClient[string(id)].SendToHTTP(HTTP_PROXY_FRAME_TYPE_CONNECT, data)
			return
		} else {
			conn.origin.Logger.Log(3, "Could not find connection to id: "+string(id))
			return
		}
	}
	cl := sv.idToClient[string(id)]
	if !cl.udpClient.IsAlive() {
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
			//Send to UDP
			cl.SendToUDP(data)
		}
	}
}

func (sv *HTTPProxyServerUDP) handleUDPReadFunc(udp *UDPClient, data []byte, ended bool) {
	//Get HTTP client
	if sv.clientToId[udp] == "" || sv.idToClient[sv.clientToId[udp]] == nil {
		//Connection does not exists
		udp.Logger.Log(3, "Connection connected to: "+udp.address.String()+" not found")
		return
	}
	id := sv.clientToId[udp]
	cl := sv.idToClient[id]

	//End other connection
	if ended {
		cl.Close(true)
	}

	//Send to client
	cl.SendToHTTP(HTTP_PROXY_FRAME_TYPE_DATA, data)
}

/*
Starts HTTP Proxy Server for UDP
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
