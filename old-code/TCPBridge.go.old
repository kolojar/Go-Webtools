package webtools

import (
	"net"
)

/*
HTTPProxy server that translates HTTP trafic from internet to local TCP and acts as TCP client
*/
type TCPBridge struct {
	tcpSourceServerAdress string
	tcpServer             *TCPServer
	connetionTCP1To2      map[net.Conn]net.Conn
	connetionTCP2To1      map[net.Conn]*TCPClient
	Logger                ConsoleLogger
}

/*
Read data Handler for TCP
*/
func (bridge *TCPBridge) readFuncTCP1(conn net.Conn, data string, ended bool) {
	if bridge.connetionTCP1To2[conn] == nil {
		bridge.Logger.Log(3, "Error writing to TCP Client - Connection does not exist!")
		return
	}
	if !ended {
		bridge.tcpServer.WriteToClient(bridge.connetionTCP1To2[conn], data)
	} else {
		//conn := proxySv.connetionWebSocketToTCPTranslator[ws].connection
		//delete(proxySv.connetionTCPToWebSocketTranslator, conn)
		//delete(proxySv.connetionWebSocketToTCPTranslator, ws)
		bridge.connetionTCP1To2[conn].Close()
	}
}

func (bridge *TCPBridge) readFuncTCP2(conn net.Conn, data string, ended bool) {
	if bridge.connetionTCP2To1[conn] == nil {
		tcpClient := MakeTCPClient(bridge.tcpSourceServerAdress, bridge.readFuncTCP1, false, "")
		tcpClient.Logger = bridge.Logger
		tcpClient.Connect()
		bridge.connetionTCP2To1[conn] = &tcpClient
		bridge.connetionTCP1To2[tcpClient.connection] = conn
	}
	if !ended {
		bridge.connetionTCP2To1[conn].WriteToServer(data)
	} else {
		//conn := proxySv.connetionWebSocketToTCPTranslator[ws].connection
		//delete(proxySv.connetionTCPToWebSocketTranslator, conn)
		//delete(proxySv.connetionWebSocketToTCPTranslator, ws)
		bridge.connetionTCP2To1[conn].Close()
	}
}

/*
Constructs new instance of HTTPProxy Server for TCP but does not start it
*/
func MakeTCPBridge(tcpSourceServerAdress string, tcpClientAddress string) TCPBridge {
	httpProxyServer := TCPBridge{tcpSourceServerAdress: tcpSourceServerAdress, connetionTCP1To2: map[net.Conn]net.Conn{}, connetionTCP2To1: map[net.Conn]*TCPClient{}, Logger: MakeConsoleLogger("TCPBridge")}
	tcp := MakeTCPServer(tcpClientAddress, httpProxyServer.readFuncTCP2, false, "")
	httpProxyServer.tcpServer = &tcp
	return httpProxyServer
}

/*
Starts HTTPProxy server
*/
func (bridge *TCPBridge) Start() {
	bridge.Logger.Log(2, "Started bridging server from "+bridge.tcpSourceServerAdress+" to "+bridge.tcpServer.GetAddress())
	bridge.tcpServer.Start()
}
