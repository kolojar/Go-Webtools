package webtools

import (
	"encoding/json"
	"strconv"
)

const TCP_MERGER_FRAME_TYPE_LIST_CONNECTIONS = uint8(4)

/*
TCP Connection merger server object
*/
type TCPConnectionMergerServer struct {
	tcpServer          *TCPServer
	idToClient         SafeMap[string, *TCPConnectionMergerServerTCPConn]
	clientToId         SafeMap[*TCPClient, string]
	tcpServerAddresses []string
	reportTrafic       bool
}

/*
TCP Connection merger connection object
*/
type TCPConnectionMergerServerTCPConn struct {
	tcpClient *TCPClient
	id        []byte
	source    *TCPServerConn
	origin    *TCPConnectionMergerServer
}

/*
Creates frame and sends it to remote TCP client
*/
func (cl *TCPConnectionMergerServerTCPConn) SendToRemoteTCP(operation uint8, data []byte) {
	cl.source.Send(PackProxyFrame(operation, cl.id, data))
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
		cl.SendToRemoteTCP(PROXY_FRAME_TYPE_CLOSE, nil)
	}
	cl.origin.clientToId.Delete(cl.tcpClient)
}

/*
Creates new TCP Connection merger Server but does not starts it
*/
func NewTCPConnectionMergerServer(tcpMergedAddress string, tcpServerAddresses []string, reportTraffic bool) (*TCPConnectionMergerServer, error) {
	sv := &TCPConnectionMergerServer{tcpServerAddresses: tcpServerAddresses, clientToId: MakeSafeMap[*TCPClient, string](), idToClient: MakeSafeMap[string, *TCPConnectionMergerServerTCPConn](), reportTrafic: reportTraffic}
	var err error
	sv.tcpServer, err = NewTCPServer(tcpMergedAddress, sv.handleMergedTCPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	sv.tcpServer.Logger.Prefix = "TCPConnMergerServer - " + sv.tcpServer.Logger.Prefix
	return sv, nil
}

func (sv *TCPConnectionMergerServer) handleMergedTCPReadFunc(conn *TCPServerConn, frame []byte, ended bool) {
	if ended {
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

	//Unpack frame
	for _, frame := range UnpackProxyFrame(frame, conn.origin.Logger) {
		operation, id, data := frame.A, frame.B, frame.C
		if operation == 0 {
			return
		}

		//Sort connections
		if sv.idToClient.Get(string(id)) == nil || operation == PROXY_FRAME_TYPE_CONNECT || operation == TCP_MERGER_FRAME_TYPE_LIST_CONNECTIONS {
			switch operation {
			case PROXY_FRAME_TYPE_CONNECT:
				//Get connection list id
				i, err2 := strconv.Atoi(string(id))
				if err2 != nil {
					conn.origin.Logger.Log(3, "Could not find server list connection with id: "+string(id)+". Error: "+err2.Error())
					return
				}

				//Create new connection
				cl, err := NewTCPClient(sv.tcpServerAddresses[i], sv.handleLocalTCPReadFunc, sv.reportTrafic)
				id = []byte(GenerateRandomId())
				cl.Logger.Prefix = "TCPConnMergerServer - " + cl.Logger.Prefix
				if err != nil {
					conn.origin.Logger.Log(3, "Could not create connection with id: "+string(id)+" to server. Error: "+err.Error())
					return
				}
				cl.Connect()
				sv.idToClient.Set(string(id), &TCPConnectionMergerServerTCPConn{tcpClient: cl, id: id, source: conn, origin: sv})
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
			conn.origin.Logger.Log(3, "Connection with id: "+string(id)+" connected to: "+conn.Conn.RemoteAddr().String()+" connected locally to: "+conn.Conn.LocalAddr().String()+" closed")
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

func (sv *TCPConnectionMergerServer) handleLocalTCPReadFunc(tcp *TCPClient, data []byte, ended bool) {
	//Get TCP remote client
	if sv.clientToId.Get(tcp) == "" || sv.idToClient.Get(sv.clientToId.Get(tcp)) == nil {
		//Connection does not exists
		tcp.Logger.Log(3, "Connection connected to: "+tcp.address.String()+" not found")
		return
	}
	id := sv.clientToId.Get(tcp)
	cl := sv.idToClient.Get(id)

	//End other connection
	if ended {
		cl.Close(true)
	}

	//Send to client
	cl.SendToRemoteTCP(PROXY_FRAME_TYPE_DATA, data)
}

/*
Starts TCP Connection merger Server
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
