package udpTools

import (
	"net"
	"webtools"
)

/*
Standardized type of function
*UDPClient = Client
String = message
Bool = is ended
*/
type UDPClientReadFunc func(client *UDPClient, sourceAddress *net.UDPAddr, data []byte, ended bool)

/*
Basic UDP Client
*/
type UDPClient struct {
	readFunc  UDPClientReadFunc
	Logger    *webtools.ConsoleLogger
	Conn      *net.UDPConn
	address   *net.UDPAddr
	isAlive   bool
	udpFramer *UDPFramer
}

func (udp *UDPClient) IsAlive() bool {
	return udp.isAlive
}

/*
Creates new UDP Client but does not starts it
*/
func NewUDPClient(address string, readFunc UDPClientReadFunc, reportTraffic bool) (*UDPClient, error) {
	//Make address
	addressObj, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	//Make client
	return &UDPClient{address: addressObj, Logger: webtools.NewConsoleLoggerForTraffic("UDPClient", reportTraffic), readFunc: readFunc}, nil
}

/*
Connects to TCP server and start reading loop, does not locks execution thread
*/
func (udp *UDPClient) Connect() {
	//Dial
	var err error
	udp.Conn, err = net.DialUDP("udp", nil, udp.address)
	if err != nil {
		udp.Logger.Log(3, "Error connecting to: "+udp.address.String()+" | Error: "+err.Error())
		return
	}
	go func() {
		udp.isAlive = true
		//Handle read
		var ok bool = true
		for ok {
			ok = handleUDPRead(udp.Conn, udp.Logger, func(addrFrom *net.UDPAddr, data []byte, ended bool) {
				processDataForUDP(addrFrom, data, ended, udp.readFuncLocal, udp.Logger, udp.udpFramer, false, udp.Conn)
			})
		}
		udp.isAlive = false
		udp.readFuncLocal(nil, nil, true)
	}()
}

func (udp *UDPClient) readFuncLocal(addrFrom *net.UDPAddr, data []byte, ended bool) {
	if udp.readFunc != nil {
		udp.readFunc(udp, addrFrom, data, ended)
	}
	return

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
Sends data to server
*/
func (udp *UDPClient) Send(data []byte) {
	processSendForUDP(false, udp.Conn, udp.address, data, udp.Logger, udp.udpFramer)
}

/*
Stops TCP client
*/
func (udp *UDPClient) Stop() {
	if udp.Conn == nil || !udp.isAlive {
		//Invalid connection
		return
	}

	//Close
	udp.Logger.Log(1, "Requested disconnect from: "+udp.address.String())
	err := udp.Conn.Close()
	if err != nil {
		udp.Logger.Log(3, "Error disconnecting from: "+udp.address.String()+" | Error: "+err.Error())
	}
}
