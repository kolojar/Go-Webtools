package proxy

import (
	"bytes"
	"webtools"
	"webtools/p2p"
	"webtools/tcp"
)

/*
P2P Proxy client for UDP object
*/
type P2PProxyClientTCP struct {
	clientToID         webtools.SafeMap[*tcp.ServerConn, string]
	idToClient         webtools.SafeMap[string, *tcp.ServerConn]
	tcpServer          *tcp.Server
	p2pClient          *p2p.Client
	pendingConnections webtools.SafeMap[string, *tcp.ServerConn]
	pendingConnsData   webtools.SafeMap[*tcp.ServerConn, [][]byte]
	p2pServerID        []byte
}

func (cl *P2PProxyClientTCP) IsAlive() bool {
	return cl.p2pClient.IsAlive()
}

/*
Creates new P2P Proxy Client for UDP but does not starts it
*/
func NewP2PProxyClientTCP(p2pCoordinatorAddress string, p2pPortForIncommingConns int, p2pProxyServerID []byte, tcpServerAddress string, reportTraffic bool) (*P2PProxyClientTCP, error) {
	cl := &P2PProxyClientTCP{
		clientToID:         webtools.MakeSafeMap[*tcp.ServerConn, string](),
		pendingConnections: webtools.MakeSafeMap[string, *tcp.ServerConn](),
		idToClient:         webtools.MakeSafeMap[string, *tcp.ServerConn](),
		pendingConnsData:   webtools.MakeSafeMap[*tcp.ServerConn, [][]byte](),
		p2pServerID:        p2pProxyServerID,
	}
	var err error
	cl.p2pClient, err = p2p.NewP2PClientUDP(p2pCoordinatorAddress, p2pPortForIncommingConns, cl.handleP2PReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.p2pClient.SetLoggerPrefix("P2PProxyClientTCP")
	cl.tcpServer, err = tcp.NewServer(tcpServerAddress, cl.handleTCPReadFunc, reportTraffic, false)
	if err != nil {
		return nil, err
	}
	cl.tcpServer.Logger.Prefix = "P2PProxyClientTCP - " + cl.tcpServer.Logger.Prefix
	return cl, nil
}

func (cl *P2PProxyClientTCP) handleP2PReadFunc(_ *p2p.Client, sourceID []byte, frame []byte, ended bool, logger *webtools.ConsoleLogger) {
	if ended {
		//Close all connections
		cl.tcpServer.Stop()
		return
	}
	if !bytes.Equal(sourceID, cl.p2pServerID) {
		//InvalID source ID
		cl.tcpServer.Logger.Log(3, "InvalID peer source ID: "+string(sourceID))
		return
	}

	//Unpack
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, logger) {
		if frame.Operation == 0 {
			return
		}

		switch frame.Operation {
		case webtools.ConnectStatus:
			{
				//Confirmed connection
				conn := cl.pendingConnections.Get(string(frame.Data))
				if conn == nil {
					cl.tcpServer.Logger.Log(3, "Pending connection with temporary ID: "+string(frame.Data)+" not found")
					return
				}
				cl.pendingConnections.Delete(string(frame.Data))
				cl.clientToID.Set(conn, string(frame.ID))
				cl.idToClient.Set(string(frame.ID), conn)
				cl.tcpServer.Logger.Log(1, "Prepared new connection with temporary ID: "+string(frame.Data)+" for connection connected to: "+conn.GetConn().RemoteAddr().String()+" with new ID: "+string(frame.ID))

				//Process pending data
				for len(cl.pendingConnsData.Get(conn)) > 0 {
					//Resend data
					cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeData, frame.ID, cl.pendingConnsData.Get(conn)[0]))
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

func (cl *P2PProxyClientTCP) handleTCPReadFunc(tcpConn *tcp.ServerConn, data []byte, status uint8) {
	if cl.pendingConnsData.Get(tcpConn) != nil {
		//Already pending connection
		cl.pendingConnsData.Set(tcpConn, append(cl.pendingConnsData.Get(tcpConn), data))
		return
	}

	ID := cl.clientToID.Get(tcpConn)
	if ID == "" {
		//No connection found, request new
		tempID := webtools.GenerateRandomID()
		cl.pendingConnections.Set(tempID, tcpConn)
		cl.tcpServer.Logger.Log(1, "Preparing new connection with temporary ID: "+tempID+" for connection connected to: "+tcpConn.GetConn().RemoteAddr().String())
		cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.ConnectStatus, []byte("0"), []byte(tempID)))
		cl.pendingConnsData.Set(tcpConn, append(make([][]byte, 0), data))
		return
	}

	if status == webtools.DisconnectStatus {
		//Connection ennded
		cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeClose, []byte(ID), nil))
		return
	}
	//Send data
	cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeData, []byte(ID), data))
}

/*
Connects to P2P Proxy server and start reading loop, does not lock execution thread
*/
func (cl *P2PProxyClientTCP) Connect() bool {
	if !cl.p2pClient.ConnectToCoordinator() {
		return false
	}
	if !cl.p2pClient.ConnectToPeer(cl.p2pServerID) {
		return false
	}
	go cl.tcpServer.Start()
	return true
}

/*
Stops P2P Proxy client
*/
func (cl *P2PProxyClientTCP) Stop() {
	cl.p2pClient.Stop()
	cl.tcpServer.Stop()
}
