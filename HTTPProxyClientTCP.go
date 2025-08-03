package webtools

/*
HTTP Proxy client for TCP object
*/
type HTTPProxyClientTCP struct {
	clientToId         map[*TCPServerConn]string
	idToClient         map[string]*TCPServerConn
	tcpServer          *TCPServer
	httpClient         *HTTPWebTransportClient
	pendingConnections map[string]*TCPServerConn
}

/*
Creates new HTTP Proxy Client for TCP but does not starts it
*/
func NewHTTPProxyClientTCP(httpProxyAddress string, tcpServerAddress string, reportTraffic bool) (*HTTPProxyClientTCP, error) {
	cl := &HTTPProxyClientTCP{clientToId: map[*TCPServerConn]string{}}
	var err error
	cl.httpClient, err = NewHTTPWebTransportClient(httpProxyAddress, cl.handleWebTransportReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.httpClient.Logger.Prefix = "HTTPProxyClientTCP - " + cl.httpClient.Logger.Prefix
	cl.tcpServer, err = NewTCPServer(tcpServerAddress, cl.handleTCPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.tcpServer.Logger.Prefix = "HTTPProxyClientTCP - " + cl.tcpServer.Logger.Prefix
	return cl, nil
}

func (cl *HTTPProxyClientTCP) handleWebTransportReadFunc(_ *HTTPWebTransportClient, frame []byte, ended bool) {
	if ended {
		//Close all connections
		cl.tcpServer.Stop()
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

func (cl *HTTPProxyClientTCP) handleTCPReadFunc(tcpConn *TCPServerConn, data []byte, ended bool) {
	id := cl.clientToId[tcpConn]
	if id == "" {
		//No connection found, request new
		tempId := GenerateRandomId()
		cl.pendingConnections[tempId] = tcpConn
		cl.httpClient.Logger.Log(1, "Preparing new connection with temporary id: "+tempId+" for connection connected to: "+tcpConn.Conn.RemoteAddr().String()+" connected locally to: "+tcpConn.Conn.LocalAddr().String())
		cl.httpClient.Send(PackHTTPProxyFrame(HTTP_PROXY_FRAME_TYPE_CONNECT, []byte("0"), []byte(tempId)))
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
