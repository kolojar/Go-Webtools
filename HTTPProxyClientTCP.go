package webtools

import "net"

/*
HTTPProxy client that translates HTTP trafic from internet to local TCP and hosts TCP server
*/
type HTTPProxyClientTCP struct {
	proxyHostAddress                  string
	tcpServer                         TCPServer
	connetionWebSocketToTCPTranslator map[net.Conn]net.Conn
	connetionTCPToWebSocketTranslator map[net.Conn]*WebSocketClient
	Logger                            ConsoleLogger
}

/*
Read data Handler for TCP
*/
func (proxyCl *HTTPProxyClientTCP) readFuncTCP(conn net.Conn, data string, ended bool) {
	if proxyCl.connetionTCPToWebSocketTranslator[conn] == nil {
		wsClient := MakeWebSocketClient(proxyCl.proxyHostAddress, proxyCl.readFuncWebSocket)
		wsClient.Logger = proxyCl.Logger
		wsClient.Connect()
		proxyCl.connetionTCPToWebSocketTranslator[conn] = &wsClient
		proxyCl.connetionWebSocketToTCPTranslator[wsClient.connection] = conn
	}
	if !ended {
		proxyCl.connetionTCPToWebSocketTranslator[conn].WriteToServer(data)
	} else {
		conn2 := proxyCl.connetionTCPToWebSocketTranslator[conn].connection
		delete(proxyCl.connetionWebSocketToTCPTranslator, conn2)
		delete(proxyCl.connetionTCPToWebSocketTranslator, conn)
		conn2.Close()
	}
}

/*
Read data Handler for WebSocket
*/
func (proxyCl *HTTPProxyClientTCP) readFuncWebSocket(conn net.Conn, data string, ended bool) {
	if proxyCl.connetionWebSocketToTCPTranslator[conn] == nil {
		proxyCl.Logger.Log(3, "Error writing to TCP - Connection does not exist!")
		return
	}
	if !ended {
		proxyCl.tcpServer.WriteToClient(proxyCl.connetionWebSocketToTCPTranslator[conn], data)
	} else {
		conn2 := proxyCl.connetionWebSocketToTCPTranslator[conn]
		delete(proxyCl.connetionTCPToWebSocketTranslator, conn2)
		delete(proxyCl.connetionWebSocketToTCPTranslator, conn)
		conn2.Close()
	}
}

func (proxyCl *HTTPProxyClientTCP) GetProxyHostAddress() string {
	return proxyCl.proxyHostAddress
}

/*
Constructs new instance of HTTPProxy Client for TCP but does not start it
*/
func MakeHTTPProxyClientTCP(tcpServerAdress string, proxyHostAddress string) HTTPProxyClientTCP {
	httpProxyClient := HTTPProxyClientTCP{proxyHostAddress: proxyHostAddress, connetionWebSocketToTCPTranslator: map[net.Conn]net.Conn{}, connetionTCPToWebSocketTranslator: map[net.Conn]*WebSocketClient{}, Logger: MakeConsoleLogger("HTTPProxyClientTCP")}
	httpProxyClient.tcpServer = MakeTCPServer(tcpServerAdress, httpProxyClient.readFuncTCP, false, "")
	httpProxyClient.tcpServer.Logger = httpProxyClient.Logger
	return httpProxyClient
}

/*
Starts HTTPProxy client
*/
func (proxyCl *HTTPProxyClientTCP) Start() {
	proxyCl.Logger.Log(2, "Started proxying client from "+proxyCl.proxyHostAddress+" to "+proxyCl.tcpServer.address)
	proxyCl.tcpServer.Start()
}

func (proxyCl *HTTPProxyClientTCP) Stop() error {
	return proxyCl.tcpServer.Stop()
}

func (proxyCl *HTTPProxyClientTCP) IsAlive() bool {
	return proxyCl.tcpServer.IsAlive()
}

func (proxyCl *HTTPProxyClientTCP) GetTCPServerIP() string {
	return proxyCl.tcpServer.GetAddress()
}
