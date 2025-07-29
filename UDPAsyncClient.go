package webtools

import "net"

type UDPClient struct {
	address            string
	readFunc           UDPReadFunc
	readFuncClient     UDPReadFuncClient
	Logger             ConsoleLogger
	connection         *net.UDPConn
	encryptionPassword string
	useEncryption      bool
	addressObject      *net.UDPAddr
}

/*
UDP Client connects to address and starts reading. Uses default prefix
*/
func MakeUDPClient(address string, readFunc UDPReadFunc, useEncryption bool, encryptionPassword string) UDPClient {
	return UDPClient{address: address, readFunc: readFunc, readFuncClient: nil, Logger: MakeConsoleLogger("UDPClient"), useEncryption: useEncryption, encryptionPassword: encryptionPassword}
}

/*
UDP Client connects to address and starts reading. Uses default prefix. Adds option to get on read what client send message
*/
func MakeUDPClientAdvanced(address string, readFuncClient UDPReadFuncClient, useEncryption bool, encryptionPassword string) UDPClient {
	return UDPClient{address: address, readFunc: nil, readFuncClient: readFuncClient, Logger: MakeConsoleLogger("UDPClient"), useEncryption: useEncryption, encryptionPassword: encryptionPassword}
}

/*
Network Client connects to address and starts reading. Set prefix to "" for default
*/
func (udp *UDPClient) Connect() *net.UDPConn {
	//Create UDP address
	var err2 error
	udp.addressObject, err2 = net.ResolveUDPAddr("udp", udp.address)
	if err2 != nil {
		udp.Logger.Log(3, "Error listening to: "+err2.Error())
		return nil
	}

	//Connect to server
	var err error
	udp.connection, err = net.DialUDP("udp", nil, udp.addressObject)
	if err != nil {
		udp.Logger.Log(3, "Error connecting to: "+udp.address+" | Error: "+err.Error())
		return nil
	}

	//Handle connection
	udp.Logger.Log(2, "Connected to server at "+udp.address+"!")
	go handleUDPRead(udp.connection, udp.handleRead, &udp.Logger, udp.useEncryption, udp.encryptionPassword)
	return udp.connection
}

/*
Handles read for client
*/
func (udp *UDPClient) handleRead(addr *net.UDPAddr, data string, ended bool) {
	if udp.readFuncClient != nil {
		udp.readFuncClient(udp, addr, data, ended)
	}
	if udp.readFunc != nil {
		udp.readFunc(addr, data, ended)
	}
}

/*
Network Client writes to connection to UDP server
*/
func (client *UDPClient) WriteToServer(message string) {
	writeToUDP(false, client.connection, client.addressObject, message, &client.Logger, client.useEncryption, client.encryptionPassword)
}

func (client *UDPClient) Close() error {
	if client == nil || client.connection == nil {
		return nil
	}
	return client.connection.Close()
}
