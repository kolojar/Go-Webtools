package webtools

import "net"

type TCPClient struct {
	address            string
	readFunc           TCPReadFunc
	Logger             ConsoleLogger
	connection         net.Conn
	encryptionPassword string
	useEncryption      bool
}

/*
TCP Client connects to address and starts reading. Uses default prefix
*/
func MakeTCPClient(address string, readFunc TCPReadFunc, useEncryption bool, encryptionPassword string) TCPClient {
	return TCPClient{address: address, readFunc: readFunc, Logger: MakeConsoleLogger("TCPClient"), useEncryption: useEncryption, encryptionPassword: encryptionPassword}
}

/*
TCP Client connects to address and starts reading. Set prefix to "" for default
*/
func (tcp *TCPClient) Connect() net.Conn {
	//Create TCP address
	address, err2 := net.ResolveTCPAddr("tcp", tcp.address)
	if err2 != nil {
		tcp.Logger.Log(3, "Error listening to: "+err2.Error())
		return nil
	}

	//Connect to server
	var err error
	tcp.connection, err = net.DialTCP("tcp", nil, address)
	if err != nil {
		tcp.Logger.Log(3, "Error connecting to: "+tcp.address+" | Error: "+err.Error())
		return nil
	}

	//Handle connection
	tcp.Logger.Log(2, "Connected to server at "+tcp.address+"!")
	go handleTCPRead(tcp.connection, tcp.readFunc, &tcp.Logger, tcp.useEncryption, tcp.encryptionPassword)
	return tcp.connection
}

/*
Network Client writes to connection to TCP server
*/
func (tcp *TCPClient) WriteToServer(message string) {
	writeToTCP(tcp.connection, message, &tcp.Logger, tcp.useEncryption, tcp.encryptionPassword)
}

func (tcp *TCPClient) Close() error {
	if tcp == nil || tcp.connection == nil {
		return nil
	}
	return tcp.connection.Close()
}
