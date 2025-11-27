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
P2PProxyClientUniversal is client for proxied UDP or TCP traffic over P2P
*/
type P2PProxyClientUniversal struct {
	clientToIDUDP                   webtools.SafeMap[*udp.ServerConn, string]
	clientToIDTCP                   webtools.SafeMap[*tcp.ServerConn, string]
	idToClient                      webtools.SafeMap[string, *P2PProxyClientUniversalConn]
	p2pClient                       *p2p.Client
	pendingConnections              webtools.SafeMap[string, *P2PProxyClientUniversalConn]
	pendingConnsDataUDP             webtools.SafeMap[*udp.ServerConn, [][]byte]
	pendingConnsDataTCP             webtools.SafeMap[*tcp.ServerConn, [][]byte]
	p2pServerID                     []byte
	proxiedServicesToLocalAddresses map[string]string
	proxiedServicesFromServer       map[string]webtools.KeyValuePair[bool, string]
	serversUDP                      map[*udp.Server]string
	serversTCP                      map[*tcp.Server]string
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
NewP2PProxyClientUniversal creates new P2P Proxy Client for UDP and TCP but does not starts it
*/
func NewP2PProxyClientUniversal(p2pCoordinatorAddress string, p2pPortForIncommingConns int, p2pProxyServerID []byte, proxiedServicesToLocalAddresses map[string]string, reportTraffic bool) (*P2PProxyClientUniversal, error) {
	cl := &P2PProxyClientUniversal{
		clientToIDUDP:                   webtools.MakeSafeMap[*udp.ServerConn, string](),
		clientToIDTCP:                   webtools.MakeSafeMap[*tcp.ServerConn, string](),
		idToClient:                      webtools.MakeSafeMap[string, *P2PProxyClientUniversalConn](),
		pendingConnections:              webtools.MakeSafeMap[string, *P2PProxyClientUniversalConn](),
		pendingConnsDataUDP:             webtools.MakeSafeMap[*udp.ServerConn, [][]byte](),
		pendingConnsDataTCP:             webtools.MakeSafeMap[*tcp.ServerConn, [][]byte](),
		p2pServerID:                     p2pProxyServerID,
		proxiedServicesToLocalAddresses: proxiedServicesToLocalAddresses,
		reportTraffic:                   reportTraffic,
		serversUDP:                      map[*udp.Server]string{},
		serversTCP:                      map[*tcp.Server]string{},
		proxiedServicesFromServer:       map[string]webtools.KeyValuePair[bool, string]{},
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
		cl.p2pClient.Stop()
		return
	}
	if !bytes.Equal(sourceID, cl.p2pServerID) {
		//InvalID source ID
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
				//List remote TCP servers
				err := json.Unmarshal(frame.Data, &cl.proxiedServicesFromServer)
				if err != nil {
					cl.p2pClient.ClientCoordinator.Logger.Log(3, "Error unmarshalling server list: "+err.Error())
					return
				}

				//Start local servers
				for entryName, entryServerValue := range cl.proxiedServicesFromServer {
					localAddr, ok := cl.proxiedServicesToLocalAddresses[entryName]
					if !ok || localAddr == "" {
						cl.p2pClient.ClientCoordinator.Logger.Log(3, "No local port found for remote IP address: "+entryName+". Stopping client...")
						cl.Stop()
						return
					}

					if entryServerValue.Key {
						//UDP Server
						sv, err := udp.NewServer(localAddr, cl.handleUDPReadFunc, cl.reportTraffic)
						if err != nil {
							cl.p2pClient.ClientCoordinator.Logger.Log(3, "Error creating TCP server for remote IP address: "+entryName+" with local address: "+localAddr+". Stopping client...")
							cl.Stop()
							return
						}
						sv.Logger.Prefix = "P2PProxyClientUniversal - " + entryName + " - " + sv.Logger.Prefix
						cl.serversUDP[sv] = entryName
						go sv.Start()
					} else {
						//TCP Server
						sv, err := tcp.NewServer(localAddr, cl.handleTCPReadFunc, cl.reportTraffic, false)
						if err != nil {
							cl.p2pClient.ClientCoordinator.Logger.Log(3, "Error creating TCP server for remote IP address: "+entryName+" with local address: "+localAddr+". Stopping client...")
							cl.Stop()
							return
						}
						sv.Logger.Prefix = "P2PProxyClientUniversal - " + entryName + " - " + sv.Logger.Prefix
						cl.serversTCP[sv] = entryName
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
				cl.pendingConnections.Delete(string(frame.Data))
				if conn.udpServerConn != nil {
					cl.clientToIDUDP.Set(conn.udpServerConn, string(frame.ID))
				}
				if conn.tcpServerConn != nil {
					cl.clientToIDTCP.Set(conn.tcpServerConn, string(frame.ID))
				}
				cl.idToClient.Set(string(frame.ID), conn)

				//Process pending data
				if conn.udpServerConn != nil {
					cl.p2pClient.ClientCoordinator.Logger.Log(1, "Prepared new connection with temporary ID: "+string(frame.Data)+" for connection connected to: "+conn.udpServerConn.Address.String()+" with new ID: "+string(frame.ID))
					for len(cl.pendingConnsDataUDP.Get(conn.udpServerConn)) > 0 {
						//Resend data
						cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeData, frame.ID, cl.pendingConnsDataUDP.Get(conn.udpServerConn)[0]))
						cl.pendingConnsDataUDP.Set(conn.udpServerConn, cl.pendingConnsDataUDP.Get(conn.udpServerConn)[1:])
					}
					cl.pendingConnsDataUDP.Delete(conn.udpServerConn)
				}
				if conn.tcpServerConn != nil {
					cl.p2pClient.ClientCoordinator.Logger.Log(1, "Prepared new connection with temporary ID: "+string(frame.Data)+" for connection connected to: "+conn.tcpServerConn.Client.GetConn().RemoteAddr().String()+" with new ID: "+string(frame.ID))
					for len(cl.pendingConnsDataTCP.Get(conn.tcpServerConn)) > 0 {
						//Resend data
						cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeData, frame.ID, cl.pendingConnsDataTCP.Get(conn.tcpServerConn)[0]))
						cl.pendingConnsDataTCP.Set(conn.tcpServerConn, cl.pendingConnsDataTCP.Get(conn.tcpServerConn)[1:])
					}
					cl.pendingConnsDataTCP.Delete(conn.tcpServerConn)
				}
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
				fmt.Println("FromP2P:", string(frame.Data))
				cl.idToClient.Get(string(frame.ID)).Send(frame.Data)
			}
		}
	}
}

