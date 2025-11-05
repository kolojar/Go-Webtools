package tcp

import (
	"net"
	"strconv"
	"time"

	"webtools"
)

/*
ServerConn is connection object of Server
*/
type ServerConn struct {
	origin *Server
	Client *ClientSimple
}

/*
GetConn gets raw TCP connection
*/
func (conn *ServerConn) GetConn() *net.TCPConn {
	return conn.Client.GetConn()
}

/*
Send sends data to client
*/
func (conn *ServerConn) Send(data []byte) {
	if conn == nil || conn.Client == nil {
		return
	}
	conn.Client.Send(data)
}

/*
Close closes connection to client
*/
func (conn *ServerConn) Close() {
	if conn == nil {
		return
	}
	if conn.Client == nil {
		return
	}
	conn.Client.Stop()
}

/*
ServerReadFunc is function definition for reading data from Server
*/
type ServerReadFunc func(conn *ServerConn, data []byte, status uint8)

/*
Server is basic TCP server
*/
type Server struct {
	listener           *net.TCPListener
	readFunc           ServerReadFunc
	address            *net.TCPAddr
	Logger             *webtools.ConsoleLogger
	requestedStop      bool
	isAlive            bool
	conns              webtools.SafeMap[*ClientSimple, *ServerConn]
	framed             bool
	useEncryption      bool
	encryptionPassword []byte
}

/*
IsAlive gets if server is alive
*/
func (sv *Server) IsAlive() bool {
	return sv.isAlive
}

/*
GetAddress gets address of server
*/
func (sv *Server) GetAddress() string {
	return sv.address.String()
}

/*
NewServer creates new TCP Server but does not starts it
*/
func NewServer(address string, readFunc ServerReadFunc, reportTraffic bool, framed bool) (*Server, error) {
	// Make address
	addressObj, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}
	return &Server{address: addressObj, readFunc: readFunc, Logger: webtools.NewConsoleLoggerForTraffic("TCPServer", reportTraffic), conns: webtools.MakeSafeMap[*ClientSimple, *ServerConn](), framed: framed}, nil
}

/*
SetupEncryption setups encryption, it is strongly recommended to use encryption with framed connection
*/
func (sv *Server) SetupEncryption(useEncryption bool, password []byte) {
	sv.useEncryption = useEncryption
	if useEncryption {
		sv.encryptionPassword = password
	} else {
		sv.encryptionPassword = nil
	}
}

/*
Start starts TCP Server. Locks execution thread
*/
func (sv *Server) Start() {
	// Check if already running
	if sv.isAlive {
		return
	}

	// Reset stop request
	sv.requestedStop = false

	// Open listener
	var err error
	sv.listener, err = net.ListenTCP("tcp", sv.address)
	if err != nil {
		sv.Logger.Log(3, "Error listening to "+sv.address.String()+" with error: "+err.Error())
		return
	}
	sv.isAlive = true
	sv.Logger.Log(2, "Started listening on "+sv.address.String())

	// Listener loop
	for !sv.requestedStop {
		conn, err2 := sv.listener.AcceptTCP()
		if err2 != nil {
			if sv.requestedStop {
				// Ignore all errors
				break
			}
			sv.Logger.Log(3, "Error accepting connection: "+err2.Error())
		}

		// Handle connection
		cl := NewClientSimpleFromConnection(conn, webtools.FormatByBool(sv.framed, 0, -1), false, sv.readFuncLocal, false)
		cl.SetLogger(sv.Logger)
		cl.SetupEncryption(sv.useEncryption, sv.encryptionPassword)
		cl.Connect()
	}
	sv.isAlive = false
}

func (sv *Server) readFuncLocal(client *ClientSimple, data []byte, status uint8) {
	// Sort connection
	var tcpConn *ServerConn = sv.conns.Get(client)
	if tcpConn == nil {
		tcpConn = &ServerConn{origin: sv, Client: client}
		sv.conns.Set(client, tcpConn)
	}
	if status == webtools.DisconnectStatus {
		sv.conns.Delete(client)
	}
	sv.Logger.Log(0, "Count of connections: "+strconv.Itoa(sv.conns.Len()))

	// Process read
	if sv.readFunc != nil {
		sv.readFunc(tcpConn, data, status)
	}
}

/*
Stop stops TCP server
*/
func (sv *Server) Stop() {
	if !sv.isAlive {
		return
	}

	// Request stop
	sv.requestedStop = true
	err := sv.listener.Close()
	time.Sleep(1 * time.Second)
	if err != nil {
		sv.Logger.Log(3, "Error stopping TCP server: "+err.Error())
	}
}
