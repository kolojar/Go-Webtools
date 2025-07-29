package webtools

import (
	"fmt"
	"net"
)

/*
HTTPProxy client that translates HTTP trafic from internet to local TCP and hosts TCP server
*/
type HTTPProxyClientUDP struct {
	proxyHostAddress                  string
	udpServer                         *UDPServer
	connetionWebSocketToUDPTranslator map[net.Conn]*net.UDPAddr
	connetionUDPToWebSocketTranslator map[*net.UDPAddr]*WebSocketClient
	Logger                            ConsoleLogger
}

/*
Read data Handler for TCP
*/
func (proxyCl *HTTPProxyClientUDP) readFuncUDP(addr *net.UDPAddr, data string, ended bool) {
	if proxyCl.connetionUDPToWebSocketTranslator[addr] == nil {
		wsClient := MakeWebSocketClient(proxyCl.proxyHostAddress, proxyCl.readFuncWebSocket)
		wsClient.Logger = proxyCl.Logger
		wsClient.Connect()
		proxyCl.connetionUDPToWebSocketTranslator[addr] = &wsClient
		proxyCl.connetionWebSocketToUDPTranslator[wsClient.connection] = addr
	}
	fmt.Println("Client count: ", len(proxyCl.connetionWebSocketToUDPTranslator))
	if !ended {
		proxyCl.connetionUDPToWebSocketTranslator[addr].WriteToServer(data)
	} else {
		conn := proxyCl.connetionUDPToWebSocketTranslator[addr].connection
		delete(proxyCl.connetionWebSocketToUDPTranslator, conn)
		delete(proxyCl.connetionUDPToWebSocketTranslator, addr)
		conn.Close()
	}

}

/*
Read data Handler for WebSocket
*/
func (proxyCl *HTTPProxyClientUDP) readFuncWebSocket(conn net.Conn, data string, ended bool) {
	if proxyCl.connetionWebSocketToUDPTranslator[conn] == nil {
		proxyCl.Logger.Log(3, "Error writing to UDP - Connection does not exist!")
		return
	}
	if !ended {
		proxyCl.udpServer.WriteToClient(proxyCl.connetionWebSocketToUDPTranslator[conn], data)
	} else {
		//In UDP no closing needed
		delete(proxyCl.connetionUDPToWebSocketTranslator, proxyCl.connetionWebSocketToUDPTranslator[conn])
		delete(proxyCl.connetionWebSocketToUDPTranslator, conn)
		fmt.Println("Client count: ", len(proxyCl.connetionWebSocketToUDPTranslator))
	}
}

func (proxyCl *HTTPProxyClientUDP) GetProxyHostAddress() string {
	return proxyCl.proxyHostAddress
}

/*
Constructs new instance of HTTPProxy Client for UDP but does not start it
*/
func MakeHTTPProxyClientUDP(tcpServerAdress string, proxyHostAddress string) HTTPProxyClientUDP {
	httpProxyClient := HTTPProxyClientUDP{proxyHostAddress: proxyHostAddress, connetionWebSocketToUDPTranslator: map[net.Conn]*net.UDPAddr{}, connetionUDPToWebSocketTranslator: map[*net.UDPAddr]*WebSocketClient{}, Logger: MakeConsoleLogger("HTTPProxyClientUDP")}
	udp := MakeUDPServer(tcpServerAdress, httpProxyClient.readFuncUDP, false, "")
	httpProxyClient.udpServer = &udp
	httpProxyClient.udpServer.Logger = httpProxyClient.Logger
	return httpProxyClient
}

/*
Starts HTTPProxy client
*/
func (proxyCl *HTTPProxyClientUDP) Start() {
	proxyCl.Logger.Log(2, "Started proxying client from "+proxyCl.proxyHostAddress+" to "+proxyCl.udpServer.address)
	proxyCl.udpServer.Start()
}

func (proxyCl *HTTPProxyClientUDP) Stop() error {
	return proxyCl.udpServer.Stop()
}

func (proxyCl *HTTPProxyClientUDP) IsAlive() bool {
	return proxyCl.udpServer.IsAlive()
}

func (proxyCl *HTTPProxyClientUDP) GetUDPServerIP() string {
	return proxyCl.udpServer.GetAddress()
}
