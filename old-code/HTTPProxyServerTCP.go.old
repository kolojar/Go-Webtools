package webtools

import (
	"net"
)

/*
HTTPProxy server that translates HTTP trafic from internet to local TCP and acts as TCP client
*/
type HTTPProxyServerTCP struct {
	tcpServerAdress                   string
	webSocketServer                   *WebSocketHTTPServer
	connetionWebSocketToTCPTranslator map[*HTTPServerWebSocketConnection]*TCPClient
	connetionTCPToWebSocketTranslator map[net.Conn]*HTTPServerWebSocketConnection
	Logger                            ConsoleLogger
}

/*
Gets address of destination address - adress of proxy server
*/
func (proxySv *HTTPProxyServerTCP) GetAddress() string {
	return proxySv.webSocketServer.GetAddress()
}

/*
Returns if local websocket server is alive (AKA proxy server)
*/
func (proxySv *HTTPProxyServerTCP) IsAlive() bool {
	return proxySv.webSocketServer.IsAlive()
}

/*
Read data Handler for TCP
*/
func (proxySv *HTTPProxyServerTCP) readFuncTCP(conn net.Conn, data string, ended bool) {
	if proxySv.connetionTCPToWebSocketTranslator[conn] == nil {
		proxySv.Logger.Log(3, "Error writing to WebSocket - Connection does not exist!")
		return
	}
	if !ended {
		//time.Sleep(5 * time.Second)
		proxySv.connetionTCPToWebSocketTranslator[conn].SendMessage(data)
	} else {
		conn2 := proxySv.connetionTCPToWebSocketTranslator[conn]
		//delete(proxySv.connetionWebSocketToTCPTranslator, conn2)
		//delete(proxySv.connetionTCPToWebSocketTranslator, conn)
		conn2.Close()
	}
}

/*
Read data Handler for WebSocket
*/
func (proxySv *HTTPProxyServerTCP) readFuncWebSocket(ws *HTTPServerWebSocketConnection, data string, ended bool) {
	if proxySv.connetionWebSocketToTCPTranslator[ws] == nil {
		tcpClient := MakeTCPClient(proxySv.tcpServerAdress, proxySv.readFuncTCP, false, "")
		tcpClient.Logger = proxySv.Logger
		tcpClient.Connect()
		proxySv.connetionWebSocketToTCPTranslator[ws] = &tcpClient
		proxySv.connetionTCPToWebSocketTranslator[tcpClient.connection] = ws
	}
	if !ended {
		proxySv.connetionWebSocketToTCPTranslator[ws].WriteToServer(data)
	} else {
		conn := proxySv.connetionWebSocketToTCPTranslator[ws].connection
		//delete(proxySv.connetionTCPToWebSocketTranslator, conn)
		//delete(proxySv.connetionWebSocketToTCPTranslator, ws)
		conn.Close()
	}
}

/*
Constructs new instance of HTTPProxy Server for TCP but does not start it
*/
func MakeHTTPProxyServerTCP(tcpServerAdress string, proxyHostAddress string, dataPathPrefix string, sharedDataPathPrefix string, httpGetViewsFunc funcViews, httpPostViewsFunc funcViews, startWebBrowser bool) HTTPProxyServerTCP {
	httpProxyServer := HTTPProxyServerTCP{tcpServerAdress: tcpServerAdress, connetionWebSocketToTCPTranslator: map[*HTTPServerWebSocketConnection]*TCPClient{}, connetionTCPToWebSocketTranslator: map[net.Conn]*HTTPServerWebSocketConnection{}, Logger: MakeConsoleLogger("HTTPProxyServerTCP")}
	httpProxyServer.webSocketServer = NewWebSocketHTTPServer(proxyHostAddress, dataPathPrefix, sharedDataPathPrefix, httpGetViewsFunc, httpPostViewsFunc, httpProxyServer.readFuncWebSocket, nil, nil, startWebBrowser)
	httpProxyServer.webSocketServer.HttpServer.Logger = httpProxyServer.Logger
	httpProxyServer.webSocketServer.Logger = httpProxyServer.Logger
	return httpProxyServer
}

/*
Starts HTTPProxy server
*/
func (proxySv *HTTPProxyServerTCP) Start() {
	proxySv.Logger.Log(2, "Started proxying server from "+proxySv.tcpServerAdress+" to "+proxySv.webSocketServer.HttpServer.address)
	proxySv.webSocketServer.Start()
}

func (proxySv HTTPProxyServerTCP) Stop() {
	proxySv.webSocketServer.Stop()
}
