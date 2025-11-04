package proxyTools

import (
	"bytes"
	"webtools"
	"webtools/p2pTools"
	"webtools/tcpTools"
)

/*
P2P Proxy client for UDP object
*/
type P2PProxyClientTCP struct {
	clientToId         webtools.SafeMap[*tcpTools.TCPServerConn, string]
	idToClient         webtools.SafeMap[string, *tcpTools.TCPServerConn]
	tcpServer          *tcpTools.TCPServer
	p2pClient          *p2pTools.P2PClient
	pendingConnections webtools.SafeMap[string, *tcpTools.TCPServerConn]
	pendingConnsData   webtools.SafeMap[*tcpTools.TCPServerConn, [][]byte]
	p2pServerId        []byte
}

func (cl *P2PProxyClientTCP) IsAlive() bool {
	return cl.p2pClient.IsAlive()
}

/*
Creates new P2P Proxy Client for UDP but does not starts it
*/
func NewP2PProxyClientTCP(p2pCoordinatorAddress string, p2pPortForIncommingConns int, p2pProxyServerId []byte, tcpServerAddress string, reportTraffic bool) (*P2PProxyClientTCP, error) {
	cl := &P2PProxyClientTCP{
		clientToId:         webtools.MakeSafeMap[*tcpTools.TCPServerConn, string](),
		pendingConnections: webtools.MakeSafeMap[string, *tcpTools.TCPServerConn](),
		idToClient:         webtools.MakeSafeMap[string, *tcpTools.TCPServerConn](),
		pendingConnsData:   webtools.MakeSafeMap[*tcpTools.TCPServerConn, [][]byte](),
		p2pServerId:        p2pProxyServerId,
	}
	var err error
	cl.p2pClient, err = p2pTools.NewP2PClientUDP(p2pCoordinatorAddress, p2pPortForIncommingConns, cl.handleP2PReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.p2pClient.SetLoggerPrefix("P2PProxyClientTCP")
	cl.tcpServer, err = tcpTools.NewTCPServer(tcpServerAddress, cl.handleTCPReadFunc, reportTraffic, false)
	if err != nil {
		return nil, err
	}
	cl.tcpServer.Logger.Prefix = "P2PProxyClientTCP - " + cl.tcpServer.Logger.Prefix
	return cl, nil
}

func (cl *P2PProxyClientTCP) handleP2PReadFunc(_ *p2pTools.P2PClient, sourceId []byte, frame []byte, ended bool, logger *webtools.ConsoleLogger) {
	if ended {
		//Close all connections
		cl.tcpServer.Stop()
		return
	}
	if !bytes.Equal(sourceId, cl.p2pServerId) {
		//Invalid source ID
		cl.tcpServer.Logger.Log(3, "Invalid peer source id: "+string(sourceId))
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
					cl.tcpServer.Logger.Log(3, "Pending connection with temporary id: "+string(frame.Data)+" not found")
					return
				}
				cl.pendingConnections.Delete(string(frame.Data))
				cl.clientToId.Set(conn, string(frame.Id))
				cl.idToClient.Set(string(frame.Id), conn)
				cl.tcpServer.Logger.Log(1, "Prepared new connection with temporary id: "+string(frame.Data)+" for connection connected to: "+conn.GetConn().RemoteAddr().String()+" with new id: "+string(frame.Id))

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

func (cl *P2PProxyClientTCP) handleTCPReadFunc(tcpConn *tcpTools.TCPServerConn, data []byte, status uint8) {
	if cl.pendingConnsData.Get(tcpConn) != nil {
		//Already pending connection
		cl.pendingConnsData.Set(tcpConn, append(cl.pendingConnsData.Get(tcpConn), data))
		return
	}

	id := cl.clientToId.Get(tcpConn)
	if id == "" {
		//No connection found, request new
		tempId := webtools.GenerateRandomId()
		cl.pendingConnections.Set(tempId, tcpConn)
		cl.tcpServer.Logger.Log(1, "Preparing new connection with temporary id: "+tempId+" for connection connected to: "+tcpConn.GetConn().RemoteAddr().String())
		cl.p2pClient.Send(cl.p2pServerId, webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_CONNECT, []byte("0"), []byte(tempId)))
		cl.pendingConnsData.Set(tcpConn, append(make([][]byte, 0), data))
		return
	}

	if status == webtools.TCP_DISCONNECT_STATUS {
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
func (cl *P2PProxyClientTCP) Connect() bool {
	if !cl.p2pClient.ConnectToCoordinator() {
		return false
	}
	if !cl.p2pClient.ConnectToPeer(cl.p2pServerId) {
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
