package webtools

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
)

type TCPClientSimpleReadFunc func(client *TCPClientSimple, data []byte, status uint8)

/*
Simple TCP Client that allows read basic not framed streams and basic framed streams with internal framing implementation.
For advanced or custom framing use universal TCP client and setup all functions as you want.
If you do not know how to use universal client, feel free to copy this one and edit it as you want. This client features 2 functions of manipulating with connections - not framed and framed
*/
type TCPClientSimple struct {
	readFunc        TCPClientSimpleReadFunc
	universalClient *TCPClientUniversal
}

func (tcp *TCPClientSimple) IsAlive() bool {
	return tcp.universalClient.IsAlive()
}

func (tcp *TCPClientSimple) GetConn() *net.TCPConn {
	return tcp.universalClient.GetConn()
}

/*
Creates new simple TCP Client but does not starts it
Set countOfNotFramedReads to negative number to read not framed stream infinitelly, set it to 0 to start reading framed immediately
*/
func NewTCPClientSimple(address string, countOfNotFramedReads int, writeOneLastNoFrame bool, readFunc TCPClientSimpleReadFunc, reportTraffic bool) (*TCPClientSimple, error) {
	cl := &TCPClientSimple{readFunc: readFunc}
	var err error
	cl.universalClient, err = NewTCPClientUniversal(address, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.universalClient.Logger = NewConsoleLoggerForTraffic("TCPClientSimple", reportTraffic)
	cl.generateReadFuncStructure(countOfNotFramedReads, writeOneLastNoFrame)
	return cl, nil
}

/*
Creates new simple TCP Client from existing connection but does not starts reading
Set countOfNotFramedReads to negative number to read not framed stream infinitelly, set it to 0 to start reading framed immediately
Set writeOneLastNoFrame to true to write one last write before switching to other function using old not framed function
*/
func NewTCPClientSimpleFromConnection(conn *net.TCPConn, countOfNotFramedReads int, writeOneLastNoFrame bool, readFunc TCPClientSimpleReadFunc, reportTraffic bool) *TCPClientSimple {
	cl := &TCPClientSimple{readFunc: readFunc}
	cl.universalClient = NewTCPClientUniversalFromConnection(conn, reportTraffic)
	cl.universalClient.Logger.Prefix = "TCPClientSimple"
	cl.generateReadFuncStructure(countOfNotFramedReads, writeOneLastNoFrame)
	return cl
}

func (cl *TCPClientSimple) SetLogger(logger *ConsoleLogger) {
	cl.universalClient.Logger = logger
}

func (cl *TCPClientSimple) GetLogger() *ConsoleLogger {
	return cl.universalClient.Logger
}

// Generates HanderFuncs for universal client, this function is not needed but because I have 2 constructors, I do not want to repeat code
func (cl *TCPClientSimple) generateReadFuncStructure(noFrameCount int, writeOneLastNoFrame bool) {
	//Generate not framed handler
	cl.universalClient.HandlerFuncs = append(cl.universalClient.HandlerFuncs,
		FiveValuePair[int, TCPClientUniversalReadHandlerFunc, TCPClientUniversalOnReadFunc, TCPClientUniversalOnWriteHandlerFunc, bool]{
			A: noFrameCount,        //Limit of not framed connections
			B: handleTCPRead,       //Function resposible for handleling reading from connection, has loop for connection limit
			C: cl.readFuncLocal,    //Function that handles read events from prevous function
			D: writeToTCPHandler,   //Founctionm responsible for writing to connection
			E: writeOneLastNoFrame, //Writes one last write after switching to other functions
		})

	//Generate framed handler
	cl.universalClient.HandlerFuncs = append(cl.universalClient.HandlerFuncs,
		FiveValuePair[int, TCPClientUniversalReadHandlerFunc, TCPClientUniversalOnReadFunc, TCPClientUniversalOnWriteHandlerFunc, bool]{
			A: -1,                      //Limit of framed connections
			B: handleTCPReadFramed,     //Function resposible for handleling reading from connection, has loop for connection limit
			C: cl.readFuncLocal,        //Function that handles read events from prevous function
			D: writeToTCPFramedHandler, //Founctionm responsible for writing to connection
			E: false,                   //Writes one last write after switching to other functions
		})
}

/*
Connects to TCP server and start reading loop, does not locks execution thread
*/
func (tcp *TCPClientSimple) Connect() {
	tcp.universalClient.Connect()
}

/*
Handles TCP Read
Implements this function: type TCPClientUniversalReadHandlerFunc func(conn *net.TCPConn, limit int, logger *ConsoleLogger, readFunc TCPClientUniversalOnReadFuncIntenal) (bool, error)
*/
func handleTCPRead(cl *TCPClientUniversal, limit int, logger *ConsoleLogger, readFunc TCPClientUniversalOnReadFuncIntenal) (bool, error) {
	buffer := make([]byte, BUFFER_SIZE)
	for i := 0; i < limit || limit < 0; i++ {
		n, err := cl.GetConn().Read(buffer)
		if err != nil {
			//Exit on errors
			return true, err
		}

		//Process read
		data := buffer[:n]
		readFunc(data, nil)
	}
	return false, nil
}

/*
Handles TCP Read with frame support
Implements this function: type TCPClientUniversalReadHandlerFunc func(conn *net.TCPConn, limit int, logger *ConsoleLogger, readFunc TCPClientUniversalOnReadFuncIntenal) (bool, error)
*/
func handleTCPReadFramed(cl *TCPClientUniversal, limit int, logger *ConsoleLogger, readFunc TCPClientUniversalOnReadFuncIntenal) (bool, error) {
	var data []byte
	var n int
	for i := 0; i < limit || limit < 0; i++ {
		//Get frame size
		sizeOfBuffer := make([]byte, 4)
		_, err := io.ReadFull(cl.GetConn(), sizeOfBuffer)
		if err != nil {
			return true, errors.New("error reading frame header")
		}
		lengthOfBuffer := binary.BigEndian.Uint32(sizeOfBuffer)

		//Read data
		data = make([]byte, lengthOfBuffer)
		_, err = io.ReadFull(cl.GetConn(), data)
		if err != nil {
			//Exit on errors
			return true, err
		}

		//Process read
		readFunc(data, nil)
	}
	readFunc(data[:n], map[string]any{"restData": true})
	return false, nil
}

/*
Handles TCP Write
Implements: type TCPClientUniversalOnWriteHandlerFunc func(cl *TCPClientUniversal, data []byte, otherData map[string]any) error
*/
func writeToTCPHandler(cl *TCPClientUniversal, data []byte, otherData map[string]any) error {
	//Write
	_, err := cl.GetConn().Write(data)
	return err
}

/*
Handles TCP Write with frames
Implements: type TCPClientUniversalOnWriteHandlerFunc func(cl *TCPClientUniversal, data []byte, otherData map[string]any) error
*/
func writeToTCPFramedHandler(cl *TCPClientUniversal, data []byte, otherData map[string]any) error {
	var frame bytes.Buffer
	err := binary.Write(&frame, binary.BigEndian, uint32(len(data)))
	if err != nil {
		return err
	}
	frame.Write(data)
	return writeToTCPHandler(cl, frame.Bytes(), otherData)
}

/*
Helper function for read event fired by read handler (used just as redirector)
Implements: type TCPClientUniversalOnReadFunc func(cl *TCPClientUniversal, data []byte, status uint8, otherData map[string]any)
*/
func (tcp *TCPClientSimple) readFuncLocal(cl *TCPClientUniversal, data []byte, status uint8, otherData map[string]any) {
	//Process read - redirect read data to main read function
	if tcp.readFunc != nil {
		tcp.readFunc(tcp, data, status)
	}
}

/*
Sends data to server
*/
func (tcp *TCPClientSimple) Send(data []byte) {
	tcp.universalClient.Send(data, nil)
}

/*
Stops TCP client
*/
func (tcp *TCPClientSimple) Stop() {
	tcp.universalClient.Stop()
}
