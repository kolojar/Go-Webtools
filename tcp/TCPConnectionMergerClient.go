package tcp

import (
	"encoding/json"
	"net"
	"slices"
	"strconv"
	"webtools"
)

/*
ConnectionMergerClient is client for merged connections, it splits one connection in multiple ones
*/
type ConnectionMergerClient struct {
	clientToID                     webtools.SafeMap[*ServerConn, string]
	idToClient                     webtools.SafeMap[string, *ServerConn]
	tcpServers                     []*Server
	tcpClient                      *ClientSimple
	pendingConnections             webtools.SafeMap[string, *ServerConn]
	pendingConnsData               webtools.SafeMap[*ServerConn, [][]byte]
	tcpServerAddressesToLocalPorts map[string]string
	localServersIPPrefix           string
	reportTrafic                   bool
}

/*
IsAlive gets if server is alive
*/
func (cl *ConnectionMergerClient) IsAlive() bool {
	return cl.tcpClient.IsAlive()
}

/*
NewConnectionMergerClient creates new TCP Connection merger Client but does not starts it
*/
func NewConnectionMergerClient(tcpMergedAddress string, localServersIPPrefix string, tcpServerAddressesToLocalPorts map[string]string, reportTraffic bool) (*ConnectionMergerClient, error) {
	cl := &ConnectionMergerClient{clientToID: webtools.MakeSafeMap[*ServerConn, string](), pendingConnections: webtools.MakeSafeMap[string, *ServerConn](), idToClient: webtools.MakeSafeMap[string, *ServerConn](), pendingConnsData: webtools.MakeSafeMap[*ServerConn, [][]byte](), tcpServerAddressesToLocalPorts: tcpServerAddressesToLocalPorts, tcpServers: make([]*Server, 0), localServersIPPrefix: localServersIPPrefix, reportTrafic: reportTraffic}
	var err error
	cl.tcpClient, err = NewClientSimple(tcpMergedAddress, 0, false, cl.handleRemoteTCPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.tcpClient.GetLogger().Prefix = "TCPConnMergerClient - " + cl.tcpClient.GetLogger().Prefix
	//cl.tcpServer.Logger.Prefix = "TCPConnMergerClient - " + cl.tcpServer.Logger.Prefix
	return cl, nil
}

func (cl *ConnectionMergerClient) handleRemoteTCPReadFunc(_ *ClientSimple, frame []byte, status uint8) {
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
		case mergerFrameTypeListConnections:
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
					sv, err := NewServer(addr, cl.handleLocalTCPReadFunc, cl.reportTrafic, false)
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
				cl.clientToID.Set(conn, string(frame.Id))
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

func (cl *ConnectionMergerClient) handleLocalTCPReadFunc(tcpConn *ServerConn, data []byte, status uint8) {
	if status == webtools.TCP_CONNECT_STATUS {
		return
	}
	if cl.pendingConnsData.Get(tcpConn) != nil {
		//Already pending connection
		cl.pendingConnsData.Set(tcpConn, append(cl.pendingConnsData.Get(tcpConn), data))
		return
	}

	id := cl.clientToID.Get(tcpConn)
	if id == "" {
		//No connection found, request new
		tempID := webtools.GenerateRandomId()
		cl.pendingConnections.Set(tempID, tcpConn)
		cl.tcpClient.GetLogger().Log(1, "Preparing new connection with temporary id: "+tempID+" for connection connected to: "+tcpConn.GetConn().RemoteAddr().String()+" connected locally to: "+tcpConn.GetConn().LocalAddr().String())
		cl.tcpClient.Send(webtools.PackWebtoolsFrame(webtools.WEBTOOLS_FRAME_TYPE_CONNECT, []byte(strconv.Itoa(slices.Index(cl.tcpServers, tcpConn.origin))), []byte(tempID)))
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
Connect connects to TCP Connection merger server and start reading loop, does not locks execution thread
*/
func (cl *ConnectionMergerClient) Connect() {
	cl.tcpClient.Connect()
	cl.tcpClient.Send(webtools.PackWebtoolsFrame(mergerFrameTypeListConnections, []byte{0}, nil))
}

/*
Stop stops TCP Connection merger client
*/
func (cl *ConnectionMergerClient) Stop() {
	cl.tcpClient.Stop()
	for i := 0; i < len(cl.tcpServers); i++ {
		cl.tcpServers[i].Stop()
	}
}
