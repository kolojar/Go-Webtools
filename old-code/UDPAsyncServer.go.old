package webtools

import (
	"net"
	"strings"
	"time"
)

/*
Standardized type of function
*UDPClient = Source client, ignore on server side
*net.UDPAddr = Address of connection
String = message
Bool = is ended
*/
type UDPReadFuncClient func(*UDPClient, *net.UDPAddr, string, bool)

/*
Standardized type of function
*net.UDPAddr = Address of connection
String = message
Bool = is ended
*/
type UDPReadFunc func(*net.UDPAddr, string, bool)

/*
Basic UDP server
*/
type UDPServer struct {
	address  string
	readFunc UDPReadFunc
	//Prefix   string
	Logger             ConsoleLogger
	listener           *net.UDPConn
	selfStop           bool
	isAlive            bool
	useEncryption      bool
	encryptionPassword string
}

/*
Gets adderss of running UDP server
*/
func (udp *UDPServer) GetAddress() string {
	return udp.address
}

/*
Returns if server is alive
*/
func (udp UDPServer) IsAlive() bool {
	return udp.isAlive
}

/*
Constructs new instance of UDP Server but does not start it
*/
func MakeUDPServer(address string, readFunc UDPReadFunc, useEncryption bool, encryptionPassword string) UDPServer {
	return UDPServer{address: address, readFunc: readFunc, Logger: MakeConsoleLogger("UDPServer"), selfStop: false, useEncryption: useEncryption, encryptionPassword: encryptionPassword}
}

func (udp *UDPServer) Start() bool {
	udp.selfStop = false
	//Create UDP address
	address, err2 := net.ResolveUDPAddr("udp", udp.address)
	if err2 != nil {
		udp.Logger.Log(3, "Error listening to: "+err2.Error())
		return false
	}

	//Open UDP listener
	var err error
	udp.listener, err = net.ListenUDP("udp", address)
	if err != nil {
		udp.Logger.Log(3, "Error listening to: "+err.Error())
		return false
	}

	//defer listener.Close()
	udp.isAlive = true
	udp.Logger.Log(2, "Started on: "+udp.address)

	//Start infinite loop for accepting connections
	for !udp.selfStop {
		handleUDPRead(udp.listener, udp.readFunc, &udp.Logger, udp.useEncryption, udp.encryptionPassword)
	}

	//Stop
	//tcp.Logger.Log(2, "Stopped on: "+tcp.address)
	udp.isAlive = false
	udp.Logger.Log(2, "Stopped!")
	return true
}

func handleUDPRead(udpConn *net.UDPConn, readFunc UDPReadFunc, logger *ConsoleLogger, useEncryption bool, encryptionPassword string) {
	//Read data from TCP
	buffer := make([]byte, 16384)
	//scanner := bufio.NewScanner()
	//println("Created scanner!")
	var addr *net.UDPAddr
	var firstRead bool = true
	for {
		var n int
		var err error
		n, addr, err = udpConn.ReadFromUDP(buffer)
		if err != nil {
			if err.Error() != "EOF" && !strings.Contains(err.Error(), "use of closed network connection") && !strings.Contains(err.Error(), "i/o timeout") {
				logger.Log(3, "Error reading from: "+addr.String()+" | Error: "+err.Error())
			}
			break
		}
		if firstRead {
			firstRead = false
			logger.Log(1, "Connection from: "+addr.String())
		}

		//text := scanner.Text()
		//textParts := strings.Split(string(buffer[:n]), string(rune(23)))
		//for i := 0; i < calcLenOfTextParts(textParts)-1; i++ {
		//text := textParts[i]
		//text := strings.Join(textParts[0:(calcLenOfTextParts(textParts)-1)], string(rune(23)))
		text := string(buffer[:n])
		logger.Log(1, "Reading from: "+addr.String()+" | Data: "+text)
		var decrypt string
		var err2 error
		if useEncryption {
			decrypt, err2 = DecryptText(encryptionPassword, text)
			if err2 != nil {
				logger.Log(3, "Error decrypting message: "+err2.Error())
			}
			logger.Log(0, "Decrypted received message: "+decrypt)
		} else {
			decrypt = text
		}
		if readFunc != nil {
			readFunc(addr, decrypt, false)
		}
		//}
	}

	//Report error
	if !firstRead {
		logger.Log(1, "Disconecting from: "+addr.String())
		readFunc(addr, "", true)
	}
	//err := scanner.Err()
	//if err != nil {
	//println(prefix + ": Error reading from: " + conn.RemoteAddr().String() + " | Error: " + err.Error())
	//}
}

func writeToUDP(isServer bool, listener *net.UDPConn, addr *net.UDPAddr, message string, logger *ConsoleLogger, useEncryption bool, encryptionPassword string) {
	var msg string
	if useEncryption {
		logger.Log(0, "Decrypted sending message: "+message)
		var err error
		msg, err = EncryptText(encryptionPassword, message)
		if err != nil {
			logger.Log(3, "Error encrypting message: "+msg)
		}
	} else {
		msg = message
	}

	logger.Log(1, "Sending to: "+addr.String()+" | Data: "+msg)
	var err error
	if isServer {
		_, err = listener.WriteToUDP([]byte(msg), addr)
	} else {
		_, err = listener.Write([]byte(msg))
	}
	if err != nil {
		logger.Log(3, "Error senting to: "+addr.String()+" | Error: "+err.Error())
	}
}

func (udp *UDPServer) WriteToClient(addr *net.UDPAddr, message string) {
	writeToUDP(true, udp.listener, addr, message, &udp.Logger, udp.useEncryption, udp.encryptionPassword)
}

func (udp *UDPServer) Stop() error {
	if !udp.IsAlive() {
		return nil
	}
	udp.selfStop = true
	_ = udp.selfStop //For removal of not valid error
	err := udp.listener.Close()
	time.Sleep(1 * time.Second)
	if err != nil {
		udp.Logger.Log(3, "Error stopping UDP server: "+err.Error())
	}
	return err
}
