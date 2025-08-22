package tcptools

import (
	"encoding/json"
	"strconv"
	"webtools"
	proxytools "webtools/proxyTools"
)

const TCP_MERGER_FRAME_TYPE_LIST_CONNECTIONS = uint8(4)

/*
TCP Connection merger server object
*/
type TCPConnectionMergerServer struct {
	tcpServer          *TCPServer
	idToClient         webtools.SafeMap[string, *TCPConnectionMergerServerTCPConn]
	clientToId         webtools.SafeMap[*TCPClientSimple, string]
	tcpServerAddresses []string
	reportTrafic       bool
}

/*
TCP Connection merger connection object
*/
type TCPConnectionMergerServerTCPConn struct {
	tcpClient *TCPClientSimple
	id        []byte
	source    *TCPServerConn
	origin    *TCPConnectionMergerServer
}

/*
Creates frame and sends it to remote TCP client
*/
func (cl *TCPConnectionMergerServerTCPConn) SendToRemoteTCP(operation uint8, data []byte) {
	cl.source.Send(proxytools.PackProxyFrame(operation, cl.id, data))
}

/*
Sends data to local TCP
*/
func (cl *TCPConnectionMergerServerTCPConn) SendToLocalTCP(data []byte) {
	cl.tcpClient.Send(data)
}

/*
Closes connection to client
*/
func (cl *TCPConnectionMergerServerTCPConn) Close(isInitiator bool) {
	if cl == nil || cl.tcpClient == nil {
		return
	}
	cl.tcpClient.Stop()
	cl.origin.idToClient.Delete(string(cl.id))
	if isInitiator {
		cl.SendToRemoteTCP(proxytools.PROXY_FRAME_TYPE_CLOSE, nil)
	}
	cl.origin.clientToId.Delete(cl.tcpClient)
}

/*
Creates new TCP Connection merger Server but does not starts it
*/
func NewTCPConnectionMergerServer(tcpMergedAddress string, tcpServerAddresses []string, reportTraffic bool) (*TCPConnectionMergerServer, error) {
	sv := &TCPConnectionMergerServer{tcpServerAddresses: tcpServerAddresses, clientToId: webtools.MakeSafeMap[*TCPClientSimple, string](), idToClient: webtools.MakeSafeMap[string, *TCPConnectionMergerServerTCPConn](), reportTrafic: reportTraffic}
	var err error
	sv.tcpServer, err = NewTCPServer(tcpMergedAddress, sv.handleMergedTCPReadFunc, reportTraffic, true)
	if err != nil {
		return nil, err
	}
	sv.tcpServer.Logger.Prefix = "TCPConnMergerServer - " + sv.tcpServer.Logger.Prefix
	return sv, nil
}

func (sv *TCPConnectionMergerServer) handleMergedTCPReadFunc(conn *TCPServerConn, frame []byte, status uint8) {
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
	for _, frame := range proxytools.UnpackProxyFrame(frame, conn.origin.Logger) {
		if frame.Operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(frame.Id)) == nil || frame.Operation == proxytools.PROXY_FRAME_TYPE_CONNECT || frame.Operation == TCP_MERGER_FRAME_TYPE_LIST_CONNECTIONS {
			switch frame.Operation {
			case proxytools.PROXY_FRAME_TYPE_CONNECT:
				//Get connection list id
				i, err2 := strconv.Atoi(string(frame.Id))
				if err2 != nil {
					conn.origin.Logger.Log(3, "Could not find server list connection with id: "+string(frame.Id)+". Error: "+err2.Error())
					return
				}

				//Create new connection
				cl, err := NewTCPClientSimple(sv.tcpServerAddresses[i], -1, false, sv.handleLocalTCPReadFunc, sv.reportTrafic)
				frame.Id = []byte(webtools.GenerateRandomId())
				cl.GetLogger().Prefix = "TCPConnMergerServer - " + cl.GetLogger().Prefix
				if err != nil {
					conn.origin.Logger.Log(3, "Could not create connection with id: "+string(frame.Id)+" to server. Error: "+err.Error())
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(frame.Id), &TCPConnectionMergerServerTCPConn{tcpClient: cl, id: id, source: conn, origin: sv})
				sv.clientToId.Set(cl, string(id))
				sv.idToClient.Get(string(id)).SendToRemoteTCP(PROXY_FRAME_TYPE_CONNECT, data)
				return
			case TCP_MERGER_FRAME_TYPE_LIST_CONNECTIONS:
				//List available connections on server
				addrs, err := json.Marshal(sv.tcpServerAddresses)
				if err != nil {
					conn.origin.Logger.Log(3, "Could not create connection list: "+err.Error())
					return
				}
				conn.Send(PackProxyFrame(TCP_MERGER_FRAME_TYPE_LIST_CONNECTIONS, []byte{0}, addrs))
				return
			default:
				conn.origin.Logger.Log(3, "Could not find connection to id: "+string(id))
				return
			}
		}

		cl := sv.idToClient.Get(string(id))
		if !cl.tcpClient.IsAlive() {
			conn.origin.Logger.Log(3, "Connection with id: "+string(id)+" connected to: "+conn.GetConn().RemoteAddr().String()+" connected locally to: "+conn.GetConn().LocalAddr().String()+" closed")
			return
		}

		//Sort operations
		switch operation {
		case PROXY_FRAME_TYPE_CLOSE:
			{
				//Close connection
				cl.Close(false)
			}
		case PROXY_FRAME_TYPE_DATA:
			{
				//Send to TCP
				cl.SendToLocalTCP(data)
			}
		}
	}
}

func (sv *TCPConnectionMergerServer) handleLocalTCPReadFunc(tcp *TCPClientSimple, data []byte, status uint8) {
	if status == webtools.TCP_CONNECT_STATUS {
		return
	}
	//Get TCP remote client
	if sv.clientToId.Get(tcp) == "" || sv.idToClient.Get(sv.clientToId.Get(tcp)) == nil {
		//Connection does not exists
		tcp.GetLogger().Log(3, "Connection connected to: "+tcp.GetConn().RemoteAddr().String()+" not found")
		return
	}
	id := sv.clientToId.Get(tcp)
	cl := sv.idToClient.Get(id)

	//End other connection
	if status == webtools.TCP_DISCONNECT_STATUS {
		cl.Close(true)
	}

	//Send to client
	cl.SendToRemoteTCP(PROXY_FRAME_TYPE_DATA, data)
}

/*
Starts TCP Connection merger Server. Locks execution thread
*/
func (sv *TCPConnectionMergerServer) Start() {
	sv.tcpServer.Start()
}

/*
Stops TCP Connection merger Server
*/
func (sv *TCPConnectionMergerServer) Stop() {
	sv.tcpServer.Stop()
}
