package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
	"webtools"
	"webtools/p2p"
	"webtools/tcp"
	"webtools/udp"
)

/*
P2PProxyServerUniversal is universal P2P server for TCP and UDP, it provides support for multiple proxies per one peer id
*/
type P2PProxyServerUniversal struct {
	idToClient    webtools.SafeMap[string, *P2PProxyServerUniversalConn]
	clientTCPToID webtools.SafeMap[*tcp.ClientSimple, string]
	clientUDPToID webtools.SafeMap[*udp.Client, string]
	p2pClient     *p2p.Client
	reportTrafic  bool
	//Key is service name, values are isUDP and address
	ProxiedServices map[string]webtools.KeyValuePair[bool, string]
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
SendToP2P creates frame and sends it to P2P
*/
func (conn *P2PProxyServerUniversalConn) SendToP2P(operation uint8, data []byte) {
	conn.origin.p2pClient.Send(conn.sourceID, webtools.PackWebtoolsFrame(operation, conn.ID, data))
}

/*
SendToLocalConnection sends data to UDP or TCP
*/
func (conn *P2PProxyServerUniversalConn) SendToLocalConnection(data []byte) {
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

	//Send to P2P
	conn.origin.idToClient.Delete(string(conn.ID))
	if isInitiator {
		conn.SendToP2P(webtools.FrameTypeClose, nil)
	}

	if conn.tcpClient != nil {
		//Stop TCP
		conn.tcpClient.Stop()
		conn.origin.clientTCPToID.Delete(conn.tcpClient)
	}
	if conn.udpClient != nil {
		//Stop UDP
		conn.udpClient.Stop()
		conn.origin.clientUDPToID.Delete(conn.udpClient)
	}
}

/*
IsAlive returns if one of the clients is alive
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
NewP2PProxyServerUniversal creates new P2P Proxy Server for ProxiedServices but does not starts it. To proxy service set it in ProxiedServices
*/
func NewP2PProxyServerUniversal(p2pCoordinatorAddress string, p2pPortForIncommingConns int, reportTraffic bool) (*P2PProxyServerUniversal, error) {
	sv := &P2PProxyServerUniversal{
		ProxiedServices: map[string]webtools.KeyValuePair[bool, string]{},
		clientTCPToID:   webtools.MakeSafeMap[*tcp.ClientSimple, string](),
		clientUDPToID:   webtools.MakeSafeMap[*udp.Client, string](),
		idToClient:      webtools.MakeSafeMap[string, *P2PProxyServerUniversalConn](),
		reportTrafic:    reportTraffic,
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
		//Close all connections with this P2P Conn - ISNT THIS AGAINST P2P LOGIC - WHEN MAIN SERVER DIES, IT KILLS ALL OTHERS
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
				split := strings.Split(string(frame.Data), string(webtools.FrameSeparatorChar))
				entry, ok := sv.ProxiedServices[split[0]]
				if !ok {
					sv.p2pClient.ClientCoordinator.Logger.Log(3, "Could not create find proxy service entry: "+string(frame.Data))
					return
				}

				//Create new connection
				frame.ID = []byte(webtools.GenerateRandomID())
				if entry.Key {
					//UDP Connection
					cl, err := udp.NewClient(entry.Value, sv.handleUDPReadFunc, sv.reportTrafic)
					cl.Logger.Prefix = "P2PProxyServerUniversal - " + cl.Logger.Prefix
					if err != nil {
						cl.Logger.Log(3, "Could not create connection with ID: "+string(frame.ID)+" to server.")
						return
					}
					cl.Connect()
					sv.idToClient.Set(string(frame.ID), &P2PProxyServerUniversalConn{udpClient: cl, ID: frame.ID, sourceID: sourceID, origin: sv, tcpClient: nil})
					sv.clientUDPToID.Set(cl, string(frame.ID))
				} else {
					//TCP Connection
					cl, err := tcp.NewClientSimple(entry.Value, -1, false, sv.handleTCPReadFunc, sv.reportTrafic)
					cl.GetLogger().Prefix = "P2PProxyServerUniversal - " + cl.GetLogger().Prefix
					if err != nil {
						cl.GetLogger().Log(3, "Could not create connection with ID: "+string(frame.ID)+" to server.")
						return
					}
					cl.Connect()
					sv.idToClient.Set(string(frame.ID), &P2PProxyServerUniversalConn{tcpClient: cl, ID: frame.ID, sourceID: sourceID, origin: sv, udpClient: nil})
					sv.clientTCPToID.Set(cl, string(frame.ID))
				}

				//Send data using P2P
				sv.idToClient.Get(string(frame.ID)).SendToP2P(webtools.FrameTypeConnect, []byte(split[1]))
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
				//Send to UDP
				fmt.Println("FromP2P:", string(frame.Data))
				cl.SendToLocalConnection(frame.Data)
			}
		}
	}
}

func (sv *P2PProxyServerUniversal) handleUDPReadFunc(udp *udp.Client, _ *net.UDPAddr, data []byte, ended bool) {
	fmt.Println("FromUDP:", string(data))
	//Get P2P client
	if sv.clientUDPToID.Get(udp) == "" || sv.idToClient.Get(sv.clientUDPToID.Get(udp)) == nil {
		//Connection does not exists
		udp.Logger.Log(3, "Connection connected to: "+udp.Conn.RemoteAddr().String()+" not found")
		return
	}
	ID := sv.clientUDPToID.Get(udp)
	cl := sv.idToClient.Get(ID)

	//End other connection
	if ended {
		cl.Close(true)
	}

	//Send to client
	cl.SendToP2P(webtools.FrameTypeData, data)
}

func (sv *P2PProxyServerUniversal) handleTCPReadFunc(tcp *tcp.ClientSimple, data []byte, status uint8) {
	fmt.Println("FromTCP:", string(data))
	//Get P2P client
	if sv.clientTCPToID.Get(tcp) == "" || sv.idToClient.Get(sv.clientTCPToID.Get(tcp)) == nil {
		//Connection does not exists
		tcp.GetLogger().Log(3, "Connection connected to: "+tcp.GetConn().RemoteAddr().String()+" not found")
		return
	}
	ID := sv.clientTCPToID.Get(tcp)
	cl := sv.idToClient.Get(ID)

	//End other connection
	if status == webtools.DisconnectStatus {
		cl.Close(true)
	}

	//Send to client
	cl.SendToP2P(webtools.FrameTypeData, data)
}

/*
Start starts P2P Proxy Server for UDP or TCP. Locks execution thread
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
Stop stops P2P Proxy Server for UDP or TCP
*/
func (sv *P2PProxyServerUniversal) Stop() {
	sv.p2pClient.Stop()
}

/*
SetupFraming setups UDP framer for P2P client
*/
func (sv *P2PProxyServerUniversal) SetupFramingP2PClient(framer *udp.Framer) {
	sv.p2pClient.SetupFraming(framer)
}

/*
SetupUPnP setups UPnP for P2P Client
*/
func (sv *P2PProxyServerUniversal) SetupUPnP(upnp *p2p.UPnPServiceManager) error {
	return sv.p2pClient.SetupUPnP(upnp)
}
