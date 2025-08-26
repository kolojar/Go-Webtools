package tcptools

import (
	"encoding/hex"
	"net"
	"strconv"
	"time"
	"webtools"
	"webtools/encryption"
)

/*
Please use limit as count of read connections, when limit is equal to count of read connections, finish read and exit read func, do not end connection! If some read error occures, return true, TCP Client will handle closing of connection
Do not forget to use logging. 0 = Traffic, 1 = Generic info, 2 = Warnings = Connect / Disconnect / Others..., 3 = Errors
Read handler must run in loop, negative limit means infinite loop, 0 means no read
Return true on connection close and error
*/
type TCPClientUniversalReadHandlerFunc func(cl *TCPClientUniversal, limit int, logger *webtools.ConsoleLogger, readFunc TCPClientUniversalOnReadFuncIntenal) (bool, error)
type TCPClientUniversalOnReadFunc func(cl *TCPClientUniversal, data []byte, status uint8, otherData map[string]any)

/*
Do not forget to use logging. 0 = Traffic, 1 = Generic info, 2 = Warnings = Connect / Disconnect / Others..., 3 = Errors
*/
type TCPClientUniversalOnWriteHandlerFunc func(cl *TCPClientUniversal, data []byte, otherData map[string]any) error
type TCPClientUniversalOnReadFuncIntenal func(data []byte, otherData map[string]any)

type TCPClientUniversalHanderFuncs struct {
	UseCount               int
	ReadHandler            TCPClientUniversalReadHandlerFunc
	ReadFunc               TCPClientUniversalOnReadFunc
	WriteHandler           TCPClientUniversalOnWriteHandlerFunc
	CanOneWriteAfterSwitch bool
}

/*
Completly universal TCP client, for usage example see TCPClientSimple
*/
type TCPClientUniversal struct {
	Logger  *webtools.ConsoleLogger
	conn    *net.TCPConn
	address *net.TCPAddr
	isAlive bool
	//First item is limit and last item is if old writer should be used for one more request after switching protocols
	HandlerFuncs                    []TCPClientUniversalHanderFuncs
	currentHandlers                 TCPClientUniversalHanderFuncs
	currentWriteHandler             TCPClientUniversalOnWriteHandlerFunc
	currentWriteHandlerWaitOneWrite bool
	switchWriteHandler              bool
	isPreparedWithConnection        bool
	useEncryption                   bool
	encryptionPassword              string
}

func (tcp *TCPClientUniversal) IsAlive() bool {
	return tcp.isAlive
}

func (tcp *TCPClientUniversal) GetConn() *net.TCPConn {
	return tcp.conn
}

func (tcp *TCPClientUniversal) GetAddress() *net.TCPAddr {
	return tcp.address
}

/*
Creates new TCP Client but does not starts it
To set up read and write mechanisms, append items to HandlerFuncs
*/
func NewTCPClientUniversal(address string, reportTraffic bool) (*TCPClientUniversal, error) {
	//Make address
	addressObj, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}

	//Make client
	return &TCPClientUniversal{address: addressObj, Logger: webtools.NewConsoleLoggerForTraffic("TCPClientUniversal", reportTraffic), HandlerFuncs: make([]TCPClientUniversalHanderFuncs, 0), isPreparedWithConnection: false}, nil
}

/*
Creates new TCP Client from existing connection but does not starts reading
To set up read and write mechanisms, append items to HandlerFuncs
*/
func NewTCPClientUniversalFromConnection(conn *net.TCPConn, reportTraffic bool) *TCPClientUniversal {
	//Make client
	return &TCPClientUniversal{conn: conn, address: conn.RemoteAddr().(*net.TCPAddr), Logger: webtools.NewConsoleLoggerForTraffic("TCPClientUniversal", reportTraffic), HandlerFuncs: make([]TCPClientUniversalHanderFuncs, 0), isPreparedWithConnection: true}
}

/*
Setups encryption for universal TCP Client, it is strongly recommended to use encryption with framed connection
*/
func (tcp *TCPClientUniversal) SetupEncryption(useEncryption bool, password string) {
	tcp.useEncryption = useEncryption
	if useEncryption {
		tcp.encryptionPassword = password
	} else {
		tcp.encryptionPassword = ""
	}
}

/*
Connects to TCP server and start reading loop, does not locks execution thread
*/
func (tcp *TCPClientUniversal) Connect() bool {
	//Dial
	if !tcp.isPreparedWithConnection {
		var err error
		tcp.conn, err = net.DialTCP("tcp", nil, tcp.address)
		if err != nil {
			tcp.Logger.Log(3, "Error connecting to: "+tcp.address.String()+" | Error: "+err.Error())
			return false
		}
	}

	//Connect
	tcp.Logger.Log(2, "Connected to: "+tcp.conn.RemoteAddr().String()+" connected locally to: "+tcp.conn.LocalAddr().String())
	tcp.isAlive = true

	//Handle read
	go tcp.readNextFunc()
	time.Sleep(500 * time.Millisecond)
	return true
}

