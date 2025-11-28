package proxy

import (
	"bytes"
	"encoding/json"
	"net"
	"strings"
	"time"
	"webtools"
	"webtools/p2p"
	"webtools/tcp"
	"webtools/udp"
)

/*
P2PProxyServerUniversal is server for proxied UDP and TCP traffic over P2P
*/
type P2PProxyServerUniversal struct {
	idToClient webtools.SafeMap[string, *P2PProxyServerUniversalConn]
	//clientToID       webtools.SafeMap[*udp.Client, string]
	p2pClient    *p2p.Client
	reportTrafic bool
	//Key is service name, values are isUDP, reportTraffic, and address
	ProxiedServices map[string]webtools.ThreeValuePair[bool, bool, string]
}

/*
P2PProxyServerUniversalConn is connection object of P2PProxyServerUniversal
*/
type P2PProxyServerUniversalConn struct {
	udpClient *udp.Client
	tcpClient *tcp.ClientSimple
	ID        []byte
	sourceID  []byte
	origin    *P2PProxyServerUniversal
}

/*
IsAlive gets if connection is alive
*/
func (conn *P2PProxyServerUniversalConn) IsAlive() bool {
	if conn.udpClient != nil {
		return conn.udpClient.IsAlive()
	}
	if conn.tcpClient != nil {
		return conn.tcpClient.IsAlive()
	}
	return false
}

/*
SendToP2P creates frame and sends it to P2P
*/
func (conn *P2PProxyServerUniversalConn) SendToP2P(operation uint8, data []byte) {
	conn.origin.p2pClient.Send(conn.sourceID, webtools.PackWebtoolsFrame(operation, conn.ID, data))
}

/*
SendToLocal sends data to UDP or TCP
*/
func (conn *P2PProxyServerUniversalConn) SendToLocal(data []byte) {
	if conn.udpClient != nil {
		conn.udpClient.Send(data)
	}
	if conn.tcpClient != nil {
		conn.tcpClient.Send(data)
	}
}

/*
Close closes connection to client
*/
func (conn *P2PProxyServerUniversalConn) Close(isInitiator bool) {
	if conn == nil {
		return
	}
	if conn.udpClient != nil {
		conn.udpClient.Stop()
	}
	if conn.tcpClient != nil {
		conn.tcpClient.Stop()
	}
	conn.origin.idToClient.Delete(string(conn.ID))
	if isInitiator {
		conn.SendToP2P(webtools.FrameTypeClose, nil)
	}
	//conn.origin.clientToID.Delete(conn.udpClient)
}

