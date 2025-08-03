package webtools

import "sync"

/*
HTTP Proxy client for TCP object
*/
type HTTPProxyClientTCP struct {
	//clientToId         map[*TCPServerConn]string
	//idToClient         map[string]*TCPServerConn
	//pendingConnections map[string]*TCPServerConn
	//pendingConnsData   map[*TCPServerConn][][]byte
	clientToId         sync.Map
	idToClient         sync.Map
	pendingConnections sync.Map
	pendingConnsData   sync.Map
	tcpServer          *TCPServer
	httpClient         *HTTPWebTransportClient
}

func (cl *HTTPProxyClientTCP) IsAlive() bool {
	return cl.httpClient.IsAlive()
}

/*
Creates new HTTP Proxy Client for TCP but does not starts it
*/
func NewHTTPProxyClientTCP(httpProxyAddress string, tcpServerAddress string, reportTraffic bool) (*HTTPProxyClientTCP, error) {
	cl := &HTTPProxyClientTCP{clientToId: sync.Map{}, pendingConnections: sync.Map{}, idToClient: sync.Map{}, pendingConnsData: sync.Map{}}
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
			gconn, _ := cl.pendingConnections.Load(string(data))
			if gconn == nil {
				cl.httpClient.Logger.Log(3, "Pending connection with temporary id: "+string(data)+" not found")
				return
			}
			conn := gconn.(*TCPServerConn)
			cl.pendingConnections.Delete(string(data))
			cl.clientToId.Store(conn, string(id))
			cl.idToClient.Store(string(id), conn)
			cl.httpClient.Logger.Log(1, "Prepared new connection with temporary id: "+string(data)+" for connection connected to: "+conn.Conn.RemoteAddr().String()+" connected locally to: "+conn.Conn.LocalAddr().String()+" with new id: "+string(id))

			//Process pending data
			gdata, _ := cl.pendingConnsData.Load(conn)
			data := gdata.([][]byte)
			for len(data) > 0 {
				//Resend data
				cl.httpClient.Send(PackHTTPProxyFrame(HTTP_PROXY_FRAME_TYPE_DATA, id, data[0]))
				cl.pendingConnsData.Store(conn, data[1:])
				gdata, _ = cl.pendingConnsData.Load(conn)
				data = gdata.([][]byte)
			}
			cl.pendingConnsData.Delete(conn)
			return
		}
	case HTTP_PROXY_FRAME_TYPE_CLOSE:
		{
			//Close connection
			gcl, _ := cl.idToClient.Load(string(id))
			gcl.(*TCPServerConn).Close()
		}
	case HTTP_PROXY_FRAME_TYPE_DATA:
		{
			//Resend data
			gcl, _ := cl.idToClient.Load(string(id))
			gcl.(*TCPServerConn).Send(data)
		}
	}
}

func (cl *HTTPProxyClientTCP) handleTCPReadFunc(tcpConn *TCPServerConn, data []byte, ended bool) {
	gdata, _ := cl.pendingConnsData.Load(tcpConn)
	if gdata != nil {
		//Already pending connection
		ldata := gdata.([][]byte)
		ldata = append(ldata, data)
		cl.pendingConnsData.Store(tcpConn, ldata)
		return
	}

	gid, _ := cl.clientToId.Load(tcpConn)
	if gid == nil || gid == "" {
		//No connection found, request new
		tempId := GenerateRandomId()
		cl.pendingConnections.Store(tempId, tcpConn)
		cl.httpClient.Logger.Log(1, "Preparing new connection with temporary id: "+tempId+" for connection connected to: "+tcpConn.Conn.RemoteAddr().String()+" connected locally to: "+tcpConn.Conn.LocalAddr().String())
		cl.httpClient.Send(PackHTTPProxyFrame(HTTP_PROXY_FRAME_TYPE_CONNECT, []byte("0"), []byte(tempId)))
		cl.pendingConnsData.Store(tcpConn, append(make([][]byte, 0), data))
		return
	}
	id := gid.(string)

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
func (cl *HTTPProxyClientTCP) Connect() {
	cl.httpClient.Connect()
	go cl.tcpServer.Start()
}

/*
Connects to HTTP Proxy server and start reading loop, does not locks execution thread
*/
func (cl *HTTPProxyClientTCP) Stop() {
	cl.httpClient.Stop()
	cl.tcpServer.Stop()
}
