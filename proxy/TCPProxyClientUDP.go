package proxy

import (
	"webtools"
	tcptools "webtools/tcp"
	udptools "webtools/udp"
)

/*
TCPProxyClientUDP is client for proxied UDP traffic over TCP
*/
type TCPProxyClientUDP struct {
	clientToID         webtools.SafeMap[*udptools.ServerConn, string]
	idToClient         webtools.SafeMap[string, *udptools.ServerConn]
	udpServer          *udptools.Server
	tcpClient          *tcptools.ClientSimple
	pendingConnections webtools.SafeMap[string, *udptools.ServerConn]
	pendingConnsData   webtools.SafeMap[*udptools.ServerConn, [][]byte]
}

/*
IsAlive gets if server is alive
*/
func (cl *TCPProxyClientUDP) IsAlive() bool {
	return cl.tcpClient.IsAlive()
}

/*
NewTCPProxyClientUDP creates new TCP Proxy Client for UDP but does not starts it
*/
func NewTCPProxyClientUDP(tcpProxyAddress string, udpServerAddress string, reportTraffic bool) (*TCPProxyClientUDP, error) {
	cl := &TCPProxyClientUDP{clientToID: webtools.MakeSafeMap[*udptools.ServerConn, string](), pendingConnections: webtools.MakeSafeMap[string, *udptools.ServerConn](), idToClient: webtools.MakeSafeMap[string, *udptools.ServerConn](), pendingConnsData: webtools.MakeSafeMap[*udptools.ServerConn, [][]byte]()}
	var err error
	cl.tcpClient, err = tcptools.NewClientSimple(tcpProxyAddress, 0, false, cl.handleTCPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.tcpClient.GetLogger().Prefix = "TCPProxyClientUDP - " + cl.tcpClient.GetLogger().Prefix
	cl.udpServer, err = udptools.NewServer(udpServerAddress, cl.handleUDPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.udpServer.Logger.Prefix = "TCPProxyClientUDP - " + cl.udpServer.Logger.Prefix
	return cl, nil
}

func (cl *TCPProxyClientUDP) handleTCPReadFunc(_ *tcptools.ClientSimple, frame []byte, status uint8) {
	if status == webtools.DisconnectStatus {
		//Close all connections
		cl.udpServer.Stop()
		return
	}
	if status != webtools.ReadDataStatus {
		return
	}

	//Unpack
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, cl.tcpClient.GetLogger()) {
		if frame.Operation == 0 {
			return
		}

		switch frame.Operation {
		case webtools.FrameTypeConnect:
			{
				//Confirmed connection
				conn := cl.pendingConnections.Get(string(frame.Data))
				if conn == nil {
					cl.tcpClient.GetLogger().Log(3, "Pending connection with temporary id: "+string(frame.Data)+" not found")
					return
				}
				cl.pendingConnections.Delete(string(frame.Data))
				cl.clientToID.Set(conn, string(frame.ID))
				cl.idToClient.Set(string(frame.ID), conn)
				cl.tcpClient.GetLogger().Log(1, "Prepared new connection with temporary id: "+string(frame.Data)+" for connection connected to: "+conn.Address.String()+" with new id: "+string(frame.ID))

				//Process pending data
				for len(cl.pendingConnsData.Get(conn)) > 0 {
					//Resend data
					cl.tcpClient.Send(webtools.PackWebtoolsFrame(webtools.FrameTypeData, frame.ID, cl.pendingConnsData.Get(conn)[0]))
					cl.pendingConnsData.Set(conn, cl.pendingConnsData.Get(conn)[1:])
				}
				cl.pendingConnsData.Delete(conn)
				return
			}
		case webtools.FrameTypeClose:
			{
				//Close connection
				cl.idToClient.Get(string(frame.ID)).Close()
			}
		case webtools.FrameTypeData:
			{
				//Resend data
				cl.idToClient.Get(string(frame.ID)).Send(frame.Data)
			}
		}
	}
}

func (cl *TCPProxyClientUDP) handleUDPReadFunc(udpConn *udptools.ServerConn, data []byte, ended bool) {
	if cl.pendingConnsData.Get(udpConn) != nil {
		//Already pending connection
		cl.pendingConnsData.Set(udpConn, append(cl.pendingConnsData.Get(udpConn), data))
		return
	}

	id := cl.clientToID.Get(udpConn)
	if id == "" {
		//No connection found, request new
		tempID := webtools.GenerateRandomID()
		cl.pendingConnections.Set(tempID, udpConn)
		cl.tcpClient.GetLogger().Log(1, "Preparing new connection with temporary id: "+tempID+" for connection connected to: "+udpConn.Address.String())
		cl.tcpClient.Send(webtools.PackWebtoolsFrame(webtools.FrameTypeConnect, []byte("0"), []byte(tempID)))
		cl.pendingConnsData.Set(udpConn, append(make([][]byte, 0), data))
		return
	}

	if ended {
		//Connection ennded
		cl.tcpClient.Send(webtools.PackWebtoolsFrame(webtools.FrameTypeClose, []byte(id), nil))
		return
	}
	//Send data
	cl.tcpClient.Send(webtools.PackWebtoolsFrame(webtools.FrameTypeData, []byte(id), data))
}

/*
Connect connects to TCP Proxy server and start reading loop, does not locks execution thread
*/
func (cl *TCPProxyClientUDP) Connect() {
	cl.tcpClient.Connect()
	go cl.udpServer.Start()
}

/*
Stop stops the client
*/
func (cl *TCPProxyClientUDP) Stop() {
	cl.tcpClient.Stop()
	cl.udpServer.Stop()
}
