/*
Package tcp provides tools for handeling TCP traffic
*/
package tcp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"

	"webtools"
)

/*
ClientSimpleReadFunc is function definition for reading data from ClientSimple
*/
type ClientSimpleReadFunc func(client *ClientSimple, data []byte, status uint8)

/*
ClientSimple is simple TCP Client that allows read basic not framed streams and basic framed streams with internal framing implementation.
For advanced or custom framing use universal TCP client and setup all functions as you want.
If you do not know how to use universal client, feel free to copy this one and edit it as you want. This client features 2 functions of manipulating with connections - not framed and framed
*/
type ClientSimple struct {
	readFunc        ClientSimpleReadFunc
	universalClient *ClientUniversal
}

/*
IsAlive gets if server is alive
*/
func (cl *ClientSimple) IsAlive() bool {
	return cl.universalClient.IsAlive()
}

/*
GetConn gets raw TCP connection
*/
func (cl *ClientSimple) GetConn() *net.TCPConn {
	return cl.universalClient.GetConn()
}

/*
NewClientSimple creates new simple TCP Client but does not starts it
Set countOfNotFramedReads to negative number to read not framed stream infinitelly, set it to 0 to start reading framed immediately
*/
func NewClientSimple(address string, countOfNotFramedReads int, writeOneLastNoFrame bool, readFunc ClientSimpleReadFunc, reportTraffic bool) (*ClientSimple, error) {
	cl := &ClientSimple{readFunc: readFunc}
	var err error
	cl.universalClient, err = NewTCPClientUniversal(address, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.universalClient.Logger = webtools.NewConsoleLoggerForTraffic("TCPClientSimple", reportTraffic)
	cl.generateReadFuncStructure(countOfNotFramedReads, writeOneLastNoFrame)
	return cl, nil
}

/*
NewClientSimpleFromConnection creates new simple TCP Client from existing connection but does not starts reading
Set countOfNotFramedReads to negative number to read not framed stream infinitelly, set it to 0 to start reading framed immediately
Set writeOneLastNoFrame to true to write one last write before switching to other function using old not framed function
*/
func NewClientSimpleFromConnection(conn *net.TCPConn, countOfNotFramedReads int, writeOneLastNoFrame bool, readFunc ClientSimpleReadFunc, reportTraffic bool) *ClientSimple {
	cl := &ClientSimple{readFunc: readFunc}
	cl.universalClient = NewTCPClientUniversalFromConnection(conn, reportTraffic)
	cl.universalClient.Logger.Prefix = "TCPClientSimple"
	cl.generateReadFuncStructure(countOfNotFramedReads, writeOneLastNoFrame)
	return cl
}

/*
SetLogger sets logger
*/
func (cl *ClientSimple) SetLogger(logger *webtools.ConsoleLogger) {
	cl.universalClient.Logger = logger
}

/*
GetLogger gets logger
*/
func (cl *ClientSimple) GetLogger() *webtools.ConsoleLogger {
	return cl.universalClient.Logger
}

/*
SetupEncryption setups encryption, it is strongly recommended to use encryption with framed connection
*/
func (cl *ClientSimple) SetupEncryption(useEncryption bool, password []byte) {
	cl.universalClient.SetupEncryption(useEncryption, password)
}

// Generates HanderFuncs for universal client, this function is not needed but because I have 2 constructors, I do not want to repeat code
func (cl *ClientSimple) generateReadFuncStructure(noFrameCount int, writeOneLastNoFrame bool) {
	// Generate not framed handler
	cl.universalClient.HandlerFuncs = append(cl.universalClient.HandlerFuncs,
		ClientUniversalHanderFuncs{
			UseCount:               noFrameCount,        // Limit of not framed connections
			ReadHandler:            HandleTCPRead,       // Function resposible for handleling reading from connection, has loop for connection limit
			ReadFunc:               cl.readFuncLocal,    // Function that handles read events from prevous function
			WriteHandler:           WriteToTCPHandler,   // Founctionm responsible for writing to connection
			CanOneWriteAfterSwitch: writeOneLastNoFrame, // Writes one last write after switching to other functions
		})

	// Generate framed handler
	cl.universalClient.HandlerFuncs = append(cl.universalClient.HandlerFuncs,
		ClientUniversalHanderFuncs{
			UseCount:               -1,                      // Limit of framed connections
			ReadHandler:            HandleTCPReadFramed,     // Function resposible for handleling reading from connection, has loop for connection limit
			ReadFunc:               cl.readFuncLocal,        // Function that handles read events from prevous function
			WriteHandler:           WriteToTCPFramedHandler, // Founctionm responsible for writing to connection
			CanOneWriteAfterSwitch: false,                   // Writes one last write after switching to other functions
		})
}

/*
Connect connects to TCP server and start reading loop, does not locks execution thread
*/
func (cl *ClientSimple) Connect() bool {
	return cl.universalClient.Connect()
}

/*
HandleTCPRead handles TCP Read
Implements this function: type TCPClientUniversalReadHandlerFunc func(conn *net.TCPConn, limit int, logger *ConsoleLogger, readFunc TCPClientUniversalOnReadFuncIntenal) (bool, error)
*/
func HandleTCPRead(cl *ClientUniversal, limit int, _ *webtools.ConsoleLogger, readFunc ClientUniversalOnReadFuncIntenal) (bool, error) {
	for i := 0; i < limit || limit < 0; i++ {
		buffer := make([]byte, webtools.BUFFER_SIZE)
		n, err := cl.GetConn().Read(buffer)
		if err != nil {
			// Exit on errors
			return true, err
		}

		// Process read
		data := buffer[:n]
		readFunc(data, nil)
	}
	return false, nil
}

/*
HandleTCPReadFramed handles TCP Read with frame support
Implements this function: type TCPClientUniversalReadHandlerFunc func(conn *net.TCPConn, limit int, logger *ConsoleLogger, readFunc TCPClientUniversalOnReadFuncIntenal) (bool, error)
*/
func HandleTCPReadFramed(cl *ClientUniversal, limit int, _ *webtools.ConsoleLogger, readFunc ClientUniversalOnReadFuncIntenal) (bool, error) {
	var data []byte
	var n int
	for i := 0; i < limit || limit < 0; i++ {
		// Get frame size
		sizeOfBuffer := make([]byte, 4)
		_, err := io.ReadFull(cl.GetConn(), sizeOfBuffer)
		if err != nil {
			return true, errors.New("error reading frame header")
		}
		lengthOfBuffer := binary.BigEndian.Uint32(sizeOfBuffer)

		// Read data
		data = make([]byte, lengthOfBuffer)
		_, err = io.ReadFull(cl.GetConn(), data)
		if err != nil {
			// Exit on errors
			return true, err
		}

		// Process read
		readFunc(data, nil)
	}
	readFunc(data[:n], map[string]any{"restData": true})
	return false, nil
}

/*
WriteToTCPHandler handles TCP Write
Implements: type TCPClientUniversalOnWriteHandlerFunc func(cl *TCPClientUniversal, data []byte, otherData map[string]any) error
*/
func WriteToTCPHandler(cl *ClientUniversal, data []byte, _ map[string]any) error {
	// Write
	_, err := cl.GetConn().Write(data)
	return err
}

/*
WriteToTCPFramedHandler handles TCP Write with frames
Implements: type TCPClientUniversalOnWriteHandlerFunc func(cl *TCPClientUniversal, data []byte, otherData map[string]any) error
*/
func WriteToTCPFramedHandler(cl *ClientUniversal, data []byte, otherData map[string]any) error {
	var frame bytes.Buffer
	err := binary.Write(&frame, binary.BigEndian, uint32(len(data)))
	if err != nil {
		return err
	}
	frame.Write(data)
	return WriteToTCPHandler(cl, frame.Bytes(), otherData)
}

/*
Helper function for read event fired by read handler (used just as redirector)
Implements: type TCPClientUniversalOnReadFunc func(cl *TCPClientUniversal, data []byte, status uint8, otherData map[string]any)
*/
func (cl *ClientSimple) readFuncLocal(_ *ClientUniversal, data []byte, status uint8, _ map[string]any) {
	// Process read - redirect read data to main read function
	if cl.readFunc != nil {
		cl.readFunc(cl, data, status)
	}
}

/*
Send sends data to server
*/
func (cl *ClientSimple) Send(data []byte) {
	cl.universalClient.Send(data, nil)
}

/*
Stop stops TCP client
*/
func (cl *ClientSimple) Stop() {
	cl.universalClient.Stop()
}
