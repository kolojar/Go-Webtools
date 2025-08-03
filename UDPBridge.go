package webtools

type UDPBridge struct {
	clientAddress  string
	udpServer      *UDPServer
	serverToClient map[*UDPServerConn]*UDPClient
	clientToServer map[*UDPClient]*UDPServerConn
}

func NewUDPBridge(sourceAddress string, clientAddress string) *UDPBridge {
	br := &UDPBridge{clientAddress: sourceAddress, serverToClient: map[*UDPServerConn]*UDPClient{}, clientToServer: map[*UDPClient]*UDPServerConn{}}
	br.udpServer, _ = NewUDPServer(clientAddress, br.serverReadFunc, true)
	return br
}

func (br *UDPBridge) serverReadFunc(conn *UDPServerConn, data []byte, ended bool) {
	if br.serverToClient[conn] == nil {
		if ended {
			return
		}

		//No connection found, create new
		cl, _ := NewUDPClient(br.clientAddress, br.clientReadFunc, true)
		br.serverToClient[conn] = cl
		br.clientToServer[cl] = conn
		cl.Connect()
	}
	if ended {
		br.serverToClient[conn].Stop()
		delete(br.clientToServer, br.serverToClient[conn])
		delete(br.serverToClient, conn)
	} else {
		for _, v := range br.serverToClient {
			v.Send(data)
		}
	}
}

func (br *UDPBridge) clientReadFunc(cl *UDPClient, data []byte, ended bool) {
	if br.clientToServer[cl] == nil {
		br.udpServer.logger.Log(3, "Invalid connection")
		return
	}

	if ended {
		br.clientToServer[cl].Close()
	} else {
		br.clientToServer[cl].Send(data)
	}
}

/*
Starts UDP Bridge
*/
func (br *UDPBridge) Start() {
	br.udpServer.Start()
}

/*
Stops UDP Bridge
*/
func (br *UDPBridge) Stop() {
	br.udpServer.Stop()
}
