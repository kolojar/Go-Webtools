package tcpTools

import (
	"net"
	"strconv"
	"time"
	"webtools/encryption"

	"webtools"
)

/*
Server connection interface
*/
type IServerConn interface {
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
	listener               *net.TCPListener
	readFunc               TCPServerReadFunc
	address                *net.TCPAddr
	Logger                 *webtools.ConsoleLogger
	requestedStop          bool
	isAlive                bool
	conns                  webtools.SafeMap[*TCPClientSimple, *TCPServerConn]
	framed                 bool
	useSymmetricEncryption bool
	encryptionPassword     []byte
	asymmetricEncryption   *encryption.AsymmetricEncryption
}

func (tcp *TCPServer) IsAlive() bool {
	return tcp.isAlive
}

func (tcp *TCPServer) GetAddress() string {
	return tcp.address.String()
}

/*
Creates new TCP Server but does not starts it
*/
func NewTCPServer(address string, readFunc TCPServerReadFunc, reportTraffic bool, framed bool) (*TCPServer, error) {
	// Make address
	addressObj, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}
	return &TCPServer{address: addressObj, readFunc: readFunc, Logger: webtools.NewConsoleLoggerForTraffic("TCPServer", reportTraffic), conns: webtools.MakeSafeMap[*TCPClientSimple, *TCPServerConn](), framed: framed}, nil
}

/*
Setups symmetric encryption, it is strongly recommended to use encryption with framed connection
*/
func (tcp *TCPServer) SetupSymmetricEncryption(useEncryption bool, password []byte) {
	tcp.useSymmetricEncryption = useEncryption
	if useEncryption {
		tcp.encryptionPassword = password
	} else {
		tcp.encryptionPassword = nil
	}
}

/*
Setups asymmetric encryption, it is strongly recommended to use encryption with framed connection
*/
func (sv *TCPServer) SetupAsymmetricEncryption(useEncryption bool, useSaving bool, privateKeyPath string, publicKeyPath string) {
	if useEncryption {
		var err error
		sv.asymmetricEncryption, err = encryption.LoadOrCreateAsymmetricEncryption(true, privateKeyPath, publicKeyPath)
		if err != nil {
			sv.Logger.Log(3, "Error setting up asymmetric encryption: "+err.Error())
			return
		}
		if useSaving {
			sv.asymmetricEncryption.SaveAsymmetricEncryption(privateKeyPath, publicKeyPath)
		}
	} else {
		sv.asymmetricEncryption = nil
	}
}

/*
Starts TCP Server. Locks execution thread
*/
func (tcp *TCPServer) Start() {
	// Check if already running
	if tcp.isAlive {
		return
	}

	// Reset stop request
	tcp.requestedStop = false

	// Open listener
	var err error
	tcp.listener, err = net.ListenTCP("tcp", tcp.address)
	if err != nil {
		tcp.Logger.Log(3, "Error listening to "+tcp.address.String()+" with error: "+err.Error())
		return
	}
	tcp.isAlive = true
	tcp.Logger.Log(2, "Started listening on "+tcp.address.String())

	// Listener loop
	for !tcp.requestedStop {
		conn, err2 := tcp.listener.AcceptTCP()
		if err2 != nil {
			if tcp.requestedStop {
				// Ignore all errors
				break
			} else {
				tcp.Logger.Log(3, "Error accepting connection: "+err2.Error())
			}
		}

		// Handle connection
		cl := NewTCPClientSimpleFromConnection(conn, webtools.FormatByBool(tcp.framed, 0, -1), false, tcp.readFuncLocal, false)
		cl.SetLogger(tcp.Logger)
		cl.SetupSymmetricEncryption(tcp.useSymmetricEncryption, tcp.encryptionPassword)
		cl.SetAsymmetricEncryption(tcp.asymmetricEncryption)
		cl.GetTCPClientUniversal().IsServerClient = true
		cl.Connect()
	}
	tcp.isAlive = false
}

func (tcp *TCPServer) readFuncLocal(client *TCPClientSimple, data []byte, status uint8) {
	// Sort connection
	var tcpConn *TCPServerConn = tcp.conns.Get(client)
	if tcpConn == nil {
		tcpConn = &TCPServerConn{origin: tcp, Client: client}
		tcp.conns.Set(client, tcpConn)
	}
	if status == webtools.TCP_DISCONNECT_STATUS {
		tcp.conns.Delete(client)
	}
	tcp.Logger.Log(0, "Count of connections: "+strconv.Itoa(tcp.conns.Len()))

	// Process read
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

	// Request stop
	tcp.requestedStop = true
	err := tcp.listener.Close()
	time.Sleep(1 * time.Second)
	if err != nil {
		tcp.Logger.Log(3, "Error stopping TCP server: "+err.Error())
	}
}
