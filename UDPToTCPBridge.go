package webtools

import "sync"

/*
UDP to TCP bridge used for converting all incoming UDP traffic to TCP
*/
type UDPToTCPBridge struct {
	tcpServer        *TCPServer
	tcpToUDP         sync.Map
	udpToTCP         sync.Map
	udpServerAddress string
	reportTraffic    bool
}

/*
Creates new UDP to TCP bridge but does not starts it
*/
func NewUDPToTCPBridge(udpServerAddress string, tcpServerAddress string, reportTraffic bool) (*UDPToTCPBridge, error) {
	br := &UDPToTCPBridge{udpToTCP: sync.Map{}, tcpToUDP: sync.Map{}, udpServerAddress: udpServerAddress, reportTraffic: reportTraffic}
	var err error
	br.tcpServer, err = NewTCPServer(tcpServerAddress, br.handleTCPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	br.tcpServer.Logger.Prefix = "UDPToTCPBridge - " + br.tcpServer.Logger.Prefix
	return br, nil
}

func (br *UDPToTCPBridge) handleTCPReadFunc(tcp *TCPServerConn, data []byte, ended bool) {
	gudp, _ := br.tcpToUDP.Load(tcp)
	if ended {
		//Connection ended
		if gudp != nil {
			udp := gudp.(*UDPClient)
			udp.Stop()
			br.udpToTCP.Delete(udp)
		}
		br.tcpToUDP.Delete(tcp)
		return
	}

	var udp *UDPClient
	if gudp == nil {
		//No connection to UDP found, creating new
		var err error
		udp, err = NewUDPClient(br.udpServerAddress, br.handleUDPReadFunc, br.reportTraffic)
		if err != nil {
			br.tcpServer.Logger.Log(3, "Error creating new UDP connection to: "+br.udpServerAddress+" | Error: "+err.Error())
			return
		}
		udp.Logger.Prefix = "UDPToTCPBridge - " + udp.Logger.Prefix
		br.udpToTCP.Store(udp, tcp)
		br.tcpToUDP.Store(tcp, udp)
		udp.Connect()
	} else {
		udp = gudp.(*UDPClient)
	}

	//Send
	udp.Send(data)
}

func (br *UDPToTCPBridge) handleUDPReadFunc(udp *UDPClient, data []byte, ended bool) {
	gtcp, _ := br.udpToTCP.Load(udp)
	if gtcp == nil {
		//Connection not found
		br.tcpServer.Logger.Log(3, "No TCP connection found for UDP connection connected to: "+udp.address.String())
		return
	}
	tcp := gtcp.(*TCPServerConn)
	if ended {
		//Close connection
		tcp.Close()
		br.tcpToUDP.Delete(tcp)
		br.udpToTCP.Delete(udp)
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