func (cl *P2PProxyClientUniversal) handleUDPReadFunc(udpConn *udp.ServerConn, data []byte, ended bool) {
	fmt.Println("FromUDP:", string(data))
	if cl.pendingConnsDataUDP.Get(udpConn) != nil {
		//Already pending connection
		fmt.Println("Pending")
		cl.pendingConnsDataUDP.Set(udpConn, append(cl.pendingConnsDataUDP.Get(udpConn), data))
		return
	}

	ID := cl.clientToIDUDP.Get(udpConn)
	if ID == "" {
		//No connection found, request new
		tempID := webtools.GenerateRandomID()
		cl.pendingConnections.Set(tempID, &P2PProxyClientUniversalConn{udpServerConn: udpConn, tcpServerConn: nil, origin: cl})
		cl.p2pClient.ClientCoordinator.Logger.Log(1, "Preparing new connection with temporary ID: "+tempID+" for connection connected to: "+udpConn.Address.String())
		cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeConnect, []byte("0"), []byte(cl.serversUDP[udpConn.GetOrigin()]+string(webtools.FrameSeparatorChar)+tempID)))
		cl.pendingConnsDataUDP.Set(udpConn, append(make([][]byte, 0), data))
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

func (cl *P2PProxyClientUniversal) handleTCPReadFunc(tcpConn *tcp.ServerConn, data []byte, status uint8) {
	fmt.Println("FromTCP:", string(data))
	if cl.pendingConnsDataTCP.Get(tcpConn) != nil {
		//Already pending connection
		cl.pendingConnsDataTCP.Set(tcpConn, append(cl.pendingConnsDataTCP.Get(tcpConn), data))
		return
	}

	ID := cl.clientToIDTCP.Get(tcpConn)
	if ID == "" {
		//No connection found, request new
		tempID := webtools.GenerateRandomID()
		cl.pendingConnections.Set(tempID, &P2PProxyClientUniversalConn{udpServerConn: nil, tcpServerConn: tcpConn, origin: cl})
		cl.p2pClient.ClientCoordinator.Logger.Log(1, "Preparing new connection with temporary ID: "+tempID+" for connection connected to: "+tcpConn.GetConn().RemoteAddr().String())
		cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeConnect, []byte("0"), []byte(cl.serversTCP[tcpConn.GetOrigin()]+string(webtools.FrameSeparatorChar)+tempID)))
		cl.pendingConnsDataTCP.Set(tcpConn, append(make([][]byte, 0), data))
		return
	}

	if status == webtools.DisconnectStatus {
		//Connection ennded
		cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeClose, []byte(ID), nil))
		return
	}
	//Send data
	cl.p2pClient.Send(cl.p2pServerID, webtools.PackWebtoolsFrame(webtools.FrameTypeData, []byte(ID), data))
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
	for k := range cl.serversUDP {
		k.Stop()
	}
	for k := range cl.serversTCP {
		k.Stop()
	}
}

/*
SetupFraming setups UDP framer for P2P client
*/
func (cl *P2PProxyClientUniversal) SetupFramingP2PClient(framer *udp.Framer) {
	cl.p2pClient.SetupFraming(framer)
}

/*
SetupUPnP setups UPnP for P2P Client
*/
func (cl *P2PProxyClientUniversal) SetupUPnP(upnp *p2p.UPnPServiceManager) error {
	return cl.p2pClient.SetupUPnP(upnp)
}
