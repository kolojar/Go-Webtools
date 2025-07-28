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
func (client *TCPClient) Connect() net.Conn {
	//Connect to server
	var err error
	client.connection, err = net.Dial("tcp", client.address)
	if err != nil {
		client.Logger.Log(3, "Error connecting to: "+client.address+" | Error: "+err.Error())
		return nil
	}

	//Handle connection
	client.Logger.Log(2, "Connected to server at "+client.address+"!")
	go handleTCPRead(client.connection, client.readFunc, &client.Logger, client.useEncryption, client.encryptionPassword)
	return client.connection
}

/*
TCP Client writes to connection (TCP server)
*/
func (client *TCPClient) WriteToTCPServer(message string) {
	writeToTCP(client.connection, message, &client.Logger, client.useEncryption, client.encryptionPassword)
}

func (client *TCPClient) Close() error {
	if client == nil || client.connection == nil {
		return nil
	}
	return client.connection.Close()
}
