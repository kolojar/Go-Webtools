package webtools

/*
HTTP Proxy client for TCP object
*/
type HTTPProxyClientTCP struct {
	clientToId         SafeMap[*TCPServerConn, string]
	idToClient         SafeMap[string, *TCPServerConn]
	tcpServer          *TCPServer
	httpClient         *WebSocketClient
	pendingConnections SafeMap[string, *TCPServerConn]
	pendingConnsData   SafeMap[*TCPServerConn, [][]byte]
}

func (cl *HTTPProxyClientTCP) IsAlive() bool {
	return cl.httpClient.IsAlive()
}

/*
Creates new HTTP Proxy Client for TCP but does not starts it, if you want to use default connection endpoint, add /webtransport to end of address
*/
func NewHTTPProxyClientTCP(httpProxyAddress string, tcpServerAddress string, reportTraffic bool) (*HTTPProxyClientTCP, error) {
	cl := &HTTPProxyClientTCP{clientToId: MakeSafeMap[*TCPServerConn, string](), pendingConnections: MakeSafeMap[string, *TCPServerConn](), idToClient: MakeSafeMap[string, *TCPServerConn](), pendingConnsData: MakeSafeMap[*TCPServerConn, [][]byte]()}
	var err error
	cl.httpClient, err = NewWebSocketClient(httpProxyAddress, cl.handleWebTransportReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.httpClient.Logger.Prefix = "HTTPProxyClientTCP - " + cl.httpClient.Logger.Prefix
	cl.tcpServer, err = NewTCPServer(tcpServerAddress, cl.handleTCPReadFunc, reportTraffic, false)
	if err != nil {
		return nil, err
	}
	cl.tcpServer.Logger.Prefix = "HTTPProxyClientTCP - " + cl.tcpServer.Logger.Prefix
	return cl, nil
}

func (cl *HTTPProxyClientTCP) handleWebTransportReadFunc(_ *WebSocketClient, frame []byte, status uint8, isBinary bool) {
	if status == TCP_DISCONNECT_STATUS {
		//Close all connections
		cl.tcpServer.Stop()
		return
	}
	if status != TCP_READ_DATA_STATUS {
		return
	}

	//Unpack
	for _, frame := range UnpackProxyFrame(frame, cl.httpClient.Logger) {
		operation, id, data := frame.A, frame.B, frame.C
		if operation == 0 {
			return
		}

		switch operation {
		case PROXY_FRAME_TYPE_CONNECT:
			{
				//Confirmed connection
				conn := cl.pendingConnections.Get(string(data))
				if conn == nil {
					cl.httpClient.Logger.Log(3, "Pending connection with temporary id: "+string(data)+" not found")
					return
				}
				cl.pendingConnections.Delete(string(data))
				cl.clientToId.Set(conn, string(id))
				cl.idToClient.Set(string(id), conn)
				cl.httpClient.Logger.Log(1, "Prepared new connection with temporary id: "+string(data)+" for connection connected to: "+conn.GetConn().RemoteAddr().String()+" connected locally to: "+conn.GetConn().LocalAddr().String()+" with new id: "+string(id))

				//Process pending data
				for len(cl.pendingConnsData.Get(conn)) > 0 {
					//Resend data
					cl.httpClient.Send(PackProxyFrame(PROXY_FRAME_TYPE_DATA, id, cl.pendingConnsData.Get(conn)[0]), 2)
					cl.pendingConnsData.Set(conn, cl.pendingConnsData.Get(conn)[1:])
				}
				cl.pendingConnsData.Delete(conn)
				return
			}
		case PROXY_FRAME_TYPE_CLOSE:
			{
				//Close connection
				cl.idToClient.Get(string(id)).Close()
			}
		case PROXY_FRAME_TYPE_DATA:
			{
				//Resend data
				cl.idToClient.Get(string(id)).Send(data)
			}
		}
	}
}

func (cl *HTTPProxyClientTCP) handleTCPReadFunc(tcpConn *TCPServerConn, data []byte, status uint8) {
	if status == TCP_CONNECT_STATUS {
		return
	}
	if cl.pendingConnsData.Get(tcpConn) != nil {
		//Already pending connection
		cl.pendingConnsData.Set(tcpConn, append(cl.pendingConnsData.Get(tcpConn), data))
		return
	}

	id := cl.clientToId.Get(tcpConn)
	if id == "" {
		//No connection found, request new
		tempId := GenerateRandomId()
		cl.pendingConnections.Set(tempId, tcpConn)
		cl.httpClient.Logger.Log(1, "Preparing new connection with temporary id: "+tempId+" for connection connected to: "+tcpConn.GetConn().RemoteAddr().String()+" connected locally to: "+tcpConn.GetConn().LocalAddr().String())
		cl.httpClient.Send(PackProxyFrame(PROXY_FRAME_TYPE_CONNECT, []byte("0"), []byte(tempId)), 2)
		cl.pendingConnsData.Set(tcpConn, append(make([][]byte, 0), data))
		return
	}

	if status == TCP_DISCONNECT_STATUS {
		//Connection ennded
		cl.httpClient.Send(PackProxyFrame(PROXY_FRAME_TYPE_CLOSE, []byte(id), nil), 2)
		return
	}
	//Send data
	cl.httpClient.Send(PackProxyFrame(PROXY_FRAME_TYPE_DATA, []byte(id), data), 2)
}

/*
Connects to HTTP Proxy server and start reading loop, does not locks execution thread
*/
func (cl *HTTPProxyClientTCP) Connect() {
	go cl.tcpServer.Start()
	cl.httpClient.Connect()
}

/*
Connects to HTTP Proxy server and start reading loop, does not locks execution thread
*/
func (cl *HTTPProxyClientTCP) Stop() {
	cl.httpClient.Stop()
	cl.tcpServer.Stop()
}
