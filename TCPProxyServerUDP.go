package webtools

/*
TCP Proxy server for UDP object
*/
type TCPProxyServerUDP struct {
	idToClient       SafeMap[string, *TCPProxyServerUDPConn]
	clientToId       SafeMap[*UDPClient, string]
	tcpServer        *TCPServer
	tcpServerAddress string
	reportTrafic     bool
}

/*
TCP Proxy server for UDP connection object
*/
type TCPProxyServerUDPConn struct {
	udpClient *UDPClient
	id        []byte
	source    *TCPServerConn
	origin    *TCPProxyServerUDP
}

/*
Creates frame and sends it to TCP
*/
func (cl *TCPProxyServerUDPConn) SendToTCP(operation uint8, data []byte) {
	cl.source.Send(PackProxyFrame(operation, cl.id, data))
}

/*
Creates frame and sends it to UDP
*/
func (cl *TCPProxyServerUDPConn) SendToUDP(data []byte) {
	cl.udpClient.Send(data)
}

/*
Closes connection to client
*/
func (cl *TCPProxyServerUDPConn) Close(isInitiator bool) {
	if cl == nil || cl.udpClient == nil {
		return
	}
	cl.udpClient.Stop()
	cl.origin.idToClient.Delete(string(cl.id))
	if isInitiator {
		cl.SendToTCP(PROXY_FRAME_TYPE_CLOSE, nil)
	}
	cl.origin.clientToId.Delete(cl.udpClient)
}

/*
Creates new TCP Proxy Server for UDP but does not starts it
*/
func NewTCPProxyServerUDP(tcpProxyAddress string, udpServerAddress string, reportTraffic bool) (*TCPProxyServerUDP, error) {
	sv := &TCPProxyServerUDP{tcpServerAddress: udpServerAddress, clientToId: MakeSafeMap[*UDPClient, string](), idToClient: MakeSafeMap[string, *TCPProxyServerUDPConn](), reportTrafic: reportTraffic}
	var err error
	sv.tcpServer, err = NewTCPServer(tcpProxyAddress, sv.handleTCPReadFunc, reportTraffic, true)
	if err != nil {
		return nil, err
	}
	sv.tcpServer.Logger.Prefix = "TCPProxyServerUDP - " + sv.tcpServer.Logger.Prefix
	return sv, nil
}

func (sv *TCPProxyServerUDP) handleTCPReadFunc(conn *TCPServerConn, frame []byte, ended bool) {
	if ended {
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

	//Unpack frame
	for _, frame := range UnpackProxyFrame(frame, conn.origin.Logger) {
		operation, id, data := frame.A, frame.B, frame.C
		if operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(id)) == nil {
			if operation == PROXY_FRAME_TYPE_CONNECT {
				//Create new connection
				id = []byte(GenerateRandomId())
				cl, err := NewUDPClient(sv.tcpServerAddress, sv.handleUDPReadFunc, sv.reportTrafic)
				cl.Logger.Prefix = "TCPProxyServerUDP - " + cl.Logger.Prefix
				if err != nil {
					conn.origin.Logger.Log(3, "Could not create connection with id: "+string(id)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(id), &TCPProxyServerUDPConn{udpClient: cl, id: id, source: conn, origin: sv})
				sv.clientToId.Set(cl, string(id))
				sv.idToClient.Get(string(id)).SendToTCP(PROXY_FRAME_TYPE_CONNECT, data)
				return
			} else {
				conn.origin.Logger.Log(3, "Could not find connection to id: "+string(id))
				return
			}
		}
		cl := sv.idToClient.Get(string(id))
		if !cl.udpClient.IsAlive() {
			conn.origin.Logger.Log(3, "Connection with id: "+string(id)+" connected to: "+conn.Conn.RemoteAddr().String()+" connected locally to: "+conn.Conn.LocalAddr().String()+" closed")
			return
		}

		//Sort operations
		switch operation {
		case PROXY_FRAME_TYPE_CLOSE:
			{
				//Close connection
				cl.Close(false)
			}
		case PROXY_FRAME_TYPE_DATA:
			{
				//Send to UDP
				cl.SendToUDP(data)
			}
		}
	}
}

func (sv *TCPProxyServerUDP) handleUDPReadFunc(udp *UDPClient, data []byte, ended bool) {
	//Get TCP client
	if sv.clientToId.Get(udp) == "" || sv.idToClient.Get(sv.clientToId.Get(udp)) == nil {
		//Connection does not exists
		udp.Logger.Log(3, "Connection connected to: "+udp.address.String()+" not found")
		return
	}
	id := sv.clientToId.Get(udp)
	cl := sv.idToClient.Get(id)

	//End other connection
	if ended {
		cl.Close(true)
	}

	//Send to client
	cl.SendToTCP(PROXY_FRAME_TYPE_DATA, data)
}

/*
Starts TCP Proxy Server for UDP
*/
func (sv *TCPProxyServerUDP) Start() {
	sv.tcpServer.Start()
}

/*
Stops TCP Proxy Server for UDP
*/
func (sv *TCPProxyServerUDP) Stop() {
	sv.tcpServer.Stop()
}
