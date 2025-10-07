package proxyTools

import (
	"webtools"
	httptools "webtools/httpTools"
	udptools "webtools/udpTools"
)

/*
HTTP Proxy client for UDP object
*/
type HTTPProxyClientUDP struct {
	clientToId         webtools.SafeMap[*udptools.UDPServerConn, string]
	idToClient         webtools.SafeMap[string, *udptools.UDPServerConn]
	udpServer          *udptools.UDPServer
	httpClient         *httptools.WebSocketClient
	pendingConnections webtools.SafeMap[string, *udptools.UDPServerConn]
	pendingConnsData   webtools.SafeMap[*udptools.UDPServerConn, [][]byte]
}

func (cl *HTTPProxyClientUDP) IsAlive() bool {
	return cl.httpClient.IsAlive()
}

/*
Creates new HTTP Proxy Client for UDP but does not starts it, if you want to use default connection endpoint, add /webtransport to end of address
*/
func NewHTTPProxyClientUDP(httpProxyAddress string, tcpServerAddress string, reportTraffic bool) (*HTTPProxyClientUDP, error) {
	cl := &HTTPProxyClientUDP{clientToId: webtools.MakeSafeMap[*udptools.UDPServerConn, string](), pendingConnections: webtools.MakeSafeMap[string, *udptools.UDPServerConn](), idToClient: webtools.MakeSafeMap[string, *udptools.UDPServerConn](), pendingConnsData: webtools.MakeSafeMap[*udptools.UDPServerConn, [][]byte]()}
	var err error
	cl.httpClient, err = httptools.NewWebSocketClient(httpProxyAddress, cl.handleWebTransportReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.httpClient.Logger.Prefix = "HTTPProxyClientUDP - " + cl.httpClient.Logger.Prefix
	cl.udpServer, err = udptools.NewUDPServer(tcpServerAddress, cl.handleUDPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.udpServer.Logger.Prefix = "HTTPProxyClientUDP - " + cl.udpServer.Logger.Prefix
	return cl, nil
}

func (cl *HTTPProxyClientUDP) handleWebTransportReadFunc(client *httptools.WebSocketClient, frame []byte, status uint8, isBinary bool) {
	if status == webtools.TCP_DISCONNECT_STATUS {
		// Close all connections
		cl.udpServer.Stop()
		return
	}
	if status != webtools.TCP_READ_DATA_STATUS {
		return
	}

	// Unpack
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, cl.httpClient.Logger) {
		if frame.Operation == 0 {
			return
		}

		switch frame.Operation {
		case webtools.WEBTOOLS_FRAME_TYPE_CONNECT:
			{
				// Confirmed connection
				conn := cl.pendingConnections.Get(string(frame.Data))
				if conn == nil {
					cl.httpClient.Logger.Log(3, "Pending connection with temporary id: "+string(frame.Data)+" not found")
					return
				}
				cl.pendingConnections.Delete(string(frame.Data))
				cl.clientToId.Set(conn, string(frame.Id))
				cl.idToClient.Set(string(frame.Id), conn)
				cl.httpClient.Logger.Log(1, "Prepared new connection with temporary id: "+string(frame.Data)+" for connection connected to: "+conn.Address.String()+" with new id: "+string(frame.Id))

				// Process pending data
				for len(cl.pendingConnsData.Get(conn)) > 0 {
					// Resend data
					cl.httpClient.Send(webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_DATA, frame.Id, cl.pendingConnsData.Get(conn)[0]), 2)
					cl.pendingConnsData.Set(conn, cl.pendingConnsData.Get(conn)[1:])
				}
				cl.pendingConnsData.Delete(conn)
				return
			}
		case webtools.WEBTOOLS_FRAME_TYPE_CLOSE:
			{
				// Close connection
				cl.idToClient.Get(string(frame.Id)).Close()
			}
		case webtools.WEBTOOLS_FRAME_TYPE_DATA:
			{
				// Resend data
				cl.idToClient.Get(string(frame.Id)).Send(frame.Data)
			}
		}
	}
}

func (cl *HTTPProxyClientUDP) handleUDPReadFunc(udpConn *udptools.UDPServerConn, data []byte, ended bool) {
	if cl.pendingConnsData.Get(udpConn) != nil {
		// Already pending connection
		cl.pendingConnsData.Set(udpConn, append(cl.pendingConnsData.Get(udpConn), data))
		return
	}
	id := cl.clientToId.Get(udpConn)
	if id == "" {
		// No connection found, request new
		tempId := webtools.GenerateRandomId()
		cl.pendingConnections.Set(tempId, udpConn)
		cl.httpClient.Logger.Log(1, "Preparing new connection with temporary id: "+tempId+" for connection connected to: "+udpConn.Address.String())
		cl.httpClient.Send(webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_CONNECT, []byte("0"), []byte(tempId)), 2)
		cl.pendingConnsData.Set(udpConn, append(make([][]byte, 0), data))
		return
	}

	if ended {
		// Connection ennded
		cl.httpClient.Send(webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_CLOSE, []byte(id), nil), 2)
		return
	}
	// Send data
	cl.httpClient.Send(webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_DATA, []byte(id), data), 2)
}

/*
Connects to HTTP Proxy server and start reading loop, does not locks execution thread
*/
func (cl *HTTPProxyClientUDP) Connect() {
	cl.httpClient.Connect()
	go cl.udpServer.Start()
}

/*
Connects to HTTP Proxy server and start reading loop, does not locks execution thread
*/
func (cl *HTTPProxyClientUDP) Stop() {
	cl.httpClient.Stop()
	cl.udpServer.Stop()
}
