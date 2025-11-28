package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"webtools"
	"webtools/p2p"
	"webtools/tcp"
	"webtools/udp"
)

/*
P2PProxyClientUniversal is client for proxied UDP and TCP traffic over P2P
*/
type P2PProxyClientUniversal struct {
	idToClient                      webtools.SafeMap[string, *P2PProxyClientUniversalConn]
	udpServers                      webtools.SafeMap[*udp.Server, string]
	tcpServers                      webtools.SafeMap[*tcp.Server, string]
	p2pClient                       *p2p.Client
	pendingConnections              webtools.SafeMap[string, *P2PProxyClientUniversalConn]
	pendingConnsData                webtools.SafeMap[string, [][]byte]
	p2pServerID                     []byte
	proxiedServicesToLocalAddresses map[string]webtools.KeyValuePair[bool, string]
	reportTraffic                   bool
}

/*
P2PProxyClientUniversalConn is connection object of P2PProxyClientUniversal
*/
type P2PProxyClientUniversalConn struct {
	udpServerConn *udp.ServerConn
	tcpServerConn *tcp.ServerConn
	origin        *P2PProxyClientUniversal
}

/*
Close closes connection to client
*/
func (conn *P2PProxyClientUniversalConn) Close() {
	if conn.udpServerConn != nil {
		conn.udpServerConn.Close()
	}
	if conn.tcpServerConn != nil {
		conn.tcpServerConn.Close()
	}
}

/*
Send sends data to UDP or TCP
*/
func (conn *P2PProxyClientUniversalConn) Send(data []byte) {
	if conn.udpServerConn != nil {
		conn.udpServerConn.Send(data)
	}
	if conn.tcpServerConn != nil {
		conn.tcpServerConn.Send(data)
	}
}

/*
IsAlive gets if client is alive
*/
func (cl *P2PProxyClientUniversal) IsAlive() bool {
	return cl.p2pClient.IsAlive()
}

