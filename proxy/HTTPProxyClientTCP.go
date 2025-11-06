/*
Package proxy provides tools for proxying lots of kinds of traffic using other protocols.
*/
package proxy

import (
	"webtools"
	"webtools/httptools"
	"webtools/tcp"
)

/*
HTTPProxyClientTCP is client for proxied TCP traffic over HTTP
*/
type HTTPProxyClientTCP struct {
	clientToID         webtools.SafeMap[*tcp.ServerConn, string]
	idToClient         webtools.SafeMap[string, *tcp.ServerConn]
	tcpServer          *tcp.Server
	httpClient         *httptools.WebSocketClient
	pendingConnections webtools.SafeMap[string, *tcp.ServerConn]
	pendingConnsData   webtools.SafeMap[*tcp.ServerConn, [][]byte]
}

/*
IsAlive gets if client is alive
*/
func (cl *HTTPProxyClientTCP) IsAlive() bool {
	return cl.httpClient.IsAlive()
}

/*
NewHTTPProxyClientTCP creates new HTTP Proxy Client for TCP but does not starts it, if you want to use default connection endpoint, add /websocket to end of address
*/
func NewHTTPProxyClientTCP(httpProxyAddress string, tcpServerAddress string, reportTraffic bool) (*HTTPProxyClientTCP, error) {
	cl := &HTTPProxyClientTCP{clientToID: webtools.MakeSafeMap[*tcp.ServerConn, string](), pendingConnections: webtools.MakeSafeMap[string, *tcp.ServerConn](), idToClient: webtools.MakeSafeMap[string, *tcp.ServerConn](), pendingConnsData: webtools.MakeSafeMap[*tcp.ServerConn, [][]byte]()}
	var err error
	cl.httpClient, err = httptools.NewWebSocketClient(httpProxyAddress, cl.handleWebTransportReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.httpClient.Logger.Prefix = "HTTPProxyClientTCP - " + cl.httpClient.Logger.Prefix
	cl.tcpServer, err = tcp.NewServer(tcpServerAddress, cl.handleTCPReadFunc, reportTraffic, false)
	if err != nil {
		return nil, err
	}
	cl.tcpServer.Logger.Prefix = "HTTPProxyClientTCP - " + cl.tcpServer.Logger.Prefix
	return cl, nil
}

func (cl *HTTPProxyClientTCP) handleWebTransportReadFunc(_ *httptools.WebSocketClient, frame []byte, status uint8, isBinary bool) {
	_ = isBinary //Get rid of unneded property
	if status == webtools.DisconnectStatus {
		// Close all connections
		cl.tcpServer.Stop()
		return
	}
	if status != webtools.ReadDataStatus {
		return
	}

	// Unpack
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, cl.httpClient.Logger) {
		if frame.Operation == 0 {
			return
		}

		switch frame.Operation {
		case webtools.FrameTypeConnect:
			{
				// Confirmed connection
				conn := cl.pendingConnections.Get(string(frame.Data))
				if conn == nil {
					cl.httpClient.Logger.Log(3, "Pending connection with temporary id: "+string(frame.Data)+" not found")
					return
				}
				cl.pendingConnections.Delete(string(frame.Data))
				cl.clientToID.Set(conn, string(frame.ID))
				cl.idToClient.Set(string(frame.ID), conn)
				cl.httpClient.Logger.Log(1, "Prepared new connection with temporary id: "+string(frame.Data)+" for connection connected to: "+conn.GetConn().RemoteAddr().String()+" connected locally to: "+conn.GetConn().LocalAddr().String()+" with new id: "+string(frame.ID))

				// Process pending data
				for len(cl.pendingConnsData.Get(conn)) > 0 {
					// Resend data
					cl.httpClient.Send(webtools.PackWebtoolsFrame(webtools.FrameTypeData, frame.ID, cl.pendingConnsData.Get(conn)[0]), 2)
					cl.pendingConnsData.Set(conn, cl.pendingConnsData.Get(conn)[1:])
				}
				cl.pendingConnsData.Delete(conn)
				return
			}
		case webtools.FrameTypeClose:
			{
				// Close connection
				cl.idToClient.Get(string(frame.ID)).Close()
			}
		case webtools.FrameTypeData:
			{
				// Resend data
				cl.idToClient.Get(string(frame.ID)).Send(frame.Data)
			}
		}
	}
}

func (cl *HTTPProxyClientTCP) handleTCPReadFunc(tcpConn *tcp.ServerConn, data []byte, status uint8) {
	if status == webtools.ConnectStatus {
		return
	}
	if cl.pendingConnsData.Get(tcpConn) != nil {
		// Already pending connection
		cl.pendingConnsData.Set(tcpConn, append(cl.pendingConnsData.Get(tcpConn), data))
		return
	}

	id := cl.clientToID.Get(tcpConn)
	if id == "" {
		// No connection found, request new
		tempID := webtools.GenerateRandomID()
		cl.pendingConnections.Set(tempID, tcpConn)
		cl.httpClient.Logger.Log(1, "Preparing new connection with temporary id: "+tempID+" for connection connected to: "+tcpConn.GetConn().RemoteAddr().String()+" connected locally to: "+tcpConn.GetConn().LocalAddr().String())
		cl.httpClient.Send(webtools.PackWebtoolsFrame(webtools.FrameTypeConnect, []byte("0"), []byte(tempID)), 2)
		cl.pendingConnsData.Set(tcpConn, append(make([][]byte, 0), data))
		return
	}

	if status == webtools.DisconnectStatus {
		// Connection ennded
		cl.httpClient.Send(webtools.PackWebtoolsFrame(webtools.FrameTypeClose, []byte(id), nil), 2)
		return
	}
	// Send data
	cl.httpClient.Send(webtools.PackWebtoolsFrame(webtools.FrameTypeData, []byte(id), data), 2)
}

/*
Connect connects to HTTP Proxy server and start reading loop, does not locks execution thread
*/
func (cl *HTTPProxyClientTCP) Connect() {
	go cl.tcpServer.Start()
	cl.httpClient.Connect()
}

/*
Stop stops the client
*/
func (cl *HTTPProxyClientTCP) Stop() {
	cl.httpClient.Stop()
	cl.tcpServer.Stop()
}
