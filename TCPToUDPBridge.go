package webtools

import "sync"

/*
TCP to UDP bridge used for converting all incoming TCP traffic to UDP
*/
type TCPToUDPBridge struct {
	udpServer        *UDPServer
	udpToTCP         sync.Map
	tcpToUDP         sync.Map
	tcpServerAddress string
	reportTraffic    bool
}

/*
Creates new TCP to UDP bridge but does not starts it
*/
func NewTCPToUDPBridge(tcpServerAddress string, udpServerAddress string, reportTraffic bool) (*TCPToUDPBridge, error) {
	br := &TCPToUDPBridge{udpToTCP: sync.Map{}, tcpToUDP: sync.Map{}, tcpServerAddress: tcpServerAddress, reportTraffic: reportTraffic}
	var err error
	br.udpServer, err = NewUDPServer(udpServerAddress, br.handleUDPReadFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	br.udpServer.Logger.Prefix = "TCPToUDPBridge - " + br.udpServer.Logger.Prefix
	return br, nil
}

func (br *TCPToUDPBridge) handleUDPReadFunc(udp *UDPServerConn, data []byte, ended bool) {
	gtcp, _ := br.udpToTCP.Load(udp)
	if ended {
		//Connection ended
		if gtcp != nil {
			tcp := gtcp.(*TCPClient)
			tcp.Stop()
			br.tcpToUDP.Delete(tcp)
		}
		br.udpToTCP.Delete(udp)
		return
	}

	var tcp *TCPClient
	if gtcp == nil {
		//No connection to TCP found, creating new
		var err error
		tcp, err = NewTCPClient(br.tcpServerAddress, br.handleTCPReadFunc, br.reportTraffic)
		if err != nil {
			br.udpServer.Logger.Log(3, "Error creating new TCP connection to: "+br.tcpServerAddress+" | Error: "+err.Error())
			return
		}
		tcp.Logger.Prefix = "TCPToUDPBridge - " + tcp.Logger.Prefix
		br.udpToTCP.Store(udp, tcp)
		br.tcpToUDP.Store(tcp, udp)
		tcp.Connect()
	} else {
		tcp = gtcp.(*TCPClient)
	}

	//Send
	tcp.Send(data)
}

func (br *TCPToUDPBridge) handleTCPReadFunc(tcp *TCPClient, data []byte, ended bool) {
	gudp, _ := br.tcpToUDP.Load(tcp)
	if gudp == nil {
		//Connection not found
		br.udpServer.Logger.Log(3, "No UDP connection found for TCP connection connected to: "+tcp.Conn.RemoteAddr().String()+" connected locally to: "+tcp.Conn.LocalAddr().String())
		return
	}
	udp := gudp.(*UDPServerConn)
	if ended {
		//Close connection
		udp.Close()
		br.tcpToUDP.Delete(tcp)
		br.udpToTCP.Delete(udp)
		return
	}

	//Send data
	udp.Send(data)
}

/*
Starts TCP to UDP Bridge
*/
func (br *TCPToUDPBridge) Start() {
	br.udpServer.Start()
}

/*
Stops TCP to UDP Bridge
*/
func (br *TCPToUDPBridge) Stop() {
	br.udpServer.Stop()
}
