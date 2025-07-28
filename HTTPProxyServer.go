package webtools

import (
	"net"
)

/*
HTTPProxy server that translates HTTP trafic from internet to local TCP and acts as TCP client
*/
type HTTPProxyServer struct {
	tcpServerAdress                   string
	webSocketServer                   *WebSocketHTTPServer
	connetionWebSocketToTCPTranslator map[*HTTPServerWebSocketConnection]*TCPClient
	connetionTCPToWebSocketTranslator map[net.Conn]*HTTPServerWebSocketConnection
	Logger                            ConsoleLogger
}

/*
Gets address of destination address - adress of proxy server
*/
func (proxySv *HTTPProxyServer) GetAddress() string {
	return proxySv.webSocketServer.GetAddress()
}

/*
Returns if local websocket server is alive (AKA proxy server)
*/
func (proxySv *HTTPProxyServer) IsAlive() bool {
	return proxySv.webSocketServer.IsAlive()
}

/*
Read data Handler for TCP
*/
func (proxySv *HTTPProxyServer) readFuncTCP(conn net.Conn, data string, ended bool) {
	if proxySv.connetionTCPToWebSocketTranslator[conn] == nil {
		proxySv.Logger.Log(3, "Error writing to WebSocket - Connection does not exist!")
		return
	}
	if !ended {
		//time.Sleep(5 * time.Second)
		proxySv.connetionTCPToWebSocketTranslator[conn].SendMessage(data)
	} else {
		proxySv.connetionTCPToWebSocketTranslator[conn].Close()
	}
}

/*
Read data Handler for WebSocket
*/
func (proxySv *HTTPProxyServer) readFuncWebSocket(ws *HTTPServerWebSocketConnection, data string, ended bool) {
	if proxySv.connetionWebSocketToTCPTranslator[ws] == nil {
		tcpClient := MakeTCPClient(proxySv.tcpServerAdress, proxySv.readFuncTCP, false, "")
		tcpClient.Logger = proxySv.Logger
		tcpClient.Connect()
		proxySv.connetionWebSocketToTCPTranslator[ws] = &tcpClient
		proxySv.connetionTCPToWebSocketTranslator[tcpClient.connection] = ws
	}
	if !ended {
		proxySv.connetionWebSocketToTCPTranslator[ws].WriteToTCPServer(data)
	} else {
		proxySv.connetionWebSocketToTCPTranslator[ws].Close()
	}
}

/*
Constructs new instance of HTTPProxy Server but does not start it
*/
func MakeHTTPProxyServer(tcpServerAdress string, proxyHostAddress string, dataPathPrefix string, sharedDataPathPrefix string, httpGetViewsFunc funcViews, httpPostViewsFunc funcViews, startWebBrowser bool) HTTPProxyServer {
	httpProxyServer := HTTPProxyServer{tcpServerAdress: tcpServerAdress, connetionWebSocketToTCPTranslator: map[*HTTPServerWebSocketConnection]*TCPClient{}, connetionTCPToWebSocketTranslator: map[net.Conn]*HTTPServerWebSocketConnection{}, Logger: MakeConsoleLogger("HTTPProxyServer")}
	httpProxyServer.webSocketServer = NewWebSocketHTTPServer(proxyHostAddress, dataPathPrefix, sharedDataPathPrefix, httpGetViewsFunc, httpPostViewsFunc, httpProxyServer.readFuncWebSocket, nil, nil, startWebBrowser)
	httpProxyServer.webSocketServer.HttpServer.Logger = httpProxyServer.Logger
	httpProxyServer.webSocketServer.Logger = httpProxyServer.Logger
	return httpProxyServer
}

/*
Starts HTTPProxy server
*/
func (proxySv *HTTPProxyServer) Start() {
	proxySv.Logger.Log(2, "Started proxying server from "+proxySv.tcpServerAdress+" to "+proxySv.webSocketServer.HttpServer.address)
	proxySv.webSocketServer.Start()
}

func (proxySv HTTPProxyServer) Stop() {
	proxySv.webSocketServer.Stop()
}
