package webtools

/*
Universal TCP and HTTP WebSocket server. It lets user choose what server it will be hosting.
It is useful when you need to select between fast but LAN (local area network) TCP - it can be bridged or port forwarded (harder to setup and has security risks),
and HTTP WebSocket - can be tunneled (usually for free) and port forwared or bridged but is slower and makes more traffic
Use TCP when: You use server only on LAN or know port forwarding
Use HTTP when: You use server for wide use on HTTP webside or you can use simple HTTP tunelling tools
*/
type TCPHTTPWsUniversalServer struct {
	tcpServer       *TCPClientSimple
	websocketServer *WebSocketServer
}

