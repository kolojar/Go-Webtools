package webtools

import "net"

/*
HTTPProxy client that translates HTTP trafic from internet to local TCP and hosts TCP server
*/
type HTTPProxyClient struct {
	proxyHostAddress                  string
	tcpServer                         TCPServer
	connetionWebSocketToTCPTranslator map[net.Conn]net.Conn
	connetionTCPToWebSocketTranslator map[net.Conn]*WebSocketClient
	Logger                            ConsoleLogger
}

/*
Read data Handler for TCP
*/
func (proxyCl *HTTPProxyClient) readFuncTCP(conn net.Conn, data string, ended bool) {
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
		proxyCl.connetionTCPToWebSocketTranslator[conn].Close()
	}

}

/*
Read data Handler for WebSocket
*/
func (proxyCl *HTTPProxyClient) readFuncWebSocket(conn net.Conn, data string, ended bool) {
	if proxyCl.connetionWebSocketToTCPTranslator[conn] == nil {
		proxyCl.Logger.Log(3, "Error writing to TCP - Connection does not exist!")
		return
	}
	if !ended {
		proxyCl.tcpServer.WriteToClient(proxyCl.connetionWebSocketToTCPTranslator[conn], data)
	} else {
		proxyCl.connetionWebSocketToTCPTranslator[conn].Close()
	}
}

func (proxyCl *HTTPProxyClient) GetProxyHostAddress() string {
	return proxyCl.proxyHostAddress
}

/*
Constructs new instance of HTTPProxy Client but does not start it
*/
func MakeHTTPProxyClient(tcpServerAdress string, proxyHostAddress string) HTTPProxyClient {
	httpProxyClient := HTTPProxyClient{proxyHostAddress: proxyHostAddress, connetionWebSocketToTCPTranslator: map[net.Conn]net.Conn{}, connetionTCPToWebSocketTranslator: map[net.Conn]*WebSocketClient{}, Logger: MakeConsoleLogger("HTTPProxyClient")}
	httpProxyClient.tcpServer = MakeTCPServer(tcpServerAdress, httpProxyClient.readFuncTCP, false, "")
	httpProxyClient.tcpServer.Logger = httpProxyClient.Logger
	return httpProxyClient
}

/*
Starts HTTPProxy client
*/
func (proxyCl *HTTPProxyClient) Start() {
	proxyCl.Logger.Log(2, "Started proxying client from "+proxyCl.proxyHostAddress+" to "+proxyCl.tcpServer.address)
	proxyCl.tcpServer.Start()
}

func (proxyCl *HTTPProxyClient) Stop() error {
	return proxyCl.tcpServer.Stop()
}

func (proxyCl *HTTPProxyClient) IsAlive() bool {
	return proxyCl.tcpServer.IsAlive()
}

func (proxyCl *HTTPProxyClient) GetTCPServerIP() string {
	return proxyCl.tcpServer.GetAddress()
}
