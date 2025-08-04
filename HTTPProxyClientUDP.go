package webtools

/*
HTTP Proxy client for UDP object
*/
type HTTPProxyClientUDP struct {
	clientToId         map[*UDPServerConn]string
	idToClient         map[string]*UDPServerConn
	udpServer          *UDPServer
	httpClient         *HTTPWebTransportClient
	pendingConnections map[string]*UDPServerConn
	pendingConnsData   map[*UDPServerConn][][]byte
}

func (cl *HTTPProxyClientUDP) IsAlive() bool {
	return cl.httpClient.IsAlive()
}

/*
Creates new HTTP Proxy Client for UDP but does not starts it
*/
func NewHTTPProxyClientUDP(httpProxyAddress string, tcpServerAddress string, reportTraffic bool) (*HTTPProxyClientUDP, error) {
	cl := &HTTPProxyClientUDP{clientToId: map[*UDPServerConn]string{}, pendingConnections: map[string]*UDPServerConn{}, idToClient: map[string]*UDPServerConn{}, pendingConnsData: map[*UDPServerConn][][]byte{}}
	var err error
	cl.httpClient, err = NewHTTPWebTransportClient(httpProxyAddress, cl.handleWebTransportReadFunc, reportTraffic)
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

func (cl *HTTPProxyClientUDP) handleWebTransportReadFunc(_ *HTTPWebTransportClient, frame []byte, ended bool) {
	if ended {
		//Close all connections
		cl.udpServer.Stop()
		return
	}

	//Unpack
	operation, id, data := UnpackHTTPProxyFrame(frame, cl.httpClient.Logger)
	if operation == 0 {
		return
	}

	switch operation {
	case HTTP_PROXY_FRAME_TYPE_CONNECT:
		{
			//Confirmed connection
			conn := cl.pendingConnections[string(data)]
			if conn == nil {
				cl.httpClient.Logger.Log(3, "Pending connection with temporary id: "+string(data)+" not found")
				return
			}
			delete(cl.pendingConnections, string(data))
			cl.clientToId[conn] = string(id)
			cl.idToClient[string(id)] = conn
			cl.httpClient.Logger.Log(1, "Prepared new connection with temporary id: "+string(data)+" for connection connected to: "+conn.Address.String()+" with new id: "+string(id))

			//Process pending data
			for len(cl.pendingConnsData[conn]) > 0 {
				//Resend data
				cl.httpClient.Send(PackHTTPProxyFrame(HTTP_PROXY_FRAME_TYPE_DATA, id, cl.pendingConnsData[conn][0]))
				cl.pendingConnsData[conn] = cl.pendingConnsData[conn][1:]
			}
			delete(cl.pendingConnsData, conn)
			return
		}
	case HTTP_PROXY_FRAME_TYPE_CLOSE:
		{
			//Close connection
			cl.idToClient[string(id)].Close()
		}
	case HTTP_PROXY_FRAME_TYPE_DATA:
		{
			//Resend data
			cl.idToClient[string(id)].Send(data)
		}
	}
}

func (cl *HTTPProxyClientUDP) handleUDPReadFunc(udpConn *UDPServerConn, data []byte, ended bool) {
	if cl.pendingConnsData[udpConn] != nil {
		//Already pending connection
		cl.pendingConnsData[udpConn] = append(cl.pendingConnsData[udpConn], data)
		return
	}

	id := cl.clientToId[udpConn]
	if id == "" {
		//No connection found, request new
		tempId := GenerateRandomId()
		cl.pendingConnections[tempId] = udpConn
		cl.httpClient.Logger.Log(1, "Preparing new connection with temporary id: "+tempId+" for connection connected to: "+udpConn.Address.String())
		cl.httpClient.Send(PackHTTPProxyFrame(HTTP_PROXY_FRAME_TYPE_CONNECT, []byte("0"), []byte(tempId)))
		cl.pendingConnsData[udpConn] = make([][]byte, 0)
		cl.pendingConnsData[udpConn] = append(cl.pendingConnsData[udpConn], data)
		return
	}

	if ended {
		//Connection ennded
		cl.httpClient.Send(PackHTTPProxyFrame(HTTP_PROXY_FRAME_TYPE_CLOSE, []byte(id), nil))
		return
	}
	//Send data
	cl.httpClient.Send(PackHTTPProxyFrame(HTTP_PROXY_FRAME_TYPE_DATA, []byte(id), data))
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
