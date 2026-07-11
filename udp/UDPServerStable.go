package udp

import (
	"net"
)

// ServerStableConn is connection object of ServerStable
type ServerStableConn struct {
	origin *ServerStable
	conn   *ServerConn
}

// GetOrigin gets origin
func (conn *ServerStableConn) GetOrigin() *ServerStable {
	return conn.origin
}

// GetAddress gets address
func (conn *ServerStableConn) GetAddress() *net.UDPAddr {
	return conn.conn.GetAddress()
}

// Send sends data to client with default server stabilizer config
func (conn *ServerStableConn) Send(data []byte) {
	conn.origin.WriteToClient(conn, data)
}

// Send sends data to client
func (conn *ServerStableConn) SendAdvanced(data []byte, useResend bool, useOrder bool) {
	conn.origin.WriteToClientAdvanced(conn, data, useResend, useOrder)
}

// ServerStableReadFunc is function definition for reading data from ServerStable
type ServerStableReadFunc func(conn *ServerStableConn, data []byte, ended bool)

// ServerStable is struct for UDP server with some enhacements to make UDP comunication more reliable
type ServerStable struct {
	udpServer            *Server
	connectionStabilizer *connectionStabilizer
	readFunc             ServerStableReadFunc
}

func NewServerStable(address string, readFunc ServerStableReadFunc, reportTraffic bool, connectionStabilizerSettings ConnectionStabilizerSettings) (sv *ServerStable, err error) {
	sv = &ServerStable{
		connectionStabilizer: newConnectionStabilizer(connectionStabilizerSettings),
		readFunc:             readFunc,
	}
	sv.udpServer, err = NewServer(address, sv.readFuncLocal, reportTraffic)
	return sv, err
}

func (sv *ServerStable) readFuncLocal(conn *ServerConn, data []byte, ended bool) {
	connectionStabilizer.
}
