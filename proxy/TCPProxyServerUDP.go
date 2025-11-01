package proxy

import (
	"net"
	"webtools"
	tcptools "webtools/tcp"
	udptools "webtools/udp"
)

/*
TCPProxyServerUDP is server for proxied UDP traffic over TCP
*/
type TCPProxyServerUDP struct {
	idToClient       webtools.SafeMap[string, *TCPProxyServerUDPConn]
	clientToID       webtools.SafeMap[*udptools.Client, string]
	tcpServer        *tcptools.Server
	tcpServerAddress string
	reportTrafic     bool
}

/*
TCPProxyServerUDPConn is connection object of TCPProxyServerUDP
*/
type TCPProxyServerUDPConn struct {
	udpClient *udptools.Client
	id        []byte
	source    *tcptools.ServerConn
	origin    *TCPProxyServerUDP
}

/*
SendToTCP creates frame and sends it to TCP
*/
func (cl *TCPProxyServerUDPConn) SendToTCP(operation uint8, data []byte) {
	cl.source.Send(webtools.PackWebtoolsFrame(operation, cl.id, data))
}

/*
SendToUDP sends data to UDP
*/
func (cl *TCPProxyServerUDPConn) SendToUDP(data []byte) {
	cl.udpClient.Send(data)
}

/*
Close closes connection to client
*/
func (cl *TCPProxyServerUDPConn) Close(isInitiator bool) {
	if cl == nil || cl.udpClient == nil {
		return
	}
	cl.udpClient.Stop()
	cl.origin.idToClient.Delete(string(cl.id))
	if isInitiator {
		cl.SendToTCP(webtools.FrameTypeClose, nil)
	}
	cl.origin.clientToID.Delete(cl.udpClient)
}

/*
NewTCPProxyServerUDP creates new TCP Proxy Server for UDP but does not starts it
*/
func NewTCPProxyServerUDP(tcpProxyAddress string, udpServerAddress string, reportTraffic bool) (*TCPProxyServerUDP, error) {
	sv := &TCPProxyServerUDP{tcpServerAddress: udpServerAddress, clientToID: webtools.MakeSafeMap[*udptools.Client, string](), idToClient: webtools.MakeSafeMap[string, *TCPProxyServerUDPConn](), reportTrafic: reportTraffic}
	var err error
	sv.tcpServer, err = tcptools.NewServer(tcpProxyAddress, sv.handleTCPReadFunc, reportTraffic, true)
	if err != nil {
		return nil, err
	}
	sv.tcpServer.Logger.Prefix = "TCPProxyServerUDP - " + sv.tcpServer.Logger.Prefix
	return sv, nil
}

func (sv *TCPProxyServerUDP) handleTCPReadFunc(conn *tcptools.ServerConn, frame []byte, status uint8) {
	if status == webtools.DisconnectStatus {
		//Close all connections with this HTTP WebTransport Conn
		for _, d := range sv.idToClient.GetData() {
			if d.Value == nil {
				continue
			}
			if d.Value.source == conn {
				d.Value.Close(true)
			}
		}
		return
	}
	if status != webtools.ReadDataStatus {
		return
	}

	//Unpack frame
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, conn.Client.GetLogger()) {
		if frame.Operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(frame.ID)) == nil {
			if frame.Operation == webtools.FrameTypeConnect {
				//Create new connection
				frame.ID = []byte(webtools.GenerateRandomID())
				cl, err := udptools.NewClient(sv.tcpServerAddress, sv.handleUDPReadFunc, sv.reportTrafic)
				cl.Logger.Prefix = "TCPProxyServerUDP - " + cl.Logger.Prefix
				if err != nil {
					conn.Client.GetLogger().Log(3, "Could not create connection with id: "+string(frame.ID)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.ID), &TCPProxyServerUDPConn{udpClient: cl, id: frame.ID, source: conn, origin: sv})
				sv.clientToID.Set(cl, string(frame.ID))
				sv.idToClient.Get(string(frame.ID)).SendToTCP(webtools.FrameTypeConnect, frame.Data)
				return
			}
			conn.Client.GetLogger().Log(3, "Could not find connection to id: "+string(frame.ID))
			return
		}
		cl := sv.idToClient.Get(string(frame.ID))
		if !cl.udpClient.IsAlive() {
			conn.Client.GetLogger().Log(3, "Connection with id: "+string(frame.ID)+" connected to: "+conn.GetConn().RemoteAddr().String()+" connected locally to: "+conn.GetConn().LocalAddr().String()+" closed")
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

func (sv *TCPProxyServerUDP) handleUDPReadFunc(udp *udptools.Client, _ *net.UDPAddr, data []byte, ended bool) {
	//Get TCP client
	if sv.clientToID.Get(udp) == "" || sv.idToClient.Get(sv.clientToID.Get(udp)) == nil {
		//Connection does not exists
		udp.Logger.Log(3, "Connection connected to: "+udp.Conn.RemoteAddr().String()+" not found")
		return
	}
	id := sv.clientToID.Get(udp)
	cl := sv.idToClient.Get(id)

	//End other connection
	if ended {
		cl.Close(true)
	}

	//Send to client
	cl.SendToTCP(webtools.FrameTypeData, data)
}

/*
Start starts TCP Proxy Server for UDP. Locks execution thread
*/
func (sv *TCPProxyServerUDP) Start() {
	sv.tcpServer.Start()
}

/*
Stop stops TCP Proxy Server for UDP
*/
func (sv *TCPProxyServerUDP) Stop() {
	sv.tcpServer.Stop()
}
