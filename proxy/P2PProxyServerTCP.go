package proxy

import (
	"bytes"
	"time"
	"webtools"
	"webtools/p2p"
	"webtools/tcp"
)

/*
P2P Proxy server for UDP object
*/
type P2PProxyServerTCP struct {
	idToClient       webtools.SafeMap[string, *P2PProxyServerTCPConn]
	clientToID       webtools.SafeMap[*tcp.ClientSimple, string]
	p2pClient        *p2p.Client
	udpServerAddress string
	reportTrafic     bool
}

/*
P2P Proxy server for UDP connection object
*/
type P2PProxyServerTCPConn struct {
	tcpClient *tcp.ClientSimple
	ID        []byte
	sourceID  []byte
	origin    *P2PProxyServerTCP
}

/*
Creates frame and sends it to P2P
*/
func (cl *P2PProxyServerTCPConn) SendToP2P(operation uint8, data []byte) {
	cl.origin.p2pClient.Send(cl.sourceID, webtools.PackWebtoolsFrame(operation, cl.ID, data))
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
	cl.origin.idToClient.Delete(string(cl.ID))
	if isInitiator {
		cl.SendToP2P(webtools.FrameTypeClose, nil)
	}
	cl.origin.clientToID.Delete(cl.tcpClient)
}

/*
Creates new P2P Proxy Server for UDP but does not starts it
*/
func NewP2PProxyServerTCP(p2pCoordinatorAddress string, p2pPortForIncommingConns int, tcpServerAddress string, reportTraffic bool) (*P2PProxyServerTCP, error) {
	sv := &P2PProxyServerTCP{
		udpServerAddress: tcpServerAddress,
		clientToID:       webtools.MakeSafeMap[*tcp.ClientSimple, string](),
		idToClient:       webtools.MakeSafeMap[string, *P2PProxyServerTCPConn](),
		reportTrafic:     reportTraffic,
	}
	var err error
	sv.p2pClient, err = p2p.NewP2PClientUDP(p2pCoordinatorAddress, p2pPortForIncommingConns, sv.handleP2PReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	sv.p2pClient.SetLoggerPrefix("P2PProxyServerTCP")
	return sv, nil
}

func (sv *P2PProxyServerTCP) handleP2PReadFunc(_ *p2p.Client, sourceID []byte, frame []byte, ended bool, logger *webtools.ConsoleLogger) {
	if ended {
		//Close all connections with this P2P Conn
		for _, d := range sv.idToClient.GetData() {
			if d.Value == nil {
				continue
			}
			if bytes.Equal(d.Value.sourceID, sourceID) {
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
		if sv.idToClient.Get(string(frame.ID)) == nil {
			if frame.Operation == webtools.FrameTypeConnect {
				//Create new connection
				frame.ID = []byte(webtools.GenerateRandomID())
				cl, err := tcp.NewClientSimple(sv.udpServerAddress, -1, false, sv.handleTCPReadFunc, sv.reportTrafic)
				cl.GetLogger().Prefix = "P2PProxyServerTCP - " + cl.GetLogger().Prefix
				if err != nil {
					cl.GetLogger().Log(3, "Could not create connection with ID: "+string(frame.ID)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.ID), &P2PProxyServerTCPConn{tcpClient: cl, ID: frame.ID, sourceID: sourceID, origin: sv})
				sv.clientToID.Set(cl, string(frame.ID))
				sv.idToClient.Get(string(frame.ID)).SendToP2P(webtools.FrameTypeConnect, frame.Data)
				return
			} else {
				logger.Log(3, "Could not find connection to ID: "+string(frame.ID))
				return
			}
		}
		cl := sv.idToClient.Get(string(frame.ID))
		if !cl.tcpClient.IsAlive() {
			logger.Log(3, "Connection with ID: "+string(frame.ID)+" connected to: "+string(sourceID)+" closed")
			return
		}

		//Sort operations
		switch frame.Operation {
		case webtools.FrameTypeClose:
			{
				//Close connection
				cl.Close(false)
			}
		case webtools.FrameTypeData:
			{
				//Send to UDP
				cl.SendToUDP(frame.Data)
			}
		}
	}
}

func (sv *P2PProxyServerTCP) handleTCPReadFunc(tcp *tcp.ClientSimple, data []byte, status uint8) {
	//Get P2P client
	if sv.clientToID.Get(tcp) == "" || sv.idToClient.Get(sv.clientToID.Get(tcp)) == nil {
		//Connection does not exists
		tcp.GetLogger().Log(3, "Connection connected to: "+tcp.GetConn().RemoteAddr().String()+" not found")
		return
	}
	ID := sv.clientToID.Get(tcp)
	cl := sv.idToClient.Get(ID)

	//End other connection
	if status == webtools.DisconnectStatus {
		cl.Close(true)
	}

	//Send to client
	cl.SendToP2P(webtools.FrameTypeData, data)
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
