package udpastcp

import (
	"bytes"
	"encoding/hex"
	"net"
	"strconv"
	"time"
	"webtools"
	"webtools/udp"
)

type ServerConnectFunc func(conn *ServerConn)

/*
Server is Server that simulates net.Conn (TCP conn) on top of UDP
*/
type Server struct {
	server        *udp.Server
	conns         webtools.SafeMap[*udp.ServerConn, *ServerConn]
	onConnectFunc ServerConnectFunc
}

/*
IsAlive gets if server is alive
*/
func (server *Server) IsAlive() bool {
	return server.server.IsAlive()
}

/*
GetAddress gets address of server
*/
func (server *Server) GetAddress() *net.UDPAddr {
	return server.server.GetAddress()
}

/*
NewServer creates new UDP Server but does not starts it
*/
func NewServer(address string, onConnectFunc ServerConnectFunc, reportTraffic bool) (*Server, error) {
	sv := &Server{conns: webtools.MakeSafeMap[*udp.ServerConn, *ServerConn](), onConnectFunc: onConnectFunc}
	var err error
	sv.server, err = udp.NewServer(address, sv.readFuncLocal, reportTraffic)
	if err != nil {
		return nil, err
	}
	sv.server.Logger.Prefix = "UDPAsTCPServer"
	return sv, nil
}

/*
Start starts UDP as TCP Server, locks execution thread
*/
func (server *Server) Start() {
	server.server.Start()
}

/*
Handles UDP Read for server
*/
func (server *Server) readFuncLocal(conn *udp.ServerConn, data []byte, ended bool) {
	if !ended {
		//Get connection association
		var udpConn *ServerConn = server.conns.Get(conn)
		if udpConn == nil {
			//No connection, create new
			udpConn = &ServerConn{conn: conn, localAddress: server.GetAddress(), readDeadline: time.Time{}, writeDeadline: time.Time{}, buffer: *bytes.NewBuffer(make([]byte, 0)), origin: server}
			server.conns.Set(conn, udpConn)
			if server.onConnectFunc != nil {
				go server.onConnectFunc(udpConn)
			}
		}

		//Process read
		server.server.Logger.Log(0, "Reading from and buffering: "+conn.Address.String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
		udpConn.buffer.Write(data)
	}
}

/*
Stop stops UDP as TCP server
*/
func (server *Server) Stop() {
	server.server.Stop()
}