/*
NewP2PProxyClientUniversal creates new P2P Proxy Client for UDP and TCP but does not starts it
Set proxiedServicesToLocalAddresses for translation to local addresses, key is name on server, values are: reportTraffic and localIP
*/
func NewP2PProxyClientUniversal(p2pCoordinatorAddress string, p2pPortForIncommingConns int, p2pProxyServerID []byte, proxiedServicesToLocalAddresses map[string]webtools.KeyValuePair[bool, string], reportTraffic bool) (*P2PProxyClientUniversal, error) {
	cl := &P2PProxyClientUniversal{
		idToClient:                      webtools.MakeSafeMap[string, *P2PProxyClientUniversalConn](),
		pendingConnections:              webtools.MakeSafeMap[string, *P2PProxyClientUniversalConn](),
		pendingConnsData:                webtools.MakeSafeMap[string, [][]byte](),
		udpServers:                      webtools.MakeSafeMap[*udp.Server, string](),
		tcpServers:                      webtools.MakeSafeMap[*tcp.Server, string](),
		p2pServerID:                     p2pProxyServerID,
		reportTraffic:                   reportTraffic,
		proxiedServicesToLocalAddresses: proxiedServicesToLocalAddresses,
	}
	var err error
	cl.p2pClient, err = p2p.NewP2PClient(p2pCoordinatorAddress, p2pPortForIncommingConns, cl.handleP2PReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.p2pClient.SetLoggerPrefix("P2PProxyClientUniversal")
	return cl, nil
}

func (cl *P2PProxyClientUniversal) handleP2PReadFunc(_ *p2p.Client, sourceID []byte, frame []byte, ended bool, logger *webtools.ConsoleLogger) {
	if ended {
		//Close all connections
		cl.Stop()
		return
	}
	if !bytes.Equal(sourceID, cl.p2pServerID) {
		//Invalid source ID
		cl.p2pClient.ClientCoordinator.Logger.Log(3, "Invalid peer source ID: "+string(sourceID))
		return
	}

	//Unpack
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, logger) {
		if frame.Operation == 0 {
			return
		}

		switch frame.Operation {
		case tcp.MergerFrameTypeListConnections:
			{
				//List remote servers
				proxiedServicesFromServer := map[string]webtools.ThreeValuePair[bool, bool, string]{}
				err := json.Unmarshal(frame.Data, &proxiedServicesFromServer)
				if err != nil {
					cl.p2pClient.ClientCoordinator.Logger.Log(3, "Error unmarshalling server list: "+err.Error())
					return
				}

				//Start local servers
				for entryName, entryServerValue := range proxiedServicesFromServer {
					localAddr, ok := cl.proxiedServicesToLocalAddresses[entryName]
					if !ok || localAddr.Value == "" {
						cl.p2pClient.ClientCoordinator.Logger.Log(3, "No local port found for remote IP address: "+entryName+". Stopping client...")
						cl.Stop()
						return
					}

					if entryServerValue.A {
						//UDP Server
						sv, err := udp.NewServer(localAddr.Value, cl.handleUDPReadFunc, cl.reportTraffic && localAddr.Key)
						if err != nil {
							cl.p2pClient.ClientCoordinator.Logger.Log(3, "Error creating TCP server for remote IP address: "+entryName+" with local address: "+localAddr.Value+". Stopping client...")
							cl.Stop()
							return
						}
						sv.Logger.Prefix = "P2PProxyClientUniversal - " + entryName + " - " + sv.Logger.Prefix
						cl.udpServers.Set(sv, entryName)
						go sv.Start()
					} else {
						//TCP Server
						sv, err := tcp.NewServer(localAddr.Value, cl.handleTCPReadFunc, cl.reportTraffic && localAddr.Key, false)
						if err != nil {
							cl.p2pClient.ClientCoordinator.Logger.Log(3, "Error creating TCP server for remote IP address: "+entryName+" with local address: "+localAddr.Value+". Stopping client...")
							cl.Stop()
							return
						}
						sv.Logger.Prefix = "P2PProxyClientUniversal - " + entryName + " - " + sv.Logger.Prefix
						cl.tcpServers.Set(sv, entryName)
						go sv.Start()
					}
				}
				return
			}
		case webtools.FrameTypeConnect:
			{
				//Confirmed connection
				conn := cl.pendingConnections.Get(string(frame.Data))
				if conn == nil {
					cl.p2pClient.ClientCoordinator.Logger.Log(3, "Pending connection with temporary ID: "+string(frame.Data)+" not found")
					return
				}

				//Set IDs
				addr := ""
				if conn.udpServerConn != nil {
					conn.udpServerConn.UserAttributes["id"] = string(frame.ID)
					addr = conn.udpServerConn.Address.String()
				}
				if conn.tcpServerConn != nil {
					conn.tcpServerConn.UserAttributes["id"] = string(frame.ID)
					addr = conn.tcpServerConn.GetConn().RemoteAddr().String()
				}
				cl.idToClient.Set(string(frame.ID), conn)

				//Process pending data
				var pendingLenght = len(cl.pendingConnsData.Get(string(frame.Data)))
				for pendingLenght > 0 {
					//Resend data
					cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeData, frame.ID, cl.pendingConnsData.Get(string(frame.Data))[0]))
					cl.pendingConnsData.Set(string(frame.Data), cl.pendingConnsData.Get(string(frame.Data))[1:])
					if pendingLenght == 1 {
						if conn.udpServerConn != nil {
							conn.udpServerConn.UserAttributes["tempID"] = ""
						}
						if conn.tcpServerConn != nil {
							conn.tcpServerConn.UserAttributes["tempID"] = ""
						}
					}
					pendingLenght = len(cl.pendingConnsData.Get(string(frame.Data)))
				}

				//Remove tempID
				if conn.udpServerConn != nil {
					conn.udpServerConn.UserAttributes["tempID"] = ""
				}
				if conn.tcpServerConn != nil {
					conn.tcpServerConn.UserAttributes["tempID"] = ""
				}
				cl.pendingConnsData.Delete(string(frame.Data))

				//TRYING NEW METHOD TO MOVE THIS AT THE END
				cl.p2pClient.ClientCoordinator.Logger.Log(1, "Prepared new connection with temporary ID: "+string(frame.Data)+" for connection connected to: "+addr+" with new ID: "+string(frame.ID))
				cl.pendingConnections.Delete(string(frame.Data))
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

func (cl *P2PProxyClientUniversal) handleUDPReadFunc(udpConn *udp.ServerConn, data []byte, ended bool) {
	if cl.pendingConnsData.Get(udpConn.UserAttributes["tempID"]) != nil {
		//Already pending connection
		fmt.Println("Pending")
		cl.pendingConnsData.Set(udpConn.UserAttributes["tempID"], append(cl.pendingConnsData.Get(udpConn.UserAttributes["tempID"]), data))
		return
	}

	if udpConn.UserAttributes["id"] == "" {
		//No connection found, request new
		tempID := webtools.GenerateRandomID()
		udpConn.UserAttributes["tempID"] = tempID
		cl.pendingConnections.Set(tempID, &P2PProxyClientUniversalConn{udpServerConn: udpConn, tcpServerConn: nil, origin: cl})
		cl.p2pClient.ClientCoordinator.Logger.Log(1, "Preparing new connection with temporary ID: "+tempID+" for connection connected to: "+udpConn.Address.String())
		cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeConnect, []byte("0"), []byte(tempID+"|"+cl.udpServers.Get(udpConn.GetOrigin()))))
		cl.pendingConnsData.Set(udpConn.UserAttributes["tempID"], append(make([][]byte, 0), data))
		return
	}

	if ended {
		//Connection ennded
		cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeClose, []byte(udpConn.UserAttributes["id"]), nil))
		return
	}
	//Send data
	cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeData, []byte(udpConn.UserAttributes["id"]), data))
}

