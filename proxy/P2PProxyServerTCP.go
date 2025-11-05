package proxyTools

import (
	"bytes"
	"time"
	"webtools"
	"webtools/p2pTools"
	"webtools/tcpTools"
)

/*
P2P Proxy server for UDP object
*/
type P2PProxyServerTCP struct {
	idToClient       webtools.SafeMap[string, *P2PProxyServerTCPConn]
	clientToId       webtools.SafeMap[*tcpTools.TCPClientSimple, string]
	p2pClient        *p2pTools.P2PClient
	udpServerAddress string
	reportTrafic     bool
}

/*
P2P Proxy server for UDP connection object
*/
type P2PProxyServerTCPConn struct {
	tcpClient *tcpTools.TCPClientSimple
	id        []byte
	sourceId  []byte
	origin    *P2PProxyServerTCP
}

/*
Creates frame and sends it to P2P
*/
func (cl *P2PProxyServerTCPConn) SendToP2P(operation uint8, data []byte) {
	cl.origin.p2pClient.Send(cl.sourceId, webtools.PackWebtoolsFrame(operation, cl.id, data))
}

/*
Creates frame and sends it to UDP
*/
func (cl *P2PProxyServerTCPConn) SendToUDP(data []byte) {
	cl.tcpClient.Send(data)
}

/*
Closes connection to client
*/
func (cl *P2PProxyServerTCPConn) Close(isInitiator bool) {
	if cl == nil || cl.tcpClient == nil {
		return
	}
	cl.tcpClient.Stop()
	cl.origin.idToClient.Delete(string(cl.id))
	if isInitiator {
		cl.SendToP2P(webtools.WEBTOOLS_FRAME_TYPE_CLOSE, nil)
	}
	cl.origin.clientToId.Delete(cl.tcpClient)
}

/*
Creates new P2P Proxy Server for UDP but does not starts it
*/
func NewP2PProxyServerTCP(p2pCoordinatorAddress string, p2pPortForIncommingConns int, tcpServerAddress string, reportTraffic bool) (*P2PProxyServerTCP, error) {
	sv := &P2PProxyServerTCP{
		udpServerAddress: tcpServerAddress,
		clientToId:       webtools.MakeSafeMap[*tcpTools.TCPClientSimple, string](),
		idToClient:       webtools.MakeSafeMap[string, *P2PProxyServerTCPConn](),
		reportTrafic:     reportTraffic,
	}
	var err error
	sv.p2pClient, err = p2pTools.NewP2PClientUDP(p2pCoordinatorAddress, p2pPortForIncommingConns, sv.handleP2PReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	sv.p2pClient.SetLoggerPrefix("P2PProxyServerTCP")
	return sv, nil
}

func (sv *P2PProxyServerTCP) handleP2PReadFunc(_ *p2pTools.P2PClient, sourceId []byte, frame []byte, ended bool, logger *webtools.ConsoleLogger) {
	if ended {
		//Close all connections with this P2P Conn
		for _, d := range sv.idToClient.GetData() {
			if d.Value == nil {
				continue
			}
			if bytes.Equal(d.Value.sourceId, sourceId) {
				d.Value.Close(true)
			}
		}
		return
	}

	//Unpack frame
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, logger) {
		if frame.Operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(frame.Id)) == nil {
			if frame.Operation == webtools.WEBTOOLS_FRAME_TYPE_CONNECT {
				//Create new connection
				frame.Id = []byte(webtools.GenerateRandomId())
				cl, err := tcpTools.NewTCPClientSimple(sv.udpServerAddress, -1, false, sv.handleTCPReadFunc, sv.reportTrafic)
				cl.GetLogger().Prefix = "P2PProxyServerTCP - " + cl.GetLogger().Prefix
				if err != nil {
					cl.GetLogger().Log(3, "Could not create connection with id: "+string(frame.Id)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.Id), &P2PProxyServerTCPConn{tcpClient: cl, id: frame.Id, sourceId: sourceId, origin: sv})
				sv.clientToId.Set(cl, string(frame.Id))
				sv.idToClient.Get(string(frame.Id)).SendToP2P(webtools.WEBTOOLS_FRAME_TYPE_CONNECT, frame.Data)
				return
			} else {
				logger.Log(3, "Could not find connection to id: "+string(frame.Id))
				return
			}
		}
		cl := sv.idToClient.Get(string(frame.Id))
		if !cl.tcpClient.IsAlive() {
			logger.Log(3, "Connection with id: "+string(frame.Id)+" connected to: "+string(sourceId)+" closed")
			return
		}

		//Sort operations
		switch frame.Operation {
		case webtools.WEBTOOLS_FRAME_TYPE_CLOSE:
			{
				//Close connection
				cl.Close(false)
			}
		case webtools.WEBTOOLS_FRAME_TYPE_DATA:
			{
				//Send to UDP
				cl.SendToUDP(frame.Data)
			}
		}
	}
}

func (sv *P2PProxyServerTCP) handleTCPReadFunc(tcp *tcpTools.TCPClientSimple, data []byte, status uint8) {
	//Get P2P client
	if sv.clientToId.Get(tcp) == "" || sv.idToClient.Get(sv.clientToId.Get(tcp)) == nil {
		//Connection does not exists
		tcp.GetLogger().Log(3, "Connection connected to: "+tcp.GetConn().RemoteAddr().String()+" not found")
		return
	}
	id := sv.clientToId.Get(tcp)
	cl := sv.idToClient.Get(id)

	//End other connection
	if status == webtools.TCP_DISCONNECT_STATUS {
		cl.Close(true)
	}

	//Send to client
	cl.SendToP2P(webtools.WEBTOOLS_FRAME_TYPE_DATA, data)
}

/*
Starts P2P Proxy Server for UDP. Locks execution thread
*/
func (sv *P2PProxyServerTCP) Start() bool {
	if !sv.p2pClient.ConnectToCoordinator() {
		return false
	}
	for sv.p2pClient.IsAlive() {
		time.Sleep(100 * time.Millisecond)
	}
	return true
}

/*
Stops P2P Proxy Server for UDP
*/
func (sv *P2PProxyServerTCP) Stop() {
	sv.p2pClient.Stop()
}
