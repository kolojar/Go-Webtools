package proxy

import (
	"bytes"
	"time"

	webtools "github.com/kolojar/Go-Webtools"
	"github.com/kolojar/Go-Webtools/helpertools"
	"github.com/kolojar/Go-Webtools/p2p"
	"github.com/kolojar/Go-Webtools/tcp"
)

/*
P2PProxyServerTCP is server for proxied TCP traffic over P2P
*/
type P2PProxyServerTCP struct {
	idToClient       helpertools.SafeMap[string, *P2PProxyServerTCPConn]
	clientToID       helpertools.SafeMap[*tcp.ClientSimple, string]
	p2pClient        *p2p.Client
	udpServerAddress string
	reportTrafic     bool
}

/*
P2PProxyServerTCPConn is connection object of P2PProxyServerTCP
*/
type P2PProxyServerTCPConn struct {
	tcpClient *tcp.ClientSimple
	ID        []byte
	sourceID  []byte
	origin    *P2PProxyServerTCP
}

/*
SendToP2P creates frame and sends it to P2P
*/
func (conn *P2PProxyServerTCPConn) SendToP2P(operation uint8, data []byte) {
	conn.origin.p2pClient.Send(conn.sourceID, PackWebtoolsFrame(operation, conn.ID, data))
}

/*
SendToTCP sends data to UDP
*/
func (conn *P2PProxyServerTCPConn) SendToTCP(data []byte) {
	conn.tcpClient.Send(data)
}

/*
Close closes connection to client
*/
func (conn *P2PProxyServerTCPConn) Close(isInitiator bool) {
	if conn == nil || conn.tcpClient == nil {
		return
	}
	conn.tcpClient.Stop()
	conn.origin.idToClient.Delete(string(conn.ID))
	if isInitiator {
		conn.SendToP2P(FrameTypeClose, nil)
	}
	conn.origin.clientToID.Delete(conn.tcpClient)
}

/*
NewP2PProxyServerTCP creates new P2P Proxy Server for TCP but does not starts it
*/
func NewP2PProxyServerTCP(p2pCoordinatorAddress string, p2pPortForIncommingConns int, tcpServerAddress string, reportTraffic bool) (*P2PProxyServerTCP, error) {
	sv := &P2PProxyServerTCP{
		udpServerAddress: tcpServerAddress,
		clientToID:       helpertools.MakeSafeMap[*tcp.ClientSimple, string](),
		idToClient:       helpertools.MakeSafeMap[string, *P2PProxyServerTCPConn](),
		reportTrafic:     reportTraffic,
	}
	var err error
	sv.p2pClient, err = p2p.NewP2PClient(p2pCoordinatorAddress, p2pPortForIncommingConns, sv.handleP2PReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	sv.p2pClient.SetLoggerPrefix("P2PProxyServerTCP")
	return sv, nil
}

func (sv *P2PProxyServerTCP) handleP2PReadFunc(_ *p2p.Client, sourceID []byte, frame []byte, ended bool, logger *helpertools.ConsoleLogger) {
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
	for _, frame := range UnpackWebtoolsFrame(frame, logger) {
		if frame.Operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(frame.ID)) == nil {
			if frame.Operation == FrameTypeConnect {
				//Create new connection
				frame.ID = []byte(helpertools.GenerateRandomID())
				cl, err := tcp.NewClientSimple(sv.udpServerAddress, -1, false, sv.handleTCPReadFunc, sv.reportTrafic)
				cl.GetLogger().Prefix = "P2PProxyServerTCP - " + cl.GetLogger().Prefix
				if err != nil {
					cl.GetLogger().Log(3, "Could not create connection with ID: "+string(frame.ID)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.ID), &P2PProxyServerTCPConn{tcpClient: cl, ID: frame.ID, sourceID: sourceID, origin: sv})
				sv.clientToID.Set(cl, string(frame.ID))
				sv.idToClient.Get(string(frame.ID)).SendToP2P(FrameTypeConnect, frame.Data)
				return
			}
			logger.Log(3, "Could not find connection to ID: "+string(frame.ID))
			return
		}
		cl := sv.idToClient.Get(string(frame.ID))
		if !cl.tcpClient.IsAlive() {
			logger.Log(3, "Connection with ID: "+string(frame.ID)+" connected to: "+string(sourceID)+" closed")
			return
		}

		//Sort operations
		switch frame.Operation {
		case FrameTypeClose:
			{
				//Close connection
				cl.Close(false)
			}
		case FrameTypeData:
			{
				//Send to UDP
				cl.SendToTCP(frame.Data)
			}
		}
	}
}

func (sv *P2PProxyServerTCP) handleTCPReadFunc(tcp *tcp.ClientSimple, data []byte, status webtools.NetworkStatus) {
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
	cl.SendToP2P(FrameTypeData, data)
}

/*
Start starts P2P Proxy Server for TCP. Locks execution thread
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
Stop stops P2P Proxy Server for TCP
*/
func (sv *P2PProxyServerTCP) Stop() {
	sv.p2pClient.Stop()
}
