package webtools

import (
	"encoding/json"
	"net"
	"slices"
	"strconv"
)

/*
TCP Connection merger client object
*/
type TCPConnectionMergerClient struct {
	clientToId                     SafeMap[*TCPServerConn, string]
	idToClient                     SafeMap[string, *TCPServerConn]
	tcpServers                     []*TCPServer
	tcpClient                      *TCPClient
	pendingConnections             SafeMap[string, *TCPServerConn]
	pendingConnsData               SafeMap[*TCPServerConn, [][]byte]
	tcpServerAddressesToLocalPorts map[string]string
	localServersIPPrefix           string
	reportTrafic                   bool
}

func (cl *TCPConnectionMergerClient) IsAlive() bool {
	return cl.tcpClient.IsAlive()
}

/*
Creates new TCP Connection merger Client but does not starts it
*/
func NewTCPConnectionMergerClient(tcpMergedAddress string, localServersIPPrefix string, tcpServerAddressesToLocalPorts map[string]string, reportTraffic bool) (*TCPConnectionMergerClient, error) {
	cl := &TCPConnectionMergerClient{clientToId: MakeSafeMap[*TCPServerConn, string](), pendingConnections: MakeSafeMap[string, *TCPServerConn](), idToClient: MakeSafeMap[string, *TCPServerConn](), pendingConnsData: MakeSafeMap[*TCPServerConn, [][]byte](), tcpServerAddressesToLocalPorts: tcpServerAddressesToLocalPorts, tcpServers: make([]*TCPServer, 0), localServersIPPrefix: localServersIPPrefix, reportTrafic: reportTraffic}
	var err error
	cl.tcpClient, err = NewTCPClient(tcpMergedAddress, cl.handleRemoteTCPReadFunc, reportTraffic, true)
	if err != nil {
		return nil, err
	}
	cl.tcpClient.Logger.Prefix = "TCPConnMergerClient - " + cl.tcpClient.Logger.Prefix
	//cl.tcpServer.Logger.Prefix = "TCPConnMergerClient - " + cl.tcpServer.Logger.Prefix
	return cl, nil
}

func (cl *TCPConnectionMergerClient) handleRemoteTCPReadFunc(_ *TCPClient, frame []byte, ended bool) {
	if ended {
		//Close all connections
		for i := 0; i < len(cl.tcpServers); i++ {
			cl.tcpServers[i].Stop()
		}
		return
	}

	//Unpack
	for _, frame := range UnpackProxyFrame(frame, cl.tcpClient.Logger) {
		operation, id, data := frame.A, frame.B, frame.C
		if operation == 0 {
			return
		}

		switch operation {
		case TCP_MERGER_FRAME_TYPE_LIST_CONNECTIONS:
			{
				//List remote TCP servers
				var addresses []string
				err := json.Unmarshal(data, &addresses)
				if err != nil {
					cl.tcpClient.Logger.Log(3, "Error unmarshalling server list: "+err.Error())
					return
				}

				//Start local TPC servers
				for i := 0; i < len(addresses); i++ {
					localPort := cl.tcpServerAddressesToLocalPorts[addresses[i]]
					if localPort == "" {
						cl.tcpClient.Logger.Log(3, "No local port found for remote IP address: "+addresses[i]+". Stopping client...")
						cl.Stop()
						return
					}
					addr := net.JoinHostPort(cl.localServersIPPrefix, localPort)
					sv, err := NewTCPServer(addr, cl.handleLocalTCPReadFunc, cl.reportTrafic, false)
					if err != nil {
						cl.tcpClient.Logger.Log(3, "Error creating TCP server for remote IP address: "+addresses[i]+" with local address: "+addr+". Stopping client...")
						cl.Stop()
						return
					}
					sv.Logger.Prefix = "TCPConnMergerClient - " + sv.Logger.Prefix
					cl.tcpServers = append(cl.tcpServers, sv)
					go sv.Start()
				}
				return
			}
		case PROXY_FRAME_TYPE_CONNECT:
			{
				//Confirmed connection
				conn := cl.pendingConnections.Get(string(data))
				if conn == nil {
					cl.tcpClient.Logger.Log(3, "Pending connection with temporary id: "+string(data)+" not found")
					return
				}
				cl.pendingConnections.Delete(string(data))
				cl.clientToId.Set(conn, string(id))
				cl.idToClient.Set(string(id), conn)
				cl.tcpClient.Logger.Log(1, "Prepared new connection with temporary id: "+string(data)+" for connection connected to: "+conn.Conn.RemoteAddr().String()+" connected locally to: "+conn.Conn.LocalAddr().String()+" with new id: "+string(id))

				//Process pending data
				for len(cl.pendingConnsData.Get(conn)) > 0 {
					//Resend data
					cl.tcpClient.Send(PackProxyFrame(PROXY_FRAME_TYPE_DATA, id, cl.pendingConnsData.Get(conn)[0]))
					cl.pendingConnsData.Set(conn, cl.pendingConnsData.Get(conn)[1:])
				}
				cl.pendingConnsData.Delete(conn)
				return
			}
		case PROXY_FRAME_TYPE_CLOSE:
			{
				//Close connection
				cl.idToClient.Get(string(id)).Close()
			}
		case PROXY_FRAME_TYPE_DATA:
			{
				//Resend data
				cl.idToClient.Get(string(id)).Send(data)
			}
		}
	}
}

func (cl *TCPConnectionMergerClient) handleLocalTCPReadFunc(tcpConn *TCPServerConn, data []byte, ended bool) {
	if cl.pendingConnsData.Get(tcpConn) != nil {
		//Already pending connection
		cl.pendingConnsData.Set(tcpConn, append(cl.pendingConnsData.Get(tcpConn), data))
		return
	}

	id := cl.clientToId.Get(tcpConn)
	if id == "" {
		//No connection found, request new
		tempId := GenerateRandomId()
		cl.pendingConnections.Set(tempId, tcpConn)
		cl.tcpClient.Logger.Log(1, "Preparing new connection with temporary id: "+tempId+" for connection connected to: "+tcpConn.Conn.RemoteAddr().String()+" connected locally to: "+tcpConn.Conn.LocalAddr().String())
		cl.tcpClient.Send(PackProxyFrame(PROXY_FRAME_TYPE_CONNECT, []byte(strconv.Itoa(slices.Index(cl.tcpServers, tcpConn.origin))), []byte(tempId)))
		cl.pendingConnsData.Set(tcpConn, append(make([][]byte, 0), data))
		return
	}

	if ended {
		//Connection ended
		cl.tcpClient.Send(PackProxyFrame(PROXY_FRAME_TYPE_CLOSE, []byte(id), nil))
		return
	}
	//Send data
	cl.tcpClient.Send(PackProxyFrame(PROXY_FRAME_TYPE_DATA, []byte(id), data))
}

/*
Connects to TCP Connection merger server and start reading loop, does not locks execution thread
*/
func (cl *TCPConnectionMergerClient) Connect() {
	cl.tcpClient.Connect()
	cl.tcpClient.Send(PackProxyFrame(TCP_MERGER_FRAME_TYPE_LIST_CONNECTIONS, []byte{0}, nil))
}

/*
Stops TCP Connection merger client
*/
func (cl *TCPConnectionMergerClient) Stop() {
	cl.tcpClient.Stop()
	for i := 0; i < len(cl.tcpServers); i++ {
		cl.tcpServers[i].Stop()
	}
}