/*
NewP2PProxyServerUniversal creates new P2P Proxy Server for UDP and TCP but does not starts it
*/
func NewP2PProxyServerUniversal(p2pCoordinatorAddress string, p2pPortForIncommingConns int, reportTraffic bool) (*P2PProxyServerUniversal, error) {
	sv := &P2PProxyServerUniversal{
		ProxiedServices: map[string]webtools.ThreeValuePair[bool, bool, string]{},
		//clientToID:       webtools.MakeSafeMap[*udp.Client, string](),
		idToClient:   webtools.MakeSafeMap[string, *P2PProxyServerUniversalConn](),
		reportTrafic: reportTraffic,
	}
	var err error
	sv.p2pClient, err = p2p.NewP2PClient(p2pCoordinatorAddress, p2pPortForIncommingConns, sv.handleP2PReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	sv.p2pClient.SetLoggerPrefix("P2PProxyServerUniversal")
	return sv, nil
}

func (sv *P2PProxyServerUniversal) handleP2PReadFunc(_ *p2p.Client, sourceID []byte, frame []byte, ended bool, logger *webtools.ConsoleLogger) {
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
			switch frame.Operation {
			case webtools.FrameTypeConnect:
				//Get proxy entry
				split := strings.SplitN(string(frame.Data), "|", 2)
				entry, ok := sv.ProxiedServices[split[1]]
				if !ok {
					sv.p2pClient.ClientCoordinator.Logger.Log(3, "Could not create find proxy service entry: "+string(frame.Data))
					return
				}
				frame.ID = []byte(webtools.GenerateRandomID())

				//Create new connection
				if entry.A {
					//UDP Connection
					cl, err := udp.NewClient(entry.C, sv.handleUDPReadFunc, sv.reportTrafic && entry.B)
					cl.Logger.Prefix = "P2PProxyServerUniversal - " + split[1] + " - " + cl.Logger.Prefix
					if err != nil {
						cl.Logger.Log(3, "Could not create connection with ID: "+string(frame.ID)+" to server.")
						return
					}
					cl.UserAttributes["id"] = string(frame.ID)
					cl.Connect()
					sv.idToClient.Set(string(frame.ID), &P2PProxyServerUniversalConn{udpClient: cl, tcpClient: nil, ID: frame.ID, sourceID: sourceID, origin: sv})
				} else {
					//TCP Connection
					cl, err := tcp.NewClientSimple(entry.C, -1, false, sv.handleTCPReadFunc, sv.reportTrafic && entry.B)
					cl.GetLogger().Prefix = "P2PProxyServerUniversal - " + split[1] + " - " + cl.GetLogger().Prefix
					if err != nil {
						cl.GetLogger().Log(3, "Could not create connection with ID: "+string(frame.ID)+" to server.")
						return
					}
					cl.UserAttributes["id"] = string(frame.ID)
					cl.Connect()
					sv.idToClient.Set(string(frame.ID), &P2PProxyServerUniversalConn{udpClient: nil, tcpClient: cl, ID: frame.ID, sourceID: sourceID, origin: sv})
				}
				//sv.clientToID.Set(cl, string(frame.ID))
				sv.idToClient.Get(string(frame.ID)).SendToP2P(webtools.FrameTypeConnect, []byte(split[0]))
				return
			case tcp.MergerFrameTypeListConnections:
				//List connections
				addrs, err := json.Marshal(sv.ProxiedServices)
				if err != nil {
					sv.p2pClient.ClientCoordinator.Logger.Log(3, "Could not create connection list: "+err.Error())
					return
				}
				sv.p2pClient.Send(sourceID, webtools.PackWebtoolsFrame(tcp.MergerFrameTypeListConnections, []byte{0}, addrs))
				return
			}
			logger.Log(3, "Could not find connection to ID: "+string(frame.ID))
			return
		}
		cl := sv.idToClient.Get(string(frame.ID))
		if !cl.IsAlive() {
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
				//Send to local
				cl.SendToLocal(frame.Data)
			}
		}
	}
}

func (sv *P2PProxyServerUniversal) handleUDPReadFunc(udp *udp.Client, _ *net.UDPAddr, data []byte, ended bool) {
	//Get P2P client
	if udp.UserAttributes["id"] == "" || sv.idToClient.Get(udp.UserAttributes["id"]) == nil {
		//Connection does not exists
		udp.Logger.Log(3, "Connection connected to: "+udp.Conn.RemoteAddr().String()+" not found")
		return
	}
	//ID := sv.clientToID.Get(udp)
	cl := sv.idToClient.Get(udp.UserAttributes["id"])

	//End other connection
	if ended {
		cl.Close(true)
	}

	//Send to client
	cl.SendToP2P(webtools.FrameTypeData, data)
}

func (sv *P2PProxyServerUniversal) handleTCPReadFunc(udp *tcp.ClientSimple, data []byte, status uint8) {
	//Get P2P client
	if udp.UserAttributes["id"] == "" || sv.idToClient.Get(udp.UserAttributes["id"]) == nil {
		//Connection does not exists
		udp.GetLogger().Log(3, "Connection connected to: "+udp.GetConn().RemoteAddr().String()+" not found")
		return
	}
	//ID := sv.clientToID.Get(udp)
	cl := sv.idToClient.Get(udp.UserAttributes["id"])

	//End other connection
	if status == webtools.DisconnectStatus {
		cl.Close(true)
	}

	//Send to client
	if status == webtools.ReadDataStatus {
		cl.SendToP2P(webtools.FrameTypeData, data)
	}
}

/*
Start starts P2P Proxy Server for UDP and TCP. Locks execution thread
*/
func (sv *P2PProxyServerUniversal) Start() bool {
	if !sv.p2pClient.ConnectToCoordinator() {
		return false
	}
	for sv.p2pClient.IsAlive() {
		time.Sleep(100 * time.Millisecond)
	}
	return true
}

/*
Stop stops P2P Proxy Server for UDP and TCP
*/
func (sv *P2PProxyServerUniversal) Stop() {
	sv.p2pClient.Stop()
}

/*
SetupUPnP setups UPnP for P2P Client
*/
func (sv *P2PProxyServerUniversal) SetupUPnP(upnp *p2p.UPnPServiceManager) error {
	return sv.p2pClient.SetupUPnP(upnp)
}
