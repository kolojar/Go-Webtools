package proxyTools

import (
	"bytes"
	"net"
	"time"
	"webtools"
	"webtools/p2pTools"
	"webtools/udpTools"
)

/*
P2P Proxy server for UDP object
*/
type P2PProxyServerUDP struct {
	idToClient       webtools.SafeMap[string, *P2PProxyServerUDPConn]
	clientToId       webtools.SafeMap[*udpTools.UDPClient, string]
	p2pClient        *p2pTools.P2PClientUDP
	udpServerAddress string
	reportTrafic     bool
}

/*
P2P Proxy server for UDP connection object
*/
type P2PProxyServerUDPConn struct {
	udpClient *udpTools.UDPClient
	id        []byte
	sourceId  []byte
	origin    *P2PProxyServerUDP
}

/*
Creates frame and sends it to P2P
*/
func (cl *P2PProxyServerUDPConn) SendToP2P(operation uint8, data []byte) {
	cl.origin.p2pClient.Send(cl.sourceId, webtools.PackWebtoolsFrame(operation, cl.id, data))
}

/*
Creates frame and sends it to UDP
*/
func (cl *P2PProxyServerUDPConn) SendToUDP(data []byte) {
	cl.udpClient.Send(data)
}

/*
Closes connection to client
*/
func (cl *P2PProxyServerUDPConn) Close(isInitiator bool) {
	if cl == nil || cl.udpClient == nil {
		return
	}
	cl.udpClient.Stop()
	cl.origin.idToClient.Delete(string(cl.id))
	if isInitiator {
		cl.SendToP2P(webtools.WEBTOOLS_FRAME_TYPE_CLOSE, nil)
	}
	cl.origin.clientToId.Delete(cl.udpClient)
}

/*
Creates new P2P Proxy Server for UDP but does not starts it
*/
func NewP2PProxyServerUDP(p2pCoordinatorAddress string, p2pPortForIncommingConns int, udpServerAddress string, reportTraffic bool) (*P2PProxyServerUDP, error) {
	sv := &P2PProxyServerUDP{
		udpServerAddress: udpServerAddress,
		clientToId:       webtools.MakeSafeMap[*udpTools.UDPClient, string](),
		idToClient:       webtools.MakeSafeMap[string, *P2PProxyServerUDPConn](),
		reportTrafic:     reportTraffic,
	}
	var err error
	sv.p2pClient, err = p2pTools.NewP2PClientUDP(p2pCoordinatorAddress, p2pPortForIncommingConns, sv.handleP2PReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	sv.p2pClient.SetLoggerPrefix("P2PProxyServerUDP")
	return sv, nil
}

func (sv *P2PProxyServerUDP) handleP2PReadFunc(_ *p2pTools.P2PClientUDP, sourceId []byte, frame []byte, ended bool, logger *webtools.ConsoleLogger) {
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
				cl, err := udpTools.NewUDPClient(sv.udpServerAddress, sv.handleUDPReadFunc, sv.reportTrafic)
				cl.Logger.Prefix = "P2PProxyServerUDP - " + cl.Logger.Prefix
				if err != nil {
					cl.Logger.Log(3, "Could not create connection with id: "+string(frame.Id)+" to server.")
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.Id), &P2PProxyServerUDPConn{udpClient: cl, id: frame.Id, sourceId: sourceId, origin: sv})
				sv.clientToId.Set(cl, string(frame.Id))
				sv.idToClient.Get(string(frame.Id)).SendToP2P(webtools.WEBTOOLS_FRAME_TYPE_CONNECT, frame.Data)
				return
			} else {
				logger.Log(3, "Could not find connection to id: "+string(frame.Id))
				return
			}
		}
		cl := sv.idToClient.Get(string(frame.Id))
		if !cl.udpClient.IsAlive() {
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

func (sv *P2PProxyServerUDP) handleUDPReadFunc(udp *udpTools.UDPClient, sourceAddress *net.UDPAddr, data []byte, ended bool) {
	//Get P2P client
	if sv.clientToId.Get(udp) == "" || sv.idToClient.Get(sv.clientToId.Get(udp)) == nil {
		//Connection does not exists
		udp.Logger.Log(3, "Connection connected to: "+udp.Conn.RemoteAddr().String()+" not found")
		return
	}
	id := sv.clientToId.Get(udp)
	cl := sv.idToClient.Get(id)

	//End other connection
	if ended {
		cl.Close(true)
	}

	//Send to client
	cl.SendToP2P(webtools.WEBTOOLS_FRAME_TYPE_DATA, data)
}

/*
Starts P2P Proxy Server for UDP. Locks execution thread
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
Stops P2P Proxy Server for UDP
*/
func (sv *P2PProxyServerUDP) Stop() {
	sv.p2pClient.Stop()
}
