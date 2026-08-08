package udp

import (
	"net"
	"strconv"

	webtools "github.com/kolojar/Go-Webtools"
	"github.com/kolojar/Go-Webtools/helpertools"
)

// StabilizerOrderLevel is helper type
type StabilizerOrderLevel uint8

// StabilizerNoOrdering disables ordering
const StabilizerNoOrdering StabilizerOrderLevel = 0

// StabilizerInstantOrdering enables instant order checking, instantly drops out of order packets.
// Example: If packet 2 arrives before packet 1, it processes packet 2 and packet 1 gets dropped.
const StabilizerInstantOrdering StabilizerOrderLevel = 1

// StabilizerTimeoutOrdering enables timout order checking, drops out of order packets after timeout.
// Example: When packet 2 arrieves, it waits timeout (if there are packets before that have not arrived) before processing the packet 2 and dropping all older packets.
//
// Note: Timeout is set via setting for connection stabilizer
const StabilizerTimeoutOrdering StabilizerOrderLevel = 2

// ServerStableConn is connection object of ServerStable
type ServerStableConn[sequenceNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowWordType ~uint8 | ~uint16 | ~uint32 | ~uint64] struct {
	origin *ServerStable[sequenceNumberType, orderNumberType, windowWordType]
	sConn  *connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType]
	conn   *ServerConn
}

// GetOrigin gets origin
func (conn *ServerStableConn[sequenceNumberType, orderNumberType, windowWordType]) GetOrigin() *ServerStable[sequenceNumberType, orderNumberType, windowWordType] {
	return conn.origin
}

// GetAddress gets address
func (conn *ServerStableConn[sequenceNumberType, orderNumberType, windowWordType]) GetAddress() *net.UDPAddr {
	return conn.conn.address
}

// Send sends data to client with default server stabilizer config
func (conn *ServerStableConn[sequenceNumberType, orderNumberType, windowWordType]) Send(data []byte) {
	conn.origin.WriteToClient(conn, data)
}

// Send sends data to client
func (conn *ServerStableConn[sequenceNumberType, orderNumberType, windowWordType]) SendAdvanced(data []byte, useResend bool, orderLevel StabilizerOrderLevel) {
	conn.origin.WriteToClientAdvanced(conn, data, useResend, orderLevel)
}

// ServerStableReadFunc is function definition for reading data from ServerStable
type ServerStableReadFunc[sequenceNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowWordType ~uint8 | ~uint16 | ~uint32 | ~uint64] func(conn *ServerStableConn[sequenceNumberType, orderNumberType, windowWordType], data []byte, status webtools.NetworkStatus)

// ServerStable is struct for UDP server with some enhacements to make UDP comunication more reliable
type ServerStable[sequenceNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowWordType ~uint8 | ~uint16 | ~uint32 | ~uint64] struct {
	udpServer  Server
	stabilizer connectionStabilizer[*ServerConn, sequenceNumberType, orderNumberType, windowWordType]
	readFunc   ServerStableReadFunc[sequenceNumberType, orderNumberType, windowWordType]
	conns      helpertools.SafeMap[*connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType], *ServerStableConn[sequenceNumberType, orderNumberType, windowWordType]]
}

func NewServerStable[sequenceNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowWordType ~uint8 | ~uint16 | ~uint32 | ~uint64](address string, readFunc ServerStableReadFunc[sequenceNumberType, orderNumberType, windowWordType], connectionStabilizerSettings *ConnectionStabilizerSettings, reportTraffic bool) (sv *ServerStable[sequenceNumberType, orderNumberType, windowWordType], err error) {
	//Create basic structure
	sv = &ServerStable[sequenceNumberType, orderNumberType, windowWordType]{
		readFunc: readFunc,
		conns:    helpertools.MakeSafeMap[*connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType], *ServerStableConn[sequenceNumberType, orderNumberType, windowWordType]](),
	}

	//Create UDP server
	udpSv, err := NewServer(address, sv.readFuncLocal, reportTraffic)
	if err != nil {
		return nil, err
	}
	sv.udpServer = *udpSv

	//Create connection stabilizer
	sv.stabilizer = *newConnectionStabilizer[*ServerConn](connectionStabilizerSettings, sv.stabilizerReadFunc, true)

	//Handle disconnect
	sv.udpServer.OnConnectionCleanup.AddEventListerner(false, func(conn *ServerConn) {
		sv.stabilizer.CleanupConnection(conn)
	})

	return sv, nil
}

