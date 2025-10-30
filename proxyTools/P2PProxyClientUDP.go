package proxyTools

import (
	"bytes"
	"webtools"
	"webtools/p2pTools"
	"webtools/udpTools"
)

/*
P2P Proxy client for UDP object
*/
type P2PProxyClientUDP struct {
	clientToId         webtools.SafeMap[*udpTools.UDPServerConn, string]
	idToClient         webtools.SafeMap[string, *udpTools.UDPServerConn]
	udpServer          *udpTools.UDPServer
	p2pClient          *p2pTools.P2PClientUDP
	pendingConnections webtools.SafeMap[string, *udpTools.UDPServerConn]
	pendingConnsData   webtools.SafeMap[*udpTools.UDPServerConn, [][]byte]
	p2pServerId        []byte
}

func (cl *P2PProxyClientUDP) IsAlive() bool {
	return cl.p2pClient.IsAlive()
}

/*
Creates new P2P Proxy Client for UDP but does not starts it
*/
func NewP2PProxyClientUDP(p2pCoordinatorAddress string, p2pPortForIncommingConns int, p2pProxyServerId []byte, udpServerAddress string, reportTraffic bool) (*P2PProxyClientUDP, error) {
	cl := &P2PProxyClientUDP{
		clientToId:         webtools.MakeSafeMap[*udpTools.UDPServerConn, string](),
		pendingConnections: webtools.MakeSafeMap[string, *udpTools.UDPServerConn](),
		idToClient:         webtools.MakeSafeMap[string, *udpTools.UDPServerConn](),
		pendingConnsData:   webtools.MakeSafeMap[*udpTools.UDPServerConn, [][]byte](),
		p2pServerId:        p2pProxyServerId,
	}
	var err error
	cl.p2pClient, err = p2pTools.NewP2PClientUDP(p2pCoordinatorAddress, p2pPortForIncommingConns, cl.handleP2PReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.p2pClient.SetLoggerPrefix("P2PProxyClientUDP")
	cl.udpServer, err = udpTools.NewUDPServer(udpServerAddress, cl.handleUDPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.udpServer.Logger.Prefix = "P2PProxyClientUDP - " + cl.udpServer.Logger.Prefix
	return cl, nil
}

func (cl *P2PProxyClientUDP) handleP2PReadFunc(_ *p2pTools.P2PClientUDP, sourceId []byte, frame []byte, ended bool, logger *webtools.ConsoleLogger) {
	if ended {
		//Close all connections
		cl.udpServer.Stop()
		return
	}
	if !bytes.Equal(sourceId, cl.p2pServerId) {
		//Invalid source ID
		cl.udpServer.Logger.Log(3, "Invalid peer source id: "+string(sourceId))
		return
	}

	//Unpack
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, logger) {
		if frame.Operation == 0 {
			return
		}

		switch frame.Operation {
		case webtools.WEBTOOLS_FRAME_TYPE_CONNECT:
			{
				//Confirmed connection
				conn := cl.pendingConnections.Get(string(frame.Data))
				if conn == nil {
					cl.udpServer.Logger.Log(3, "Pending connection with temporary id: "+string(frame.Data)+" not found")
					return
				}
				cl.pendingConnections.Delete(string(frame.Data))
				cl.clientToId.Set(conn, string(frame.Id))
				cl.idToClient.Set(string(frame.Id), conn)
				cl.udpServer.Logger.Log(1, "Prepared new connection with temporary id: "+string(frame.Data)+" for connection connected to: "+conn.Address.String()+" with new id: "+string(frame.Id))

				//Process pending data
				for len(cl.pendingConnsData.Get(conn)) > 0 {
					//Resend data
					cl.p2pClient.Send(cl.p2pServerId, webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_DATA, frame.Id, cl.pendingConnsData.Get(conn)[0]))
					cl.pendingConnsData.Set(conn, cl.pendingConnsData.Get(conn)[1:])
				}
				cl.pendingConnsData.Delete(conn)
				return
			}
		case webtools.WEBTOOLS_FRAME_TYPE_CLOSE:
			{
				//Close connection
				cl.idToClient.Get(string(frame.Id)).Close()
			}
		case webtools.WEBTOOLS_FRAME_TYPE_DATA:
			{
				//Resend data
				cl.idToClient.Get(string(frame.Id)).Send(frame.Data)
			}
		}
	}
}

func (cl *P2PProxyClientUDP) handleUDPReadFunc(udpConn *udpTools.UDPServerConn, data []byte, ended bool) {
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
		cl.udpServer.Logger.Log(1, "Preparing new connection with temporary id: "+tempId+" for connection connected to: "+udpConn.Address.String())
		cl.p2pClient.Send(cl.p2pServerId, webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_CONNECT, []byte("0"), []byte(tempId)))
		cl.pendingConnsData.Set(udpConn, append(make([][]byte, 0), data))
		return
	}

	if ended {
		//Connection ennded
		cl.p2pClient.Send(cl.p2pServerId, webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_CLOSE, []byte(id), nil))
		return
	}
	//Send data
	cl.p2pClient.Send(cl.p2pServerId, webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_DATA, []byte(id), data))
}

/*
Connects to P2P Proxy server and start reading loop, does not lock execution thread
*/
func (cl *P2PProxyClientUDP) Connect() bool {
	if !cl.p2pClient.ConnectToCoordinator() {
		return false
	}
	if !cl.p2pClient.ConnectToPeer(cl.p2pServerId) {
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
