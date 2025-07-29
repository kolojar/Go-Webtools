package webtools

import (
	"fmt"
	"net"
	"time"
)

const UDP_CONN_DEADLINE = 10 * time.Second

/*
HTTPProxy UDP Frame
*/
type UDPProxyFrame struct {
	Data          string
	ClientIP      string
	ClientProxyIP string
}

/*
HTTPProxy server that translates HTTP trafic from internet to local TCP and acts as TCP client
*/
type HTTPProxyServerUDP struct {
	udpServerAdress                   string
	webSocketServer                   *WebSocketHTTPServer
	connetionWebSocketToUDPTranslator map[*HTTPServerWebSocketConnection]*UDPClient
	connetionUDPToWebSocketTranslator map[*UDPClient]*HTTPServerWebSocketConnection
	Logger                            ConsoleLogger
}

/*
Gets address of destination address - adress of proxy server
*/
func (proxySv *HTTPProxyServerUDP) GetAddress() string {
	return proxySv.webSocketServer.GetAddress()
}

/*
Returns if local websocket server is alive (AKA proxy server)
*/
func (proxySv *HTTPProxyServerUDP) IsAlive() bool {
	return proxySv.webSocketServer.IsAlive()
}

/*
Read data Handler for TCP
*/
func (proxySv *HTTPProxyServerUDP) readFuncUDP(client *UDPClient, addr *net.UDPAddr, data string, ended bool) {
	if proxySv.connetionUDPToWebSocketTranslator[client] == nil {
		proxySv.Logger.Log(3, "Error writing to WebSocket - Connection does not exist! Looking for address: "+addr.String())
		return
	}
	if !ended {
		//time.Sleep(5 * time.Second)
		proxySv.connetionUDPToWebSocketTranslator[client].SendMessage(data)
	} else {
		conn := proxySv.connetionUDPToWebSocketTranslator[client]
		delete(proxySv.connetionWebSocketToUDPTranslator, conn)
		delete(proxySv.connetionUDPToWebSocketTranslator, client)
		conn.Close()
		fmt.Println("Client count: ", len(proxySv.connetionWebSocketToUDPTranslator))
	}
}

/*
Read data Handler for WebSocket
*/
func (proxySv *HTTPProxyServerUDP) readFuncWebSocket(ws *HTTPServerWebSocketConnection, data string, ended bool) {
	if proxySv.connetionWebSocketToUDPTranslator[ws] == nil {
		udpClient := MakeUDPClientAdvanced(proxySv.udpServerAdress, proxySv.readFuncUDP, false, "")
		udpClient.Logger = proxySv.Logger
		udpClient.Connect()
		proxySv.connetionWebSocketToUDPTranslator[ws] = &udpClient
		proxySv.connetionUDPToWebSocketTranslator[&udpClient] = ws
	}
	fmt.Println("Client count: ", len(proxySv.connetionWebSocketToUDPTranslator))
	if !ended {
		proxySv.connetionWebSocketToUDPTranslator[ws].WriteToServer(data)
		proxySv.connetionWebSocketToUDPTranslator[ws].connection.SetReadDeadline(time.Now().Add(UDP_CONN_DEADLINE))
	} else {
		client := proxySv.connetionWebSocketToUDPTranslator[ws]
		delete(proxySv.connetionUDPToWebSocketTranslator, client)
		delete(proxySv.connetionWebSocketToUDPTranslator, ws)
		client.Close()
	}
}

/*
Constructs new instance of HTTPProxy Server for UDP but does not start it
*/
func MakeHTTPProxyServerUDP(udpServerAdress string, proxyHostAddress string, dataPathPrefix string, sharedDataPathPrefix string, httpGetViewsFunc funcViews, httpPostViewsFunc funcViews, startWebBrowser bool) HTTPProxyServerUDP {
	httpProxyServer := HTTPProxyServerUDP{udpServerAdress: udpServerAdress, connetionWebSocketToUDPTranslator: map[*HTTPServerWebSocketConnection]*UDPClient{}, connetionUDPToWebSocketTranslator: map[*UDPClient]*HTTPServerWebSocketConnection{}, Logger: MakeConsoleLogger("HTTPProxyServerUDP")}
	httpProxyServer.webSocketServer = NewWebSocketHTTPServer(proxyHostAddress, dataPathPrefix, sharedDataPathPrefix, httpGetViewsFunc, httpPostViewsFunc, httpProxyServer.readFuncWebSocket, nil, nil, startWebBrowser)
	httpProxyServer.webSocketServer.HttpServer.Logger = httpProxyServer.Logger
	httpProxyServer.webSocketServer.Logger = httpProxyServer.Logger
	return httpProxyServer
}

/*
Starts HTTPProxy server
*/
func (proxySv *HTTPProxyServerUDP) Start() {
	proxySv.Logger.Log(2, "Started proxying server from "+proxySv.udpServerAdress+" to "+proxySv.webSocketServer.HttpServer.address)
	proxySv.webSocketServer.Start()
}

func (proxySv HTTPProxyServerUDP) Stop() {
	proxySv.webSocketServer.Stop()
}