func (tcp *TCPClientUniversal) readNextFunc() {
	var firstValid bool = true
	for len(tcp.HandlerFuncs) > 0 {
		//Get new
		newHandler := tcp.HandlerFuncs[0]
		tcp.HandlerFuncs = tcp.HandlerFuncs[1:]

		//Skip invalid readers
		if newHandler.UseCount == 0 {
			continue
		}
		if newHandler.ReadHandler == nil {
			continue
		}

		//Inform about connect
		tcp.switchWriteHandler = true
		tcp.currentHandlers = newHandler
		if firstValid {
			firstValid = false
			tcp.currentHandlers.ReadFunc(tcp, nil, webtools.TCP_CONNECT_STATUS, nil)
		}

		//Handle reading
		close, err := tcp.currentHandlers.ReadHandler(tcp, tcp.currentHandlers.UseCount, tcp.Logger, tcp.localReadFunc)
		if err != nil {
			//Error occured while reading
			tcp.Logger.Log(3, "Error reading from: "+tcp.conn.RemoteAddr().String()+" connected locally to: "+tcp.conn.LocalAddr().String()+" | Error: "+err.Error())
			break
		}
		if close {
			//Requested connection close
			break
		}

		//Inform about reached limit
		tcp.currentHandlers.ReadFunc(tcp, nil, webtools.TCP_FINISHED_READ_FUNC_STATUS, nil)
		tcp.Logger.Log(1, "Switching read function")
	}

	//Finished all readers
	tcp.Logger.Log(2, "Disconneted from: "+tcp.conn.RemoteAddr().String()+" connected locally to: "+tcp.conn.LocalAddr().String())
	if tcp.currentHandlers.ReadFunc != nil {
		tcp.currentHandlers.ReadFunc(tcp, nil, webtools.TCP_DISCONNECT_STATUS, nil)
	}
	defer tcp.conn.Close()
	tcp.isAlive = false
}

// Local helper read function
func (tcp *TCPClientUniversal) localReadFunc(data []byte, otherData map[string]any) {
	if tcp.useEncryption {
		//Decrypt
		tcp.Logger.Log(0, "Reading enrypted from: "+tcp.conn.RemoteAddr().String()+" connected locally to: "+tcp.conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data)+" | Other data: "+webtools.MapToString(otherData))
		var err error
		data, err = encryption.DecryptSymmetric(tcp.encryptionPassword, data)
		if err != nil {
			tcp.Logger.Log(3, "Error decrypting: "+err.Error())
			return
		}
	}

	//Read
	tcp.Logger.Log(0, "Reading from: "+tcp.conn.RemoteAddr().String()+" connected locally to: "+tcp.conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data)+" | Other data: "+webtools.MapToString(otherData))
	if tcp.currentHandlers.ReadFunc != nil {
		tcp.currentHandlers.ReadFunc(tcp, data, webtools.TCP_READ_DATA_STATUS, otherData)
	}
}

/*
Sends data to server
*/
func (tcp *TCPClientUniversal) Send(data []byte, otherData map[string]any) {
	//Switch writers before one write
	if tcp.switchWriteHandler {
		if !tcp.currentWriteHandlerWaitOneWrite {
			//Switch when there is no pending one write
			tcp.switchWriteHandler = false
			tcp.currentWriteHandler = tcp.currentHandlers.WriteHandler
			tcp.currentWriteHandlerWaitOneWrite = tcp.currentHandlers.CanOneWriteAfterSwitch
		}
	}

	//Invalid connection
	if tcp.conn == nil {
		tcp.Logger.Log(1, "Invalid connection, cancelling write.")
		return
	}

	//Write
	if tcp.currentHandlers.WriteHandler != nil {
		tcp.Logger.Log(0, "Writing to: "+tcp.conn.RemoteAddr().String()+" connected locally to: "+tcp.conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data)+" | Other data: "+webtools.MapToString(otherData))
		if tcp.useEncryption {
			//Encrypt
			var err error
			data, err = encryption.EncryptSymmetric(tcp.encryptionPassword, data)
			if err != nil {
				tcp.Logger.Log(3, "Error encrypting: "+err.Error())
				return
			}
			tcp.Logger.Log(0, "Writing enrypted from: "+tcp.conn.RemoteAddr().String()+" connected locally to: "+tcp.conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data)+" | Other data: "+webtools.MapToString(otherData))
		}

		//Write
		err := tcp.currentHandlers.WriteHandler(tcp, data, otherData)
		if err != nil {
			tcp.Logger.Log(3, "Error writing to: "+tcp.conn.RemoteAddr().String()+" connected locally to: "+tcp.conn.LocalAddr().String()+" | Error: "+err.Error())
		}
	}

	//Switch writers after one write
	if tcp.switchWriteHandler {
		if tcp.currentWriteHandlerWaitOneWrite {
			//Switch when there was pending one write
			tcp.switchWriteHandler = false
			tcp.currentWriteHandler = tcp.currentHandlers.WriteHandler
			tcp.currentWriteHandlerWaitOneWrite = tcp.currentHandlers.CanOneWriteAfterSwitch
		}
	}
}

/*
Stops TCP client
*/
func (tcp *TCPClientUniversal) Stop() {
	if tcp.conn == nil || !tcp.isAlive {
		//Invalid connection
		return
	}

	//Close
	tcp.Logger.Log(1, "Requested disconnect from: "+tcp.address.String())
	err := tcp.conn.Close()
	if err != nil {
		tcp.Logger.Log(3, "Error disconnecting from: "+tcp.address.String()+" | Error: "+err.Error())
	}
}
