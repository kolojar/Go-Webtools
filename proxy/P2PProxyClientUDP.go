package proxy

import (
	"bytes"
	"webtools"
	"webtools/p2p"
	"webtools/udp"
)

/*
P2P Proxy client for UDP object
*/
type P2PProxyClientUDP struct {
	clientToID         webtools.SafeMap[*udp.ServerConn, string]
	idToClient         webtools.SafeMap[string, *udp.ServerConn]
	udpServer          *udp.Server
	p2pClient          *p2p.P2PClient
	pendingConnections webtools.SafeMap[string, *udp.ServerConn]
	pendingConnsData   webtools.SafeMap[*udp.ServerConn, [][]byte]
	p2pServerID        []byte
}

func (cl *P2PProxyClientUDP) IsAlive() bool {
	return cl.p2pClient.IsAlive()
}

/*
Creates new P2P Proxy Client for UDP but does not starts it
*/
func NewP2PProxyClientUDP(p2pCoordinatorAddress string, p2pPortForIncommingConns int, p2pProxyServerID []byte, udpServerAddress string, reportTraffic bool) (*P2PProxyClientUDP, error) {
	cl := &P2PProxyClientUDP{
		clientToID:         webtools.MakeSafeMap[*udp.ServerConn, string](),
		pendingConnections: webtools.MakeSafeMap[string, *udp.ServerConn](),
		idToClient:         webtools.MakeSafeMap[string, *udp.ServerConn](),
		pendingConnsData:   webtools.MakeSafeMap[*udp.ServerConn, [][]byte](),
		p2pServerID:        p2pProxyServerID,
	}
	var err error
	cl.p2pClient, err = p2p.NewP2PClientUDP(p2pCoordinatorAddress, p2pPortForIncommingConns, cl.handleP2PReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.p2pClient.SetLoggerPrefix("P2PProxyClientUDP")
	cl.udpServer, err = udp.NewServer(udpServerAddress, cl.handleUDPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.udpServer.Logger.Prefix = "P2PProxyClientUDP - " + cl.udpServer.Logger.Prefix
	return cl, nil
}

func (cl *P2PProxyClientUDP) handleP2PReadFunc(_ *p2p.P2PClient, sourceID []byte, frame []byte, ended bool, logger *webtools.ConsoleLogger) {
	if ended {
		//Close all connections
		cl.udpServer.Stop()
		return
	}
	if !bytes.Equal(sourceID, cl.p2pServerID) {
		//InvalID source ID
		cl.udpServer.Logger.Log(3, "InvalID peer source ID: "+string(sourceID))
		return
	}

	//Unpack
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, logger) {
		if frame.Operation == 0 {
			return
		}

		switch frame.Operation {
		case webtools.FrameTypeConnect:
			{
				//Confirmed connection
				conn := cl.pendingConnections.Get(string(frame.Data))
				if conn == nil {
					cl.udpServer.Logger.Log(3, "Pending connection with temporary ID: "+string(frame.Data)+" not found")
					return
				}
				cl.pendingConnections.Delete(string(frame.Data))
				cl.clientToID.Set(conn, string(frame.ID))
				cl.idToClient.Set(string(frame.ID), conn)
				cl.udpServer.Logger.Log(1, "Prepared new connection with temporary ID: "+string(frame.Data)+" for connection connected to: "+conn.Address.String()+" with new ID: "+string(frame.ID))

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

func (cl *P2PProxyClientUDP) handleUDPReadFunc(udpConn *udp.ServerConn, data []byte, ended bool) {
	if cl.pendingConnsData.Get(udpConn) != nil {
		//Already pending connection
		cl.pendingConnsData.Set(udpConn, append(cl.pendingConnsData.Get(udpConn), data))
		return
	}

	ID := cl.clientToID.Get(udpConn)
	if ID == "" {
		//No connection found, request new
		tempID := webtools.GenerateRandomID()
		cl.pendingConnections.Set(tempID, udpConn)
		cl.udpServer.Logger.Log(1, "Preparing new connection with temporary ID: "+tempID+" for connection connected to: "+udpConn.Address.String())
		cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.ConnectStatus, []byte("0"), []byte(tempID)))
		cl.pendingConnsData.Set(udpConn, append(make([][]byte, 0), data))
		return
	}

	if ended {
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
func (cl *P2PProxyClientUDP) Connect() bool {
	if !cl.p2pClient.ConnectToCoordinator() {
		return false
	}
	if !cl.p2pClient.ConnectToPeer(cl.p2pServerID) {
		return false
	}
	go cl.udpServer.Start()
	return true
}

/*
Stops P2P Proxy client
*/
func (cl *P2PProxyClientUDP) Stop() {
	cl.p2pClient.Stop()
	cl.udpServer.Stop()
}
