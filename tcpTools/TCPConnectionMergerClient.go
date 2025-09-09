package tcptools

import (
	"encoding/json"
	"net"
	"slices"
	"strconv"
	"webtools"
)

/*
TCP Connection merger client object
*/
type TCPConnectionMergerClient struct {
	clientToId                     webtools.SafeMap[*TCPServerConn, string]
	idToClient                     webtools.SafeMap[string, *TCPServerConn]
	tcpServers                     []*TCPServer
	tcpClient                      *TCPClientSimple
	pendingConnections             webtools.SafeMap[string, *TCPServerConn]
	pendingConnsData               webtools.SafeMap[*TCPServerConn, [][]byte]
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
	cl := &TCPConnectionMergerClient{clientToId: webtools.MakeSafeMap[*TCPServerConn, string](), pendingConnections: webtools.MakeSafeMap[string, *TCPServerConn](), idToClient: webtools.MakeSafeMap[string, *TCPServerConn](), pendingConnsData: webtools.MakeSafeMap[*TCPServerConn, [][]byte](), tcpServerAddressesToLocalPorts: tcpServerAddressesToLocalPorts, tcpServers: make([]*TCPServer, 0), localServersIPPrefix: localServersIPPrefix, reportTrafic: reportTraffic}
	var err error
	cl.tcpClient, err = NewTCPClientSimple(tcpMergedAddress, 0, false, cl.handleRemoteTCPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.tcpClient.GetLogger().Prefix = "TCPConnMergerClient - " + cl.tcpClient.GetLogger().Prefix
	//cl.tcpServer.Logger.Prefix = "TCPConnMergerClient - " + cl.tcpServer.Logger.Prefix
	return cl, nil
}

func (cl *TCPConnectionMergerClient) handleRemoteTCPReadFunc(_ *TCPClientSimple, frame []byte, status uint8) {
	if status == webtools.TCP_DISCONNECT_STATUS {
		//Close all connections
		for i := 0; i < len(cl.tcpServers); i++ {
			cl.tcpServers[i].Stop()
		}
		return
	}
	if status != webtools.TCP_READ_DATA_STATUS {
		return
	}

	//Unpack
	for _, frame := range webtools.UnpackWebtoolsFrame(frame, cl.tcpClient.GetLogger()) {
		if frame.Operation == 0 {
			return
		}

		switch frame.Operation {
		case TCP_MERGER_FRAME_TYPE_LIST_CONNECTIONS:
			{
				//List remote TCP servers
				var addresses []string
				err := json.Unmarshal(frame.Data, &addresses)
				if err != nil {
					cl.tcpClient.GetLogger().Log(3, "Error unmarshalling server list: "+err.Error())
					return
				}

				//Start local TPC servers
				for i := 0; i < len(addresses); i++ {
					localPort := cl.tcpServerAddressesToLocalPorts[addresses[i]]
					if localPort == "" {
						cl.tcpClient.GetLogger().Log(3, "No local port found for remote IP address: "+addresses[i]+". Stopping client...")
						cl.Stop()
						return
					}
					addr := net.JoinHostPort(cl.localServersIPPrefix, localPort)
					sv, err := NewTCPServer(addr, cl.handleLocalTCPReadFunc, cl.reportTrafic, false)
					if err != nil {
						cl.tcpClient.GetLogger().Log(3, "Error creating TCP server for remote IP address: "+addresses[i]+" with local address: "+addr+". Stopping client...")
						cl.Stop()
						return
					}
					sv.Logger.Prefix = "TCPConnMergerClient - " + sv.Logger.Prefix
					cl.tcpServers = append(cl.tcpServers, sv)
					go sv.Start()
				}
				return
			}
		case webtools.WEBTOOLS_FRAME_TYPE_CONNECT:
			{
				//Confirmed connection
				conn := cl.pendingConnections.Get(string(frame.Data))
				if conn == nil {
					cl.tcpClient.GetLogger().Log(3, "Pending connection with temporary id: "+string(frame.Data)+" not found")
					return
				}
				cl.pendingConnections.Delete(string(frame.Data))
				cl.clientToId.Set(conn, string(frame.Id))
				cl.idToClient.Set(string(frame.Id), conn)
				cl.tcpClient.GetLogger().Log(1, "Prepared new connection with temporary id: "+string(frame.Data)+" for connection connected to: "+conn.GetConn().RemoteAddr().String()+" connected locally to: "+conn.GetConn().LocalAddr().String()+" with new id: "+string(frame.Id))

				//Process pending data
				for len(cl.pendingConnsData.Get(conn)) > 0 {
					//Resend data
					cl.tcpClient.Send(webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_DATA, frame.Id, cl.pendingConnsData.Get(conn)[0]))
					cl.pendingConnsData.Set(conn, cl.pendingConnsData.Get(conn)[1:])
				}
				cl.pendingConnsData.Delete(conn)
				return
			}
		case webtools.WEBTOOLS_FRAME_TYPE_CLOSE:
			{
				//Close connection
				cl.idToClient.Get(string(frame.Id)).Close()
			}
		case webtools.WEBTOOLS_FRAME_TYPE_DATA:
			{
				//Resend data
				cl.idToClient.Get(string(frame.Id)).Send(frame.Data)
			}
		}
	}
}

func (cl *TCPConnectionMergerClient) handleLocalTCPReadFunc(tcpConn *TCPServerConn, data []byte, status uint8) {
	if status == webtools.TCP_CONNECT_STATUS {
		return
	}
	if cl.pendingConnsData.Get(tcpConn) != nil {
		//Already pending connection
		cl.pendingConnsData.Set(tcpConn, append(cl.pendingConnsData.Get(tcpConn), data))
		return
	}

	id := cl.clientToId.Get(tcpConn)
	if id == "" {
		//No connection found, request new
		tempId := webtools.GenerateRandomId()
		cl.pendingConnections.Set(tempId, tcpConn)
		cl.tcpClient.GetLogger().Log(1, "Preparing new connection with temporary id: "+tempId+" for connection connected to: "+tcpConn.GetConn().RemoteAddr().String()+" connected locally to: "+tcpConn.GetConn().LocalAddr().String())
		cl.tcpClient.Send(webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_CONNECT, []byte(strconv.Itoa(slices.Index(cl.tcpServers, tcpConn.origin))), []byte(tempId)))
		cl.pendingConnsData.Set(tcpConn, append(make([][]byte, 0), data))
		return
	}

	if status == webtools.TCP_DISCONNECT_STATUS {
		//Connection ended
		cl.tcpClient.Send(webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_CLOSE, []byte(id), nil))
		return
	}
	//Send data
	cl.tcpClient.Send(webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_DATA, []byte(id), data))
}

/*
Connects to TCP Connection merger server and start reading loop, does not locks execution thread
*/
func (cl *TCPConnectionMergerClient) Connect() {
	cl.tcpClient.Connect()
	cl.tcpClient.Send(webtools.PackWebtoolsFrame(TCP_MERGER_FRAME_TYPE_LIST_CONNECTIONS, []byte{0}, nil))
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
