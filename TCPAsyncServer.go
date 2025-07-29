package webtools

import (
	"net"
	"strings"
	"time"
)

/*
Standardized type of function
net.Conn = Connection
String = message
Bool = is ended
*/
type TCPReadFunc func(net.Conn, string, bool)

/*
Basic TCP server
*/
type TCPServer struct {
	address  string
	readFunc TCPReadFunc
	//Prefix   string
	Logger             ConsoleLogger
	listener           net.Listener
	selfStop           bool
	isAlive            bool
	useEncryption      bool
	encryptionPassword string
}

/*
Gets adderss of running TCP server
*/
func (tcp *TCPServer) GetAddress() string {
	return tcp.address
}

/*
Returns if server is alive
*/
func (tcp TCPServer) IsAlive() bool {
	return tcp.isAlive
}

/*
Constructs new instance of TCP Server but does not start it
*/
func MakeTCPServer(address string, readFunc TCPReadFunc, useEncryption bool, encryptionPassword string) TCPServer {
	return TCPServer{address: address, readFunc: readFunc, Logger: MakeConsoleLogger("TCPServer"), selfStop: false, useEncryption: useEncryption, encryptionPassword: encryptionPassword}
}

func (tcp *TCPServer) Start() bool {
	tcp.selfStop = false
	//Create TCP address
	//address, err2 := net.ResolveTCPAddr("tcp", tcp.address)
	//if err2 != nil {
	//	tcp.Logger.Log(3, "Error listening to: "+err2.Error())
	//	return false
	//}

	//Open TCP listener
	var err error
	tcp.listener, err = net.Listen("tcp", tcp.address)
	if err != nil {
		tcp.Logger.Log(3, "Error listening to: "+err.Error())
		return false
	}

	//defer listener.Close()
	tcp.isAlive = true
	tcp.Logger.Log(2, "Started on: "+tcp.address)

	//Start infinite loop for accepting connections
	for !tcp.selfStop {
		conn, err := tcp.listener.Accept()
		if err != nil {
			if tcp.selfStop {
				//Suppress all errors
				break
			} else {
				tcp.Logger.Log(3, "Error accepting connection: "+err.Error())
				continue
			}
		}

		go tcp.handleTCPServerConnection(conn)
	}

	//Stop
	//tcp.Logger.Log(2, "Stopped on: "+tcp.address)
	tcp.isAlive = false
	tcp.Logger.Log(2, "Stopped!")
	return true
}

//func calcLenOfTextParts(textParts []string) int {
//	lenOfParts := len(textParts)
//	if len(textParts) == 1 && len(textParts[0]) > 0 && textParts[0] != string(rune(23)) {
//		//Valid one part text
//		lenOfParts++
//	} else {
//		if len(textParts[len(textParts)-1]) > 0 && textParts[len(textParts)-1] != string(rune(23)) {
//			//Valid last part text
//			lenOfParts++
//		}
//	}
//	return lenOfParts
//}

func handleTCPRead(conn net.Conn, readFunc TCPReadFunc, logger *ConsoleLogger, useEncryption bool, encryptionPassword string) {
	//Read data from TCP
	buffer := make([]byte, 16384)
	//scanner := bufio.NewScanner()
	//println("Created scanner!")
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err.Error() != "EOF" && !strings.Contains(err.Error(), "use of closed network connection") {
				logger.Log(3, "Error reading from: "+conn.RemoteAddr().String()+" | Error: "+err.Error())
			}
			break
		}
		//text := scanner.Text()
		//textParts := strings.Split(string(buffer[:n]), string(rune(23)))
		//for i := 0; i < calcLenOfTextParts(textParts)-1; i++ {
		//text := textParts[i]
		//text := strings.Join(textParts[0:(calcLenOfTextParts(textParts)-1)], string(rune(23)))
		text := string(buffer[:n])
		logger.Log(1, "Reading from: "+conn.RemoteAddr().String()+" | Data: "+text)
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
		readFunc(conn, decrypt, false)
		//}
	}

	//Report error
	logger.Log(1, "Disconecting from: "+conn.RemoteAddr().String())
	readFunc(conn, "", true)
	defer conn.Close()
	//err := scanner.Err()
	//if err != nil {
	//println(prefix + ": Error reading from: " + conn.RemoteAddr().String() + " | Error: " + err.Error())
	//}
}

func writeToTCP(conn net.Conn, message string, logger *ConsoleLogger, useEncryption bool, encryptionPassword string) {
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

	logger.Log(1, "Sending to: "+conn.RemoteAddr().String()+" | Data: "+msg)
	_, err := conn.Write([]byte(msg))
	if err != nil {
		logger.Log(3, "Error senting to: "+conn.RemoteAddr().String()+" | Error: "+err.Error())
	}
}

func (tcp *TCPServer) WriteToClient(conn net.Conn, message string) {
	writeToTCP(conn, message, &tcp.Logger, tcp.useEncryption, tcp.encryptionPassword)
}

func (tcp *TCPServer) handleTCPServerConnection(conn net.Conn) {
	tcp.Logger.Log(1, "Connection from: "+conn.RemoteAddr().String())
	handleTCPRead(conn, tcp.readFunc, &tcp.Logger, tcp.useEncryption, tcp.encryptionPassword)
	//ServerWriteToTCPClient(conn, "Welcome to TCP server!")
}

func (tcp *TCPServer) Stop() error {
	if !tcp.IsAlive() {
		return nil
	}
	tcp.selfStop = true
	_ = tcp.selfStop //For removal of not valid error
	err := tcp.listener.Close()
	time.Sleep(1 * time.Second)
	if err != nil {
		tcp.Logger.Log(3, "Error stopping TCP server: "+err.Error())
	}
	return err
}
