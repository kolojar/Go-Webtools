package proxy

import (
	"net"
	"webtools"
	tcptools "webtools/tcp"
	udptools "webtools/udpTools"
)

/*
TCPProxyServerUDP is server for proxied UDP traffic over TCP
*/
type TCPProxyServerUDP struct {
	idToClient       webtools.SafeMap[string, *TCPProxyServerUDPConn]
	clientToID       webtools.SafeMap[*udptools.UDPClient, string]
	tcpServer        *tcptools.Server
	tcpServerAddress string
	reportTrafic     bool
}

/*
TCPProxyServerUDPConn is connection object of TCPProxyServerUDP
*/
type TCPProxyServerUDPConn struct {
	udpClient *udptools.UDPClient
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
		cl.SendToTCP(webtools.WEBTOOLS_FRAME_TYPE_CLOSE, nil)
	}
	cl.origin.clientToID.Delete(cl.udpClient)
}

/*
NewTCPProxyServerUDP creates new TCP Proxy Server for UDP but does not starts it
*/
func NewTCPProxyServerUDP(tcpProxyAddress string, udpServerAddress string, reportTraffic bool) (*TCPProxyServerUDP, error) {
	sv := &TCPProxyServerUDP{tcpServerAddress: udpServerAddress, clientToID: webtools.MakeSafeMap[*udptools.UDPClient, string](), idToClient: webtools.MakeSafeMap[string, *TCPProxyServerUDPConn](), reportTrafic: reportTraffic}
	var err error
	sv.tcpServer, err = tcptools.NewServer(tcpProxyAddress, sv.handleTCPReadFunc, reportTraffic, true)
	if err != nil {
		return nil, err
	}
	sv.tcpServer.Logger.Prefix = "TCPProxyServerUDP - " + sv.tcpServer.Logger.Prefix
	return sv, nil
}

func (sv *TCPProxyServerUDP) handleTCPReadFunc(conn *tcptools.ServerConn, frame []byte, status uint8) {
	if status == webtools.TCP_DISCONNECT_STATUS {
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
	if status != webtools.TCP_READ_DATA_STATUS {
		return
	}

	//Unpack frame
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, conn.Client.GetLogger()) {
		if frame.Operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(frame.Id)) == nil {
			if frame.Operation == webtools.WEBTOOLS_FRAME_TYPE_CONNECT {
				//Create new connection
				frame.Id = []byte(webtools.GenerateRandomId())
				cl, err := udptools.NewUDPClient(sv.tcpServerAddress, sv.handleUDPReadFunc, sv.reportTrafic)
				cl.Logger.Prefix = "TCPProxyServerUDP - " + cl.Logger.Prefix
				if err != nil {
					conn.Client.GetLogger().Log(3, "Could not create connection with id: "+string(frame.Id)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.Id), &TCPProxyServerUDPConn{udpClient: cl, id: frame.Id, source: conn, origin: sv})
				sv.clientToID.Set(cl, string(frame.Id))
				sv.idToClient.Get(string(frame.Id)).SendToTCP(webtools.WEBTOOLS_FRAME_TYPE_CONNECT, frame.Data)
				return
			}
			conn.Client.GetLogger().Log(3, "Could not find connection to id: "+string(frame.Id))
			return
		}
		cl := sv.idToClient.Get(string(frame.Id))
		if !cl.udpClient.IsAlive() {
			conn.Client.GetLogger().Log(3, "Connection with id: "+string(frame.Id)+" connected to: "+conn.GetConn().RemoteAddr().String()+" connected locally to: "+conn.GetConn().LocalAddr().String()+" closed")
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

func (sv *TCPProxyServerUDP) handleUDPReadFunc(udp *udptools.UDPClient, _ *net.UDPAddr, data []byte, ended bool) {
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
	cl.SendToTCP(webtools.WEBTOOLS_FRAME_TYPE_DATA, data)
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
