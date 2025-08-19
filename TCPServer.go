package webtools

import (
	"net"
	"strconv"
	"time"
)

const BUFFER_SIZE = 1024 * 16

/*
Creates new ConsoleLogger with option to disable traffic report. Traffic reports are reports with 0 level
*/
func NewConsoleLoggerForTraffic(prefix string, reportTraffic bool) *ConsoleLogger {
	return NewConsoleLogger(prefix, FormatByBool[uint8](reportTraffic, 0, 1))
}

/*
Server connection interface
*/
type ServerConn interface {
	Send([]byte)
	Close()
}

/*
TCP server connection object
*/
type TCPServerConn struct {
	origin *TCPServer
	Client *TCPClientSimple
}

func (tcp *TCPServerConn) GetConn() *net.TCPConn {
	return tcp.Client.GetConn()
}

/*
Sends data to client
*/
func (tcpConn *TCPServerConn) Send(data []byte) {
	if tcpConn == nil || tcpConn.Client == nil {
		return
	}
	tcpConn.Client.Send(data)
}

/*
Closes connection to client
*/
func (tcpConn *TCPServerConn) Close() {
	if tcpConn == nil {
		return
	}
	if tcpConn.Client == nil {
		return
	}
	tcpConn.Client.Stop()
}

/*
Standardized type of function
*TCPServerConn = Connection
String = message
Uint8 = status
*/
type TCPServerReadFunc func(conn *TCPServerConn, data []byte, status uint8)

/*
Basic TCP server
*/
type TCPServer struct {
	listener           *net.TCPListener
	readFunc           TCPServerReadFunc
	address            *net.TCPAddr
	Logger             *ConsoleLogger
	requestedStop      bool
	isAlive            bool
	conns              SafeMap[*TCPClientSimple, *TCPServerConn]
	framed             bool
	useEncryption      bool
	encryptionPassword string
}

func (sv *TCPServer) IsAlive() bool {
	return sv.isAlive
}

func (sv *TCPServer) GetAddress() string {
	return sv.address.String()
}

/*
Creates new TCP Server but does not starts it
*/
func NewTCPServer(address string, readFunc TCPServerReadFunc, reportTraffic bool, framed bool) (*TCPServer, error) {
	//Make address
	addressObj, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}
	level := uint8(0)
	if !reportTraffic {
		level = 1
	}
	return &TCPServer{address: addressObj, readFunc: readFunc, Logger: NewConsoleLogger("TCPServer", level), conns: MakeSafeMap[*TCPClientSimple, *TCPServerConn](), framed: framed}, nil
}

/*
Setups encryption, it is strongly recommended to use encryption with framed connection
*/
func (sv *TCPServer) SetupEncryption(useEncryption bool, password string) {
	sv.useEncryption = useEncryption
	sv.encryptionPassword = password
}

/*
Starts TCP Server. Locks execution thread
*/
func (tcp *TCPServer) Start() {
	//Check if already running
	if tcp.isAlive {
		return
	}

	//Reset stop request
	tcp.requestedStop = false

	//Open listener
	var err error
	tcp.listener, err = net.ListenTCP("tcp", tcp.address)
	if err != nil {
		tcp.Logger.Log(3, "Error listening to "+tcp.address.String()+" with error: "+err.Error())
		return
	}
	tcp.isAlive = true
	tcp.Logger.Log(2, "Started listening on "+tcp.address.String())

	//Listener loop
	for !tcp.requestedStop {
		conn, err2 := tcp.listener.AcceptTCP()
		if err2 != nil {
			if tcp.requestedStop {
				//Ignore all errors
				break
			} else {
				tcp.Logger.Log(3, "Error accepting connection: "+err2.Error())
			}
		}

		//Handle connection
		cl := NewTCPClientSimpleFromConnection(conn, FormatByBool(tcp.framed, 0, -1), false, tcp.readFuncLocal, false)
		cl.SetLogger(tcp.Logger)
		cl.SetupEncryption(tcp.useEncryption, tcp.encryptionPassword)
		cl.Connect()
	}
	tcp.isAlive = false
}

func (tcp *TCPServer) readFuncLocal(client *TCPClientSimple, data []byte, status uint8) {
	//Sort connection
	var tcpConn *TCPServerConn = tcp.conns.Get(client)
	if tcpConn == nil {
		tcpConn = &TCPServerConn{origin: tcp, Client: client}
		tcp.conns.Set(client, tcpConn)
	}
	if status == TCP_DISCONNECT_STATUS {
		tcp.conns.Delete(client)
	}
	tcp.Logger.Log(0, "Count of connections: "+strconv.Itoa(tcp.conns.Len()))

	//Process read
	if tcp.readFunc != nil {
		tcp.readFunc(tcpConn, data, status)
	}
}

/*
Stops TCP server
*/
func (tcp *TCPServer) Stop() {
	if !tcp.isAlive {
		return
	}

	//Request stop
	tcp.requestedStop = true
	err := tcp.listener.Close()
	time.Sleep(1 * time.Second)
	if err != nil {
		tcp.Logger.Log(3, "Error stopping TCP server: "+err.Error())
	}
}
