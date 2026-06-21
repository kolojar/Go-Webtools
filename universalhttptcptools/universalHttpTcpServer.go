/*
Package universalhttptcp provides tools universal handeling TCP and WebSocket traffic, because they work almost the same
*/
package universalhttptcp

import (
	"net"

	webtools "github.com/kolojar/Go-Webtools"
	"github.com/kolojar/Go-Webtools/httptools"
	"github.com/kolojar/Go-Webtools/tcp"
)

/*
ServerConn is connection object of Server
*/
type ServerConn struct {
	tcpConn *tcp.ServerConn
	wsConn  *httptools.WebSocketServerConn
}

/*
Close closes connection to client
*/
func (conn *ServerConn) Close() {
	if conn.tcpConn != nil {
		conn.tcpConn.Close()
	}
	if conn.wsConn != nil {
		conn.wsConn.Close()
	}
}

/*
Send sends data to client using TCP or WS
*/
func (conn *ServerConn) Send(data []byte) {
	if conn.tcpConn != nil {
		conn.tcpConn.Send(data)
	}
	if conn.wsConn != nil {
		conn.wsConn.Send(data)
	}
}

/*
GetConn gets raw TCP connection
*/
func (conn *ServerConn) GetConn() *net.TCPConn {
	if conn.tcpConn != nil {
		return conn.tcpConn.GetConn()
	}
	if conn.wsConn != nil {
		return conn.wsConn.Client.GetConn()
	}
	return nil
}

/*
ServerReadFunc is function definition for reading data from Server
*/
type ServerReadFunc func(conn *ServerConn, data []byte, status webtools.NetworkStatus, isBinary bool)

/*
Server [rovides universal API for selecting TCP or/and HTTP WebSocket app hosting
*/
type Server struct {
	tcpServer     *tcp.Server
	readFunc      ServerReadFunc
	httpServer    *httptools.WebSocketServer
	reportTraffic bool
}

// NewServer creates new Universal HTTP and TCP server but does not starts it or configures any subservers, for setup use ConfigureTCP or ConfigureWS
func NewServer(readFunc ServerReadFunc, reportTraffic bool) *Server {
	return &Server{reportTraffic: reportTraffic, readFunc: readFunc}
}

// ConfigureTCP configures usage of TCP on this server
func (sv *Server) ConfigureTCP(address string, isFramed bool) error {
	var err error
	sv.tcpServer, err = tcp.NewServer(address, sv.readFuncTCP, sv.reportTraffic, isFramed)
	sv.tcpServer.Logger.Prefix = "Universal - " + sv.tcpServer.Logger.Prefix
	return err
}

// ConfigureWS configures usage of WebSockets on this server
func (sv *Server) ConfigureWS(address string) {
	sv.httpServer = httptools.NewWebSocketServer(address, sv.readFuncWS, nil, "", false, false, sv.reportTraffic)
	sv.httpServer.GetLogger().Prefix = "Universal - " + sv.httpServer.GetLogger().Prefix
}

func (sv *Server) readFuncTCP(conn *tcp.ServerConn, data []byte, status webtools.NetworkStatus) {
	if sv.readFunc != nil {
		sv.readFunc(&ServerConn{tcpConn: conn, wsConn: nil}, data, status, true)
	}
}

func (sv *Server) readFuncWS(conn *httptools.WebSocketServerConn, data []byte, status webtools.NetworkStatus, isBinary bool) {
	if sv.readFunc != nil {
		sv.readFunc(&ServerConn{tcpConn: nil, wsConn: conn}, data, status, isBinary)
	}
}

// Start starts server(s), locks execution thread
func (sv *Server) Start() {
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

// Stop stops server(s)
func (sv *Server) Stop() {
	if sv.tcpServer != nil {
		sv.tcpServer.Stop()
	}
	if sv.httpServer != nil {
		sv.httpServer.Stop()
	}
}
