package proxytools

import (
	"webtools"
	tcptools "webtools/tcpTools"
)

/*
TCP Proxy client for UDP object
*/
type TCPProxyClientUDP struct {
	clientToId         webtools.SafeMap[*UDPServerConn, string]
	idToClient         webtools.SafeMap[string, *UDPServerConn]
	udpServer          *UDPServer
	tcpClient          *tcptools.TCPClientSimple
	pendingConnections webtools.SafeMap[string, *UDPServerConn]
	pendingConnsData   webtools.SafeMap[*UDPServerConn, [][]byte]
}

func (cl *TCPProxyClientUDP) IsAlive() bool {
	return cl.tcpClient.IsAlive()
}

/*
Creates new TCP Proxy Client for UDP but does not starts it
*/
func NewTCPProxyClientUDP(tcpProxyAddress string, udpServerAddress string, reportTraffic bool) (*TCPProxyClientUDP, error) {
	cl := &TCPProxyClientUDP{clientToId: webtools.MakeSafeMap[*UDPServerConn, string](), pendingConnections: webtools.MakeSafeMap[string, *UDPServerConn](), idToClient: webtools.MakeSafeMap[string, *UDPServerConn](), pendingConnsData: webtools.MakeSafeMap[*UDPServerConn, [][]byte]()}
	var err error
	cl.tcpClient, err = tcptools.NewTCPClientSimple(tcpProxyAddress, 0, false, cl.handleTCPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.tcpClient.GetLogger().Prefix = "TCPProxyClientUDP - " + cl.tcpClient.GetLogger().Prefix
	cl.udpServer, err = NewUDPServer(udpServerAddress, cl.handleUDPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.udpServer.Logger.Prefix = "TCPProxyClientUDP - " + cl.udpServer.Logger.Prefix
	return cl, nil
}

func (cl *TCPProxyClientUDP) handleTCPReadFunc(_ *tcptools.TCPClientSimple, frame []byte, status uint8) {
	if status == webtools.TCP_DISCONNECT_STATUS {
		//Close all connections
		cl.udpServer.Stop()
		return
	}
	if status != webtools.TCP_READ_DATA_STATUS {
		return
	}

	//Unpack
	for _, frame := range UnpackProxyFrame(frame, cl.tcpClient.GetLogger()) {
		if operation == 0 {
			return
		}

		switch frame.Operation {
		case PROXY_FRAME_TYPE_CONNECT:
			{
				//Confirmed connection
				conn := cl.pendingConnections.Get(string(frame.Data))
				if conn == nil {
					cl.tcpClient.GetLogger().Log(3, "Pending connection with temporary id: "+string(frame.Data)+" not found")
					return
				}
				cl.pendingConnections.Delete(string(frame.Data))
				cl.clientToId.Set(conn, string(frame.Id))
				cl.idToClient.Set(string(frame.Id), conn)
				cl.tcpClient.GetLogger().Log(1, "Prepared new connection with temporary id: "+string(frame.Data)+" for connection connected to: "+conn.Address.String()+" with new id: "+string(id))

				//Process pending data
				for len(cl.pendingConnsData.Get(conn)) > 0 {
					//Resend data
					cl.tcpClient.Send(PackProxyFrame(PROXY_FRAME_TYPE_DATA, frame.Id, cl.pendingConnsData.Get(conn)[0]))
					cl.pendingConnsData.Set(conn, cl.pendingConnsData.Get(conn)[1:])
				}
				cl.pendingConnsData.Delete(conn)
				return
			}
		case PROXY_FRAME_TYPE_CLOSE:
			{
				//Close connection
				cl.idToClient.Get(string(frame.Id)).Close()
			}
		case PROXY_FRAME_TYPE_DATA:
			{
				//Resend data
				cl.idToClient.Get(string(frame.Id)).Send(frame.Data)
			}
		}
	}
}

func (cl *TCPProxyClientUDP) handleUDPReadFunc(udpConn *UDPServerConn, data []byte, ended bool) {
	if cl.pendingConnsData.Get(udpConn) != nil {
		//Already pending connection
		cl.pendingConnsData.Set(udpConn, append(cl.pendingConnsData.Get(udpConn), data))
		return
	}

	id := cl.clientToId.Get(udpConn)
	if id == "" {
		//No connection found, request new
		tempId := webtools.GenerateRandomId()
		cl.pendingConnections.Set(tempId, udpConn)
		cl.tcpClient.GetLogger().Log(1, "Preparing new connection with temporary id: "+tempId+" for connection connected to: "+udpConn.Address.String())
		cl.tcpClient.Send(PackProxyFrame(PROXY_FRAME_TYPE_CONNECT, []byte("0"), []byte(tempId)))
		cl.pendingConnsData.Set(udpConn, append(make([][]byte, 0), data))
		return
	}

	if ended {
		//Connection ennded
		cl.tcpClient.Send(PackProxyFrame(PROXY_FRAME_TYPE_CLOSE, []byte(id), nil))
		return
	}
	//Send data
	cl.tcpClient.Send(PackProxyFrame(PROXY_FRAME_TYPE_DATA, []byte(id), data))
}

/*
Connects to TCP Proxy server and start reading loop, does not locks execution thread
*/
func (cl *TCPProxyClientUDP) Connect() {
	cl.tcpClient.Connect()
	go cl.udpServer.Start()
}

/*
Connects to TCP Proxy server and start reading loop, does not locks execution thread
*/
func (cl *TCPProxyClientUDP) Stop() {
	cl.tcpClient.Stop()
	cl.udpServer.Stop()
}
