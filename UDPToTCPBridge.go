package webtools

/*
UDP to TCP bridge used for converting all incoming UDP traffic to TCP
*/
type UDPToTCPBridge struct {
	tcpServer        *TCPServer
	tcpToUDP         map[*TCPServerConn]*UDPClient
	udpToTCP         map[*UDPClient]*TCPServerConn
	udpServerAddress string
	reportTraffic    bool
}

/*
Creates new UDP to TCP bridge but does not starts it
*/
func NewUDPToTCPBridge(udpServerAddress string, tcpServerAddress string, reportTraffic bool) (*UDPToTCPBridge, error) {
	br := &UDPToTCPBridge{udpToTCP: map[*UDPClient]*TCPServerConn{}, tcpToUDP: map[*TCPServerConn]*UDPClient{}, udpServerAddress: udpServerAddress, reportTraffic: reportTraffic}
	var err error
	br.tcpServer, err = NewTCPServer(tcpServerAddress, br.handleTCPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	br.tcpServer.Logger.Prefix = "UDPToTCPBridge - " + br.tcpServer.Logger.Prefix
	return br, nil
}

func (br *UDPToTCPBridge) handleTCPReadFunc(tcp *TCPServerConn, data []byte, ended bool) {
	udp := br.tcpToUDP[tcp]
	if ended {
		//Connection ended
		if udp != nil {
			udp.Stop()
			delete(br.udpToTCP, udp)
		}
		delete(br.tcpToUDP, tcp)
		return
	}

	if udp == nil {
		//No connection to UDP found, creating new
		var err error
		udp, err = NewUDPClient(br.udpServerAddress, br.handleUDPReadFunc, br.reportTraffic)
		if err != nil {
			br.tcpServer.Logger.Log(3, "Error creating new UDP connection to: "+br.udpServerAddress+" | Error: "+err.Error())
			return
		}
		udp.Logger.Prefix = "UDPToTCPBridge - " + udp.Logger.Prefix
		br.udpToTCP[udp] = tcp
		br.tcpToUDP[tcp] = udp
		udp.Connect()
	}

	//Send
	udp.Send(data)
}

func (br *UDPToTCPBridge) handleUDPReadFunc(udp *UDPClient, data []byte, ended bool) {
	tcp := br.udpToTCP[udp]
	if tcp == nil {
		//Connection not found
		br.tcpServer.Logger.Log(3, "No TCP connection found for UDP connection connected to: "+udp.address.String())
		return
	}
	if ended {
		//Close connection
		tcp.Close()
		delete(br.tcpToUDP, tcp)
		delete(br.udpToTCP, udp)
		return
	}

	//Send data
	tcp.Send(data)
}

/*
Starts UDP to TCP Bridge
*/
func (br *UDPToTCPBridge) Start() {
	br.tcpServer.Start()
}

/*
Stops UDP to TCP Bridge
*/
func (br *UDPToTCPBridge) Stop() {
	br.tcpServer.Stop()
}
