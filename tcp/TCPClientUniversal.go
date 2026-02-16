package tcp

import (
	"encoding/hex"
	"net"
	"strconv"
	"time"

	"webtools"
	"webtools/encryption"
)

/*
ClientUniversalReadHandlerFunc must run in loop, negative limit means infinite loop, 0 means no read
Return true on connection close and error
Please use limit as count of read connections, when limit is equal to count of read connections, finish read and exit read func, do not end connection! If some read error occures, return true, TCP Client will handle closing of connection
Do not forget to use logging. 0 = Traffic, 1 = Generic info, 2 = Warnings = Connect / Disconnect / Others..., 3 = Errors
*/
type ClientUniversalReadHandlerFunc func(cl *ClientUniversal, limit int, logger *webtools.ConsoleLogger, readFunc ClientUniversalOnReadFuncIntenal) (bool, error)

/*
ClientUniversalOnReadFunc is used for calling event on read
*/
type ClientUniversalOnReadFunc func(cl *ClientUniversal, data []byte, status webtools.NetworkStatus, otherData map[string]any)

/*
ClientUniversalOnWriteHandlerFunc is write handler
Do not forget to use logging. 0 = Traffic, 1 = Generic info, 2 = Warnings = Connect / Disconnect / Others..., 3 = Errors
*/
type ClientUniversalOnWriteHandlerFunc func(cl *ClientUniversal, data []byte, otherData map[string]any) error

/*
ClientUniversalOnReadFuncIntenal is used for calling internal event on read - used for unpacking frames for example
Do not forget to use logging. 0 = Traffic, 1 = Generic info, 2 = Warnings = Connect / Disconnect / Others..., 3 = Errors
*/
type ClientUniversalOnReadFuncIntenal func(data []byte, otherData map[string]any)

/*
ClientUniversalHanderFuncs is function set of one type for reading and writing
*/
type ClientUniversalHanderFuncs struct {
	UseCount               int
	ReadHandler            ClientUniversalReadHandlerFunc
	ReadFunc               ClientUniversalOnReadFunc
	WriteHandler           ClientUniversalOnWriteHandlerFunc
	CanOneWriteAfterSwitch bool
}

/*
ClientUniversal is completly universal TCP client, for example usage see tcp.ClientSimple
*/
type ClientUniversal struct {
	Logger  *webtools.ConsoleLogger
	conn    *net.TCPConn
	address *net.TCPAddr
	isAlive bool
	// First item is limit and last item is if old writer should be used for one more request after switching protocols
	HandlerFuncs                    []ClientUniversalHanderFuncs
	currentHandlers                 ClientUniversalHanderFuncs
	currentWriteHandler             ClientUniversalOnWriteHandlerFunc
	currentWriteHandlerWaitOneWrite bool
	switchWriteHandler              bool
	isPreparedWithConnection        bool
	useEncryption                   bool
	encryptionPassword              []byte
}

/*
IsAlive gets if server is alive
*/
func (cl *ClientUniversal) IsAlive() bool {
	return cl.isAlive
}

/*
GetConn gets raw TCP connection
*/
func (cl *ClientUniversal) GetConn() *net.TCPConn {
	return cl.conn
}

/*
GetAddress gets address of server
*/
func (cl *ClientUniversal) GetAddress() *net.TCPAddr {
	return cl.address
}

/*
NewTCPClientUniversal creates new TCP Client but does not starts it
To set up read and write mechanisms, append items to HandlerFuncs
*/
func NewTCPClientUniversal(address string, reportTraffic bool) (*ClientUniversal, error) {
	// Make address
	addressObj, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}

	// Make client
	return &ClientUniversal{address: addressObj, Logger: webtools.NewConsoleLoggerForTraffic("TCPClientUniversal", reportTraffic), HandlerFuncs: make([]ClientUniversalHanderFuncs, 0), isPreparedWithConnection: false}, nil
}

/*
NewTCPClientUniversalFromConnection creates new TCP Client from existing connection but does not starts reading
To set up read and write mechanisms, append items to HandlerFuncs
*/
func NewTCPClientUniversalFromConnection(conn *net.TCPConn, reportTraffic bool) *ClientUniversal {
	// Make client
	return &ClientUniversal{conn: conn, address: conn.RemoteAddr().(*net.TCPAddr), Logger: webtools.NewConsoleLoggerForTraffic("TCPClientUniversal", reportTraffic), HandlerFuncs: make([]ClientUniversalHanderFuncs, 0), isPreparedWithConnection: true}
}

/*
SetupEncryption setups encryption for universal TCP Client, it is strongly recommended to use encryption with framed connection
*/
func (cl *ClientUniversal) SetupEncryption(useEncryption bool, password []byte) {
	cl.useEncryption = useEncryption
	if useEncryption {
		cl.encryptionPassword = password
	} else {
		cl.encryptionPassword = nil
	}
}

/*
Connect connects to TCP server and start reading loop, does not locks execution thread
*/
func (cl *ClientUniversal) Connect() bool {
	// Dial
	if !cl.isPreparedWithConnection {
		var err error
		cl.conn, err = net.DialTCP("tcp", nil, cl.address)
		if err != nil {
			cl.Logger.Log(3, "Error connecting to: "+cl.address.String()+" | Error: "+err.Error())
			return false
		}
	}

	// Connect
	cl.Logger.Log(2, "Connected to: "+cl.conn.RemoteAddr().String()+" connected locally to: "+cl.conn.LocalAddr().String())
	cl.isAlive = true

	// Handle read
	go cl.readNextFunc()
	time.Sleep(500 * time.Millisecond)
	return true
}

