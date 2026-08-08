package proxy

import (
	"bytes"
	"net"
	"time"

	"github.com/kolojar/Go-Webtools/helpertools"
	"github.com/kolojar/Go-Webtools/p2p"
	"github.com/kolojar/Go-Webtools/udp"
)

/*
P2PProxyServerUDP is server for proxied UDP traffic over P2P
*/
type P2PProxyServerUDP struct {
	idToClient       helpertools.SafeMap[string, *P2PProxyServerUDPConn]
	clientToID       helpertools.SafeMap[*udp.Client, string]
	p2pClient        *p2p.Client
	udpServerAddress string
	reportTrafic     bool
}

/*
P2PProxyServerUDPConn is connection object of P2PProxyServerUDP
*/
type P2PProxyServerUDPConn struct {
	udpClient *udp.Client
	ID        []byte
	sourceID  []byte
	origin    *P2PProxyServerUDP
}

/*
SendToP2P creates frame and sends it to P2P
*/
func (conn *P2PProxyServerUDPConn) SendToP2P(operation uint8, data []byte) {
	conn.origin.p2pClient.Send(conn.sourceID, PackWebtoolsFrame(operation, conn.ID, data))
}

/*
SendToUDP sends data to UDP
*/
func (conn *P2PProxyServerUDPConn) SendToUDP(data []byte) {
	conn.udpClient.Send(data)
}

/*
Close closes connection to client
*/
func (conn *P2PProxyServerUDPConn) Close(isInitiator bool) {
	if conn == nil || conn.udpClient == nil {
		return
	}
	conn.udpClient.Stop()
	conn.origin.idToClient.Delete(string(conn.ID))
	if isInitiator {
		conn.SendToP2P(FrameTypeClose, nil)
	}
	conn.origin.clientToID.Delete(conn.udpClient)
}

/*
NewP2PProxyServerUDP creates new P2P Proxy Server for UDP but does not starts it
*/
func NewP2PProxyServerUDP(p2pCoordinatorAddress string, p2pPortForIncommingConns int, udpServerAddress string, reportTraffic bool) (*P2PProxyServerUDP, error) {
	sv := &P2PProxyServerUDP{
		udpServerAddress: udpServerAddress,
		clientToID:       helpertools.MakeSafeMap[*udp.Client, string](),
		idToClient:       helpertools.MakeSafeMap[string, *P2PProxyServerUDPConn](),
		reportTrafic:     reportTraffic,
	}
	var err error
	sv.p2pClient, err = p2p.NewP2PClient(p2pCoordinatorAddress, p2pPortForIncommingConns, sv.handleP2PReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	sv.p2pClient.SetLoggerPrefix("P2PProxyServerUDP")
	return sv, nil
}

func (sv *P2PProxyServerUDP) handleP2PReadFunc(_ *p2p.Client, sourceID []byte, frame []byte, ended bool, logger *helpertools.ConsoleLogger) {
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
				cl, err := udp.NewClient(sv.udpServerAddress, sv.handleUDPReadFunc, sv.reportTrafic)
				cl.Logger.Prefix = "P2PProxyServerUDP - " + cl.Logger.Prefix
				if err != nil {
					cl.Logger.Log(3, "Could not create connection with ID: "+string(frame.ID)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.ID), &P2PProxyServerUDPConn{udpClient: cl, ID: frame.ID, sourceID: sourceID, origin: sv})
				sv.clientToID.Set(cl, string(frame.ID))
				sv.idToClient.Get(string(frame.ID)).SendToP2P(FrameTypeConnect, frame.Data)
				return
			}
			logger.Log(3, "Could not find connection to ID: "+string(frame.ID))
			return
		}
		cl := sv.idToClient.Get(string(frame.ID))
		if !cl.udpClient.IsAlive() {
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
				cl.SendToUDP(frame.Data)
			}
		}
	}
}

func (sv *P2PProxyServerUDP) handleUDPReadFunc(udp *udp.Client, _ *net.UDPAddr, data []byte, ended bool) {
	//Get P2P client
	if sv.clientToID.Get(udp) == "" || sv.idToClient.Get(sv.clientToID.Get(udp)) == nil {
		//Connection does not exists
		udp.Logger.Log(3, "Connection connected to: "+udp.Conn.RemoteAddr().String()+" not found")
		return
	}
	ID := sv.clientToID.Get(udp)
	cl := sv.idToClient.Get(ID)

	//End other connection
	if ended {
		cl.Close(true)
	}

	//Send to client
	cl.SendToP2P(FrameTypeData, data)
}

/*
Start starts P2P Proxy Server for UDP. Locks execution thread
*/
func (sv *P2PProxyServerUDP) Start() bool {
	if !sv.p2pClient.ConnectToCoordinator() {
		return false
	}
	for sv.p2pClient.IsAlive() {
		time.Sleep(100 * time.Millisecond)
	}
	return true
}

/*
Stop stops P2P Proxy Server for UDP
*/
func (sv *P2PProxyServerUDP) Stop() {
	sv.p2pClient.Stop()
}
