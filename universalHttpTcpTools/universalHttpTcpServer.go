package universalHttpTcpTools

import (
	"net"
	httptools "webtools/httpTools"
	tcptools "webtools/tcpTools"
)

/*
Simple and universal server connection
*/
type UniversalHttpTcpServerConn struct {
	tcpConn *tcptools.TCPServerConn
	wsConn  *httptools.WebSocketServerConn
}

func (conn *UniversalHttpTcpServerConn) Close() {
	if conn.tcpConn != nil {
		conn.tcpConn.Close()
	}
	if conn.wsConn != nil {
		conn.wsConn.Close()
	}
}

func (conn *UniversalHttpTcpServerConn) Send(data []byte) {
	if conn.tcpConn != nil {
		conn.tcpConn.Send(data)
	}
	if conn.wsConn != nil {
		conn.wsConn.Send(data)
	}
}

// Gets conn, used for comparision
func (conn *UniversalHttpTcpServerConn) GetConn() *net.TCPConn {
	if conn.tcpConn != nil {
		return conn.tcpConn.GetConn()
	}
	if conn.wsConn != nil {
		return conn.wsConn.Client.GetConn()
	}
	return nil
}

// Definition of function standard
type UniversalHttpTcpServerReadFunc func(conn *UniversalHttpTcpServerConn, data []byte, status uint8, isBinary bool)

/*
Provides universal API for selecting TCP or/and HTTP WebSocket app hosting
*/
type UniversalHttpTcpServer struct {
	tcpServer     *tcptools.TCPServer
	readFunc      UniversalHttpTcpServerReadFunc
	httpServer    *httptools.WebSocketServer
	reportTraffic bool
}

// Creates new Universal HTTP and TCP server but does not starts it or configures any subservers, for setup use ConfigureTCP or ConfigureWS
func NewUniversalHttpTcpServer(readFunc UniversalHttpTcpServerReadFunc, reportTraffic bool) *UniversalHttpTcpServer {
	return &UniversalHttpTcpServer{reportTraffic: reportTraffic, readFunc: readFunc}
}

// Configures usage of TCP on this server
func (sv *UniversalHttpTcpServer) ConfigureTCP(address string, isFramed bool) error {
	var err error
	sv.tcpServer, err = tcptools.NewTCPServer(address, sv.readFuncTcp, sv.reportTraffic, isFramed)
	sv.tcpServer.Logger.Prefix = "Universal - " + sv.tcpServer.Logger.Prefix
	return err
}

// Configures usage of WebSockets on this server
func (sv *UniversalHttpTcpServer) ConfigureWS(address string) {
	sv.httpServer = httptools.NewHTTPWebSocketServer(address, sv.readFuncWs, nil, "", sv.reportTraffic)
	sv.httpServer.Logger.Prefix = "Universal - " + sv.httpServer.Logger.Prefix
}

func (sv *UniversalHttpTcpServer) readFuncTcp(conn *tcptools.TCPServerConn, data []byte, status uint8) {
	if sv.readFunc != nil {
		sv.readFunc(&UniversalHttpTcpServerConn{tcpConn: conn, wsConn: nil}, data, status, true)
	}
}

func (sv *UniversalHttpTcpServer) readFuncWs(conn *httptools.WebSocketServerConn, data []byte, status uint8, isBinary bool) {
	if sv.readFunc != nil {
		sv.readFunc(&UniversalHttpTcpServerConn{tcpConn: nil, wsConn: conn}, data, status, isBinary)
	}
}

// Starts servers, locks execution thread
func (sv *UniversalHttpTcpServer) Start() {
	if sv.tcpServer != nil && sv.httpServer != nil {
		go sv.tcpServer.Start()
		sv.httpServer.Start()
	} else {
		if sv.tcpServer != nil {
			sv.tcpServer.Start()
		} else {
			sv.httpServer.Start()
		}
	}
}

// Stops servers
func (sv *UniversalHttpTcpServer) Stop() {
	if sv.tcpServer != nil {
		sv.tcpServer.Stop()
	}
	if sv.httpServer != nil {
		sv.httpServer.Stop()
	}
}
