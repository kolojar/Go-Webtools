package webtools

import (
	"net"
)

/*
HTTPProxy server that translates HTTP trafic from internet to local TCP and acts as TCP client
*/
type UDPBridge struct {
	tcpSourceServerAdress string
	udpServer             *UDPServer
	connetionUDP1To2      map[*UDPClient]*net.UDPAddr
	connetionUDP2To1      map[*net.UDPAddr]*UDPClient
	Logger                ConsoleLogger
}

/*
Read data Handler for TCP
*/
func (bridge *UDPBridge) readFuncTCP1(client *UDPClient, addr *net.UDPAddr, data string, ended bool) {
	if bridge.connetionUDP1To2[client] == nil {
		bridge.Logger.Log(3, "Error writing to UDP Client - Connection does not exist!")
		return
	}
	if !ended {
		bridge.udpServer.WriteToClient(bridge.connetionUDP1To2[client], data)
	} else {
		//conn := proxySv.connetionWebSocketToTCPTranslator[ws].connection
		//delete(proxySv.connetionTCPToWebSocketTranslator, conn)
		//delete(proxySv.connetionWebSocketToTCPTranslator, ws)
		//bridge.connetionUDP1To2[conn].Close()
	}
}

func (bridge *UDPBridge) readFuncTCP2(addr *net.UDPAddr, data string, ended bool) {
	if bridge.connetionUDP2To1[addr] == nil {
		udpClient := MakeUDPClientAdvanced(bridge.tcpSourceServerAdress, bridge.readFuncTCP1, false, "")
		udpClient.Logger = bridge.Logger
		udpClient.Connect()
		bridge.connetionUDP2To1[addr] = &udpClient
		bridge.connetionUDP1To2[&udpClient] = addr
	}
	if !ended {
		bridge.connetionUDP2To1[addr].WriteToServer(data)
	} else {
		//conn := proxySv.connetionWebSocketToTCPTranslator[ws].connection
		//delete(proxySv.connetionTCPToWebSocketTranslator, conn)
		//delete(proxySv.connetionWebSocketToTCPTranslator, ws)
		bridge.connetionUDP2To1[addr].Close()
	}
}

/*
Constructs new instance of HTTPProxy Server for TCP but does not start it
*/
func MakeUDPBridge(tcpSourceServerAdress string, tcpClientAddress string) UDPBridge {
	httpProxyServer := UDPBridge{tcpSourceServerAdress: tcpSourceServerAdress, connetionUDP1To2: map[*UDPClient]*net.UDPAddr{}, connetionUDP2To1: map[*net.UDPAddr]*UDPClient{}, Logger: MakeConsoleLogger("UDPBridge")}
	tcp := MakeUDPServer(tcpClientAddress, httpProxyServer.readFuncTCP2, false, "")
	httpProxyServer.udpServer = &tcp
	return httpProxyServer
}

/*
Starts HTTPProxy server
*/
func (bridge *UDPBridge) Start() {
	bridge.Logger.Log(2, "Started bridging server from "+bridge.tcpSourceServerAdress+" to "+bridge.udpServer.GetAddress())
	bridge.udpServer.Start()
}