// readFuncLocal is helper function for reading from udpServer
func (sv *ServerStable[sequenceNumberType, orderNumberType, windowWordType]) readFuncLocal(conn *ServerConn, data []byte, status webtools.NetworkStatus) {
	switch status {
	case webtools.ConnectStatus:
		{
			//Handle connect
			sConn := sv.stabilizer.HandleConnect(conn)

			//Create server conn
			svConn := &ServerStableConn[sequenceNumberType, orderNumberType, windowWordType]{
				origin: sv,
				sConn:  sConn,
				conn:   conn,
			}
			sv.conns.Set(sConn, svConn)

			//Pass to read func
			if sv.readFunc != nil {
				sv.readFunc(svConn, data, status)
			}
		}
	case webtools.DisconnectStatus:
		{
			//Handle disconnect
			sConn := sv.stabilizer.GetConn(conn)
			svConn := sv.conns.Get(sConn)
			sv.conns.Delete(sConn)
			sv.stabilizer.CleanupConnection(conn)

			//Pass to read func
			if sv.readFunc != nil {
				sv.readFunc(svConn, data, status)
			}
		}
	case webtools.ReadDataStatus:
		{
			sv.stabilizer.HandleRead(conn, data)
		}
	default:
		{
			sv.udpServer.Logger.Log(4, "Invalid connection status: "+strconv.FormatUint(uint64(status), 10)+" for connection: "+conn.address.String())
		}
	}
}

// stabilizerReadFunc is read func for stabilizer
func (sv *ServerStable[sequenceNumberType, orderNumberType, windowWordType]) stabilizerReadFunc(conn *connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType], data []byte) {
	if sv.readFunc != nil {
		sv.readFunc(sv.conns.Get(conn), data, webtools.ReadDataStatus)
	}
}

// WriteToClient writes data to connection using default settings
func (sv *ServerStable[sequenceNumberType, orderNumberType, windowWordType]) WriteToClient(conn *ServerStableConn[sequenceNumberType, orderNumberType, windowWordType], data []byte) {
	sv.stabilizer.HandleWriteDefault(conn.conn, data)
}

// WriteToClientAdvanced writes data to connection using specific frame settings
//
// Note: When useResend is true, any orderLevel not equal to 0 will enable ordering
func (sv *ServerStable[sequenceNumberType, orderNumberType, windowWordType]) WriteToClientAdvanced(conn *ServerStableConn[sequenceNumberType, orderNumberType, windowWordType], data []byte, useResend bool, orderLevel StabilizerOrderLevel) {
	if useResend {
		if orderLevel == StabilizerNoOrdering {
			sv.stabilizer.HandleWrite(conn.conn, StableDataWithResendFrame, data)
		} else {
			sv.stabilizer.HandleWrite(conn.conn, StableDataWithOrderResendFrame, data)
		}
	} else {
		switch orderLevel {
		case StabilizerNoOrdering:
			sv.stabilizer.HandleWrite(conn.conn, StableDataFrame, data)
		case StabilizerInstantOrdering:
			sv.stabilizer.HandleWrite(conn.conn, StableDataWithOrderInstantFrame, data)
		case StabilizerTimeoutOrdering:
			sv.stabilizer.HandleWrite(conn.conn, StableDataWithOrderTimeoutFrame, data)
		default:
			panic("unknown orderLevel: " + strconv.FormatUint(uint64(orderLevel), 10))
		}
	}
}

/*
IsAlive gets if server is alive
*/
func (sv *ServerStable[sequenceNumberType, orderNumberType, windowWordType]) IsAlive() bool {
	return sv.udpServer.isAlive
}

/*
GetAddress gets address of server
*/
func (sv *ServerStable[sequenceNumberType, orderNumberType, windowWordType]) GetAddress() *net.UDPAddr {
	return sv.udpServer.address
}

/*
Start starts UDP Server, locks execution thread
*/
func (sv *ServerStable[sequenceNumberType, orderNumberType, windowWordType]) Start() {
	sv.udpServer.Start()
}

/*
Stops stops UDP Server
*/
func (sv *ServerStable[sequenceNumberType, orderNumberType, windowWordType]) Stop() {
	sv.udpServer.Stop()
	sv.stabilizer.Stop()
}