func (cl *P2PProxyClientUniversal) handleTCPReadFunc(tcpConn *tcp.ServerConn, data []byte, status uint8) {
	if cl.pendingConnsData.Get(tcpConn.UserAttributes["tempID"]) != nil {
		//Already pending connection
		cl.pendingConnsData.Set(tcpConn.UserAttributes["tempID"], append(cl.pendingConnsData.Get(tcpConn.UserAttributes["tempID"]), data))
		return
	}

	if tcpConn.UserAttributes["id"] == "" {
		//No connection found, request new
		tempID := webtools.GenerateRandomID()
		tcpConn.UserAttributes["tempID"] = tempID
		cl.pendingConnections.Set(tempID, &P2PProxyClientUniversalConn{udpServerConn: nil, tcpServerConn: tcpConn, origin: cl})
		cl.p2pClient.ClientCoordinator.Logger.Log(1, "Preparing new connection with temporary ID: "+tempID+" for connection connected to: "+tcpConn.GetConn().RemoteAddr().String())
		cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeConnect, []byte("0"), []byte(tempID+"|"+cl.tcpServers.Get(tcpConn.GetOrigin()))))
		if status != webtools.ReadDataStatus {
			cl.pendingConnsData.Set(tcpConn.UserAttributes["tempID"], make([][]byte, 0))
		} else {
			cl.pendingConnsData.Set(tcpConn.UserAttributes["tempID"], append(make([][]byte, 0), data))
		}
		return
	}

	if status == webtools.DisconnectStatus {
		//Connection ennded
		cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeClose, []byte(tcpConn.UserAttributes["id"]), nil))
		return
	}
	if status == webtools.ReadDataStatus {
		//Send data
		cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeData, []byte(tcpConn.UserAttributes["id"]), data))
	}
}

/*
Connect connects to P2P Proxy server and start reading loop, does not lock execution thread
*/
func (cl *P2PProxyClientUniversal) Connect() bool {
	if !cl.p2pClient.ConnectToCoordinator() {
		return false
	}
	if !cl.p2pClient.ConnectToPeer(cl.p2pServerID) {
		return false
	}
	cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(tcp.MergerFrameTypeListConnections, []byte{0}, nil))
	return true
}

/*
Stop stops P2P Proxy client
*/
func (cl *P2PProxyClientUniversal) Stop() {
	cl.p2pClient.Stop()
	for _, k := range cl.udpServers.GetKeys() {
		k.Stop()
	}
	for _, k := range cl.tcpServers.GetKeys() {
		k.Stop()
	}
}

/*
SetupUPnP setups UPnP for P2P Client
*/
func (cl *P2PProxyClientUniversal) SetupUPnP(upnp *p2p.UPnPServiceManager) error {
	return cl.p2pClient.SetupUPnP(upnp)
}
