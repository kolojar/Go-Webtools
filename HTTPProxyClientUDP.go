package webtools

/*
HTTP Proxy client for UDP object
*/
type HTTPProxyClientUDP struct {
	clientToId         SafeMap[*UDPServerConn, string]
	idToClient         SafeMap[string, *UDPServerConn]
	udpServer          *UDPServer
	httpClient         *HTTPWebSocketClient
	pendingConnections SafeMap[string, *UDPServerConn]
	pendingConnsData   SafeMap[*UDPServerConn, [][]byte]
}

func (cl *HTTPProxyClientUDP) IsAlive() bool {
	return cl.httpClient.IsAlive()
}

/*
Creates new HTTP Proxy Client for UDP but does not starts it, if you want to use default connection endpoint, add /webtransport to end of address
*/
func NewHTTPProxyClientUDP(httpProxyAddress string, tcpServerAddress string, reportTraffic bool) (*HTTPProxyClientUDP, error) {
	cl := &HTTPProxyClientUDP{clientToId: MakeSafeMap[*UDPServerConn, string](), pendingConnections: MakeSafeMap[string, *UDPServerConn](), idToClient: MakeSafeMap[string, *UDPServerConn](), pendingConnsData: MakeSafeMap[*UDPServerConn, [][]byte]()}
	var err error
	cl.httpClient, err = NewHTTPWebSocketClient(httpProxyAddress, cl.handleWebTransportReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.httpClient.Logger.Prefix = "HTTPProxyClientUDP - " + cl.httpClient.Logger.Prefix
	cl.udpServer, err = NewUDPServer(tcpServerAddress, cl.handleUDPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.udpServer.Logger.Prefix = "HTTPProxyClientUDP - " + cl.udpServer.Logger.Prefix
	return cl, nil
}

func (cl *HTTPProxyClientUDP) handleWebTransportReadFunc(client *HTTPWebSocketClient, frame []byte, status uint8, isBinary bool) {
	if status == TCP_DISCONNECT_STATUS {
		//Close all connections
		cl.udpServer.Stop()
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
				cl.httpClient.Logger.Log(1, "Prepared new connection with temporary id: "+string(data)+" for connection connected to: "+conn.Address.String()+" with new id: "+string(id))

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

func (cl *HTTPProxyClientUDP) handleUDPReadFunc(udpConn *UDPServerConn, data []byte, ended bool) {
	if cl.pendingConnsData.Get(udpConn) != nil {
		//Already pending connection
		cl.pendingConnsData.Set(udpConn, append(cl.pendingConnsData.Get(udpConn), data))
		return
	}

	id := cl.clientToId.Get(udpConn)
	if id == "" {
		//No connection found, request new
		tempId := GenerateRandomId()
		cl.pendingConnections.Set(tempId, udpConn)
		cl.httpClient.Logger.Log(1, "Preparing new connection with temporary id: "+tempId+" for connection connected to: "+udpConn.Address.String())
		cl.httpClient.Send(PackProxyFrame(PROXY_FRAME_TYPE_CONNECT, []byte("0"), []byte(tempId)), 2)
		cl.pendingConnsData.Set(udpConn, append(make([][]byte, 0), data))
		return
	}

	if ended {
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
