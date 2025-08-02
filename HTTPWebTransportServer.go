package webtools

import (
	"encoding/hex"
	"net"
	"net/http"
	"strconv"
	"strings"
)

/*
Standardized type of function
*HTTPWebTransportServerConn = Connection
String = message
Bool = is ended
*/
type HTTPWebTransportServerReadFunc func(*HTTPWebTransportServerConn, []byte, bool)

/*
HTTP WebTransport server connection object
*/
type HTTPWebTransportServerConn struct {
	origin *HTTPWebTransportServer
	conn   net.Conn
}

/*
Sends data to client
*/
func (httpConn *HTTPWebTransportServerConn) Send(data []byte) {
	httpConn.origin.WriteToClient(httpConn.conn, data)
}

/*
Closes connection to client
*/
func (httpConn *HTTPWebTransportServerConn) Close() {
	err := httpConn.conn.Close()
	if err != nil {
		httpConn.origin.logger.Log(3, "Error closing connection from: "+httpConn.conn.RemoteAddr().String()+" connected locally to: "+httpConn.conn.LocalAddr().String()+" with error: "+err.Error())
	} else {
		httpConn.origin.logger.Log(0, "Closed connectin on "+httpConn.conn.RemoteAddr().String()+" connected locally to: "+httpConn.conn.LocalAddr().String())
	}
}

/*
Simple HTTP connection hijack server fo switching from HTTP to TCP.
This is NOT WebSocket HTTP server for JavaScript, it is intended for inner communication between Go server (this file) and Go client. It is used for HTTPProxy (TCP and UDP traffic over HTTP)
*/
type HTTPWebTransportServer struct {
	httpServer *HTTPServer
	logger     *ConsoleLogger
	conns      map[net.Conn]*HTTPWebTransportServerConn
	readFunc   HTTPWebTransportServerReadFunc
}

/*
Creates new HTTP WebTransport Server but does not starts it
*/
func NewHTTPWebTransportServer(address string, readFunc HTTPWebTransportServerReadFunc, reportTraffic bool) *HTTPWebTransportServer {
	level := uint8(0)
	if !reportTraffic {
		level = 1
	}
	sv := &HTTPWebTransportServer{logger: NewConsoleLogger("HTTP-WTServer", level), readFunc: readFunc, conns: map[net.Conn]*HTTPWebTransportServerConn{}}
	sv.httpServer = NewHTTPServer(address, sv.handleHTTPAccess, "", false)
	sv.httpServer.Logger = sv.logger
	return sv
}

func (sv *HTTPWebTransportServer) handleHTTPAccess(_ *HTTPServer, w http.ResponseWriter, r *http.Request, params map[string]string) bool {
	if r.Method != http.MethodGet {
		//Invalid method
		return false
	}
	if r.URL.Path != "/webtransport" {
		//Invalid path
		return false
	}

	//Correct URL and Method
	sv.logger.Log(1, "Preparing connection from: "+r.RemoteAddr)

	//Verify if connection wants WebTransport
	if !strings.Contains(r.Header.Get("Upgrade"), "webtransport") || !strings.Contains(r.Header.Get("Connection"), "Upgrade") {
		http.Error(w, "Invalid WebTransport request", http.StatusBadRequest)
		return false
	}

	//Valid connection
	w.Header().Set("Upgrade", "webtransport")
	w.Header().Set("Connection", "Upgrade")

	//Request to switch to Webtransport keep-alive connection
	w.WriteHeader(http.StatusSwitchingProtocols)

	//Hijack connection
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		sv.logger.Log(3, "Failed to hijact connection from: "+r.RemoteAddr+" | Error: "+err.Error())
		return true
	}
	sv.logger.Log(2, "Connection from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String())
	go handleTCPRead(conn.(*net.TCPConn), sv.logger, sv.readFuncLocal)
	return true
}

func (sv *HTTPWebTransportServer) readFuncLocal(conn *net.TCPConn, data []byte, ended bool) {
	var httpConn *HTTPWebTransportServerConn = sv.conns[conn]
	if httpConn == nil {
		httpConn = &HTTPWebTransportServerConn{origin: sv, conn: conn}
		sv.conns[conn] = httpConn
	}
	//Process read
	if sv.readFunc != nil {
		if !ended {
			sv.logger.Log(0, "Reading from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
		}
		sv.readFunc(httpConn, data, ended)
	}
}

/*
Writes to Client
*/
func (sv *HTTPWebTransportServer) WriteToClient(conn net.Conn, data []byte) {
	writeToTCP(conn.(*net.TCPConn), data, sv.logger)
}

/*
Starts HTTP Server
*/
func (sv *HTTPWebTransportServer) Start() {
	sv.httpServer.Start()
}

/*
Stops HTTP Server
*/
func (sv *HTTPWebTransportServer) Stop() {
	sv.httpServer.Stop()
}
