package proxy

import (
	webtools "github.com/kolojar/Go-Webtools"
	"github.com/kolojar/Go-Webtools/httptools"
	"github.com/kolojar/Go-Webtools/udp"
)

/*
HTTPProxyClientUDP is client for proxied UDP traffic over httpTools
*/
type HTTPProxyClientUDP struct {
	clientToID         webtools.SafeMap[*udp.ServerConn, string]
	idToClient         webtools.SafeMap[string, *udp.ServerConn]
	udpServer          *udp.Server
	httpClient         *httptools.WebSocketClient
	pendingConnections webtools.SafeMap[string, *udp.ServerConn]
	pendingConnsData   webtools.SafeMap[*udp.ServerConn, [][]byte]
}

/*
IsAlive gets if client is alive
*/
func (cl *HTTPProxyClientUDP) IsAlive() bool {
	return cl.httpClient.IsAlive()
}

/*
NewHTTPProxyClientUDP creates new http Proxy Client for UDP but does not starts it, if you want to use default connection endpoint, add /websocket to end of address
*/
func NewHTTPProxyClientUDP(httpProxyAddress string, tcpServerAddress string, reportTraffic bool) (*HTTPProxyClientUDP, error) {
	cl := &HTTPProxyClientUDP{clientToID: webtools.MakeSafeMap[*udp.ServerConn, string](), pendingConnections: webtools.MakeSafeMap[string, *udp.ServerConn](), idToClient: webtools.MakeSafeMap[string, *udp.ServerConn](), pendingConnsData: webtools.MakeSafeMap[*udp.ServerConn, [][]byte]()}
	var err error
	cl.httpClient, err = httptools.NewWebSocketClient(httpProxyAddress, cl.handleWebTransportReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.httpClient.Logger.Prefix = "HTTPProxyClientUDP - " + cl.httpClient.Logger.Prefix
	cl.udpServer, err = udp.NewServer(tcpServerAddress, cl.handleUDPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.udpServer.Logger.Prefix = "HTTPProxyClientUDP - " + cl.udpServer.Logger.Prefix
	return cl, nil
}

func (cl *HTTPProxyClientUDP) handleWebTransportReadFunc(_ *httptools.WebSocketClient, frame []byte, status webtools.NetworkStatus, isBinary bool) {
	_ = isBinary //Get rid of unneded property
	if status == webtools.DisconnectStatus {
		// Close all connections
		cl.udpServer.Stop()
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
				cl.httpClient.Logger.Log(1, "Prepared new connection with temporary id: "+string(frame.Data)+" for connection connected to: "+conn.Address.String()+" with new id: "+string(frame.ID))

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

func (cl *HTTPProxyClientUDP) handleUDPReadFunc(udpConn *udp.ServerConn, data []byte, ended bool) {
	if cl.pendingConnsData.Get(udpConn) != nil {
		// Already pending connection
		cl.pendingConnsData.Set(udpConn, append(cl.pendingConnsData.Get(udpConn), data))
		return
	}
	id := cl.clientToID.Get(udpConn)
	if id == "" {
		// No connection found, request new
		tempID := webtools.GenerateRandomID()
		cl.pendingConnections.Set(tempID, udpConn)
		cl.httpClient.Logger.Log(1, "Preparing new connection with temporary id: "+tempID+" for connection connected to: "+udpConn.Address.String())
		cl.httpClient.Send(webtools.PackWebtoolsFrame(webtools.FrameTypeConnect, []byte("0"), []byte(tempID)), 2)
		cl.pendingConnsData.Set(udpConn, append(make([][]byte, 0), data))
		return
	}

	if ended {
		// Connection ennded
		cl.httpClient.Send(webtools.PackWebtoolsFrame(webtools.FrameTypeClose, []byte(id), nil), 2)
		return
	}
	// Send data
	cl.httpClient.Send(webtools.PackWebtoolsFrame(webtools.FrameTypeData, []byte(id), data), 2)
}

/*
Connect connects to httpTools Proxy server and start reading loop, does not locks execution thread
*/
func (cl *HTTPProxyClientUDP) Connect() {
	cl.httpClient.Connect()
	go cl.udpServer.Start()
}

/*
Stop stops the client
*/
func (cl *HTTPProxyClientUDP) Stop() {
	cl.httpClient.Stop()
	cl.udpServer.Stop()
}