func (cl *ClientUniversal) readNextFunc() {
	var firstValid = true
	for len(cl.HandlerFuncs) > 0 {
		// Get new
		newHandler := cl.HandlerFuncs[0]
		cl.HandlerFuncs = cl.HandlerFuncs[1:]

		// Skip invalid readers
		if newHandler.UseCount == 0 {
			continue
		}
		if newHandler.ReadHandler == nil {
			continue
		}

		// Inform about connect
		cl.switchWriteHandler = true
		cl.currentHandlers = newHandler
		if firstValid {
			firstValid = false
			cl.currentHandlers.ReadFunc(cl, nil, webtools.ConnectStatus, nil)
		}

		// Handle reading
		closed, err := cl.currentHandlers.ReadHandler(cl, cl.currentHandlers.UseCount, cl.Logger, cl.localReadFunc)
		if err != nil {
			// Error occured while reading
			cl.Logger.Log(3, "Error reading from: "+cl.conn.RemoteAddr().String()+" connected locally to: "+cl.conn.LocalAddr().String()+" | Error: "+err.Error())
			break
		}
		if closed {
			// Requested connection close
			break
		}

		// Inform about reached limit
		cl.currentHandlers.ReadFunc(cl, nil, webtools.FinishedReadFuncStatus, nil)
		cl.Logger.Log(1, "Switching read function")
	}

	// Finished all readers
	cl.Logger.Log(2, "Disconneted from: "+cl.conn.RemoteAddr().String()+" connected locally to: "+cl.conn.LocalAddr().String())
	if cl.currentHandlers.ReadFunc != nil {
		cl.currentHandlers.ReadFunc(cl, nil, webtools.DisconnectStatus, nil)
	}
	defer cl.conn.Close()
	cl.isAlive = false
}

// Local helper read function
func (cl *ClientUniversal) localReadFunc(data []byte, otherData map[string]any) {
	if cl.useEncryption {
		// Decrypt
		cl.Logger.Log(0, "Reading enrypted from: "+cl.conn.RemoteAddr().String()+" connected locally to: "+cl.conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data)+" | Other data: "+webtools.MapToString(otherData))
		var err error
		data, err = encryption.DecryptSymmetric([]byte(cl.encryptionPassword), data)
		if err != nil {
			cl.Logger.Log(3, "Error decrypting: "+err.Error())
			return
		}
	}

	// Read
	cl.Logger.Log(0, "Reading from: "+cl.conn.RemoteAddr().String()+" connected locally to: "+cl.conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data)+" | Other data: "+webtools.MapToString(otherData))
	if cl.currentHandlers.ReadFunc != nil {
		cl.currentHandlers.ReadFunc(cl, data, webtools.ReadDataStatus, otherData)
	}
}

/*
Send sends data to server
*/
func (cl *ClientUniversal) Send(data []byte, otherData map[string]any) {
	// Switch writers before one write
	if cl.switchWriteHandler {
		if !cl.currentWriteHandlerWaitOneWrite {
			// Switch when there is no pending one write
			cl.switchWriteHandler = false
			cl.currentWriteHandler = cl.currentHandlers.WriteHandler
			cl.currentWriteHandlerWaitOneWrite = cl.currentHandlers.CanOneWriteAfterSwitch
		}
	}

	// Invalid connection
	if cl.conn == nil {
		cl.Logger.Log(1, "Invalid connection, cancelling write.")
		return
	}

	// Write
	if cl.currentHandlers.WriteHandler != nil {
		cl.Logger.Log(0, "Writing to: "+cl.conn.RemoteAddr().String()+" connected locally to: "+cl.conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data)+" | Other data: "+webtools.MapToString(otherData))
		if cl.useEncryption {
			// Encrypt
			var err error
			data, err = encryption.EncryptSymmetric([]byte(cl.encryptionPassword), data)
			if err != nil {
				cl.Logger.Log(3, "Error encrypting: "+err.Error())
				return
			}
			cl.Logger.Log(0, "Writing enrypted from: "+cl.conn.RemoteAddr().String()+" connected locally to: "+cl.conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data)+" | Other data: "+webtools.MapToString(otherData))
		}

		// Write
		err := cl.currentHandlers.WriteHandler(cl, data, otherData)
		if err != nil {
			cl.Logger.Log(3, "Error writing to: "+cl.conn.RemoteAddr().String()+" connected locally to: "+cl.conn.LocalAddr().String()+" | Error: "+err.Error())
		}
	}

	// Switch writers after one write
	if cl.switchWriteHandler {
		if cl.currentWriteHandlerWaitOneWrite {
			// Switch when there was pending one write
			cl.switchWriteHandler = false
			cl.currentWriteHandler = cl.currentHandlers.WriteHandler
			cl.currentWriteHandlerWaitOneWrite = cl.currentHandlers.CanOneWriteAfterSwitch
		}
	}
}

/*
Stop stops TCP client
*/
func (cl *ClientUniversal) Stop() {
	if cl.conn == nil || !cl.isAlive {
		// Invalid connection
		return
	}

	// Close
	cl.Logger.Log(1, "Requested disconnect from: "+cl.address.String())
	err := cl.conn.Close()
	if err != nil {
		cl.Logger.Log(3, "Error disconnecting from: "+cl.address.String()+" | Error: "+err.Error())
	}
}
