package tcp

import (
	"encoding/json"
	"strconv"
	"webtools"
)

/*
MergerFrameTypeListConnections is frame operator for listing connections
*/
const MergerFrameTypeListConnections = uint8(4)

/*
ConnectionMergerServer is server for merged connections, it join multiple connections into one
*/
type ConnectionMergerServer struct {
	tcpServer          *Server
	idToClient         webtools.SafeMap[string, *ConnectionMergerServerTCPConn]
	clientToID         webtools.SafeMap[*ClientSimple, string]
	tcpServerAddresses []string
	reportTrafic       bool
}

/*
ConnectionMergerServerTCPConn is connection object of ConnectionMergerServer
*/
type ConnectionMergerServerTCPConn struct {
	tcpClient *ClientSimple
	id        []byte
	source    *ServerConn
	origin    *ConnectionMergerServer
}

/*
SendToRemoteTCP creates frame and sends it to remote TCP client
*/
func (cl *ConnectionMergerServerTCPConn) SendToRemoteTCP(operation uint8, data []byte) {
	cl.source.Send(webtools.PackWebtoolsFrame(operation, cl.id, data))
}

/*
SendToLocalTCP sends data to local TCP
*/
func (cl *ConnectionMergerServerTCPConn) SendToLocalTCP(data []byte) {
	cl.tcpClient.Send(data)
}

/*
Close closes connection to client
*/
func (cl *ConnectionMergerServerTCPConn) Close(isInitiator bool) {
	if cl == nil || cl.tcpClient == nil {
		return
	}
	cl.tcpClient.Stop()
	cl.origin.idToClient.Delete(string(cl.id))
	if isInitiator {
		cl.SendToRemoteTCP(webtools.FrameTypeClose, nil)
	}
	cl.origin.clientToID.Delete(cl.tcpClient)
}

/*
NewConnectionMergerServer creates new TCP Connection merger Server but does not starts it
*/
func NewConnectionMergerServer(tcpMergedAddress string, tcpServerAddresses []string, reportTraffic bool) (*ConnectionMergerServer, error) {
	sv := &ConnectionMergerServer{tcpServerAddresses: tcpServerAddresses, clientToID: webtools.MakeSafeMap[*ClientSimple, string](), idToClient: webtools.MakeSafeMap[string, *ConnectionMergerServerTCPConn](), reportTrafic: reportTraffic}
	var err error
	sv.tcpServer, err = NewServer(tcpMergedAddress, sv.handleMergedTCPReadFunc, reportTraffic, true)
	if err != nil {
		return nil, err
	}
	sv.tcpServer.Logger.Prefix = "TCPConnMergerServer - " + sv.tcpServer.Logger.Prefix
	return sv, nil
}

func (sv *ConnectionMergerServer) handleMergedTCPReadFunc(conn *ServerConn, frame []byte, status uint8) {
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
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, conn.origin.Logger) {
		if frame.Operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(frame.ID)) == nil || frame.Operation == webtools.FrameTypeConnect || frame.Operation == MergerFrameTypeListConnections {
			switch frame.Operation {
			case webtools.FrameTypeConnect:
				//Get connection list id
				i, err2 := strconv.Atoi(string(frame.ID))
				if err2 != nil {
					conn.origin.Logger.Log(3, "Could not find server list connection with id: "+string(frame.ID)+". Error: "+err2.Error())
					return
				}

				//Create new connection
				cl, err := NewClientSimple(sv.tcpServerAddresses[i], -1, false, sv.handleLocalTCPReadFunc, sv.reportTrafic)
				frame.ID = []byte(webtools.GenerateRandomID())
				cl.GetLogger().Prefix = "TCPConnMergerServer - " + cl.GetLogger().Prefix
				if err != nil {
					conn.origin.Logger.Log(3, "Could not create connection with id: "+string(frame.ID)+" to server. Error: "+err.Error())
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.ID), &ConnectionMergerServerTCPConn{tcpClient: cl, id: frame.ID, source: conn, origin: sv})
				sv.clientToID.Set(cl, string(frame.ID))
				sv.idToClient.Get(string(frame.ID)).SendToRemoteTCP(webtools.FrameTypeConnect, frame.Data)
				return
			case MergerFrameTypeListConnections:
				//List available connections on server
				addrs, err := json.Marshal(sv.tcpServerAddresses)
				if err != nil {
					conn.origin.Logger.Log(3, "Could not create connection list: "+err.Error())
					return
				}
				conn.Send(webtools.PackWebtoolsFrame(MergerFrameTypeListConnections, []byte{0}, addrs))
				return
			default:
				conn.origin.Logger.Log(3, "Could not find connection to id: "+string(frame.ID))
				return
			}
		}

		cl := sv.idToClient.Get(string(frame.ID))
		if !cl.tcpClient.IsAlive() {
			conn.origin.Logger.Log(3, "Connection with id: "+string(frame.ID)+" connected to: "+conn.GetConn().RemoteAddr().String()+" connected locally to: "+conn.GetConn().LocalAddr().String()+" closed")
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
				//Send to TCP
				cl.SendToLocalTCP(frame.Data)
			}
		}
	}
}

func (sv *ConnectionMergerServer) handleLocalTCPReadFunc(tcp *ClientSimple, data []byte, status uint8) {
	if status == webtools.ConnectStatus {
		return
	}
	//Get TCP remote client
	if sv.clientToID.Get(tcp) == "" || sv.idToClient.Get(sv.clientToID.Get(tcp)) == nil {
		//Connection does not exists
		tcp.GetLogger().Log(3, "Connection connected to: "+tcp.GetConn().RemoteAddr().String()+" not found")
		return
	}
	id := sv.clientToID.Get(tcp)
	cl := sv.idToClient.Get(id)

	//End other connection
	if status == webtools.DisconnectStatus {
		cl.Close(true)
	}

	//Send to client
	cl.SendToRemoteTCP(webtools.FrameTypeData, data)
}

/*
Start starts TCP Connection merger Server. Locks execution thread
*/
func (sv *ConnectionMergerServer) Start() {
	sv.tcpServer.Start()
}

/*
Stop stops TCP Connection merger Server
*/
func (sv *ConnectionMergerServer) Stop() {
	sv.tcpServer.Stop()
}
