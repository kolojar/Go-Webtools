package udp

import (
	"net"

	"github.com/kolojar/Go-Webtools/helpertools"
)

/*
ClientReadFunc is function definition for reading data from Client
*/
type ClientReadFunc func(client *Client, sourceAddress *net.UDPAddr, data []byte, ended bool)

/*
Client is basic UDP Client
*/
type Client struct {
	readFunc ClientReadFunc
	Logger   *helpertools.ConsoleLogger
	Conn     *net.UDPConn
	address  *net.UDPAddr
	isAlive  bool
}

/*
IsAlive gets if client is alive
*/
func (cl *Client) IsAlive() bool {
	return cl.isAlive
}

func (cl *Client) GetAddress() *net.UDPAddr {
	return cl.address
}

/*
NewClient creates new UDP Client but does not starts it
*/
func NewClient(address string, readFunc ClientReadFunc, reportTraffic bool) (*Client, error) {
	//Make address
	addressObj, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	//Make client
	return &Client{address: addressObj, Logger: helpertools.NewConsoleLoggerForTraffic("UDPClient", reportTraffic), readFunc: readFunc}, nil
}

/*
Connect connects to UDP server and start reading loop, does not locks execution thread
*/
func (cl *Client) Connect() error {
	if cl.isAlive {
		return nil
	}

	//Dial
	var err error
	cl.Conn, err = net.DialUDP("udp", nil, cl.address)
	if err != nil {
		cl.Logger.Log(3, "Error connecting to: "+cl.address.String()+" | Error: "+err.Error())
		return err
	}
	go func() {
		cl.isAlive = true
		//Handle read
		var ok = true
		for ok {
			ok = handleUDPRead(cl.Conn, cl.Logger, cl.readFuncLocal)
		}
		cl.isAlive = false
		cl.readFuncLocal(nil, nil, true)
	}()
	return nil
}

func (cl *Client) readFuncLocal(addrFrom *net.UDPAddr, data []byte, ended bool) {
	if cl.readFunc != nil {
		cl.readFunc(cl, addrFrom, data, ended)
	}
	//Sort if framed

	//Process read
	//if udp.readFunc != nil {
	//	if !ended {
	//		udp.Logger.Log(0, "Reading from: "+addr.String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
	//	}
	//	udp.readFunc(udp, data, ended)
	//}
}

/*
Send sends data to server
*/
func (cl *Client) Send(data []byte) {
	writeToUDP(false, cl.Conn, cl.address, data, cl.Logger)
}

/*
Stop stops TCP client
*/
func (cl *Client) Stop() {
	if cl.Conn == nil || !cl.isAlive {
		//Invalid connection
		return
	}

	//Close
	cl.Logger.Log(1, "Requested disconnect from: "+cl.address.String())
	err := cl.Conn.Close()
	if err != nil {
		cl.Logger.Log(3, "Error disconnecting from: "+cl.address.String()+" | Error: "+err.Error())
	}
}

// Close is alias for Stop
func (cl *Client) Close() {
	cl.Stop()
}

// GetLogger gets logger of client
func (cl *Client) GetLogger() *helpertools.ConsoleLogger {
	return cl.Logger
}
