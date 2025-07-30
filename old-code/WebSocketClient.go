package webtools

import (
	"encoding/base64"
	"math/rand/v2"
	"net"
	"strings"
)

type WebSocketClient struct {
	address    string
	readFunc   TCPReadFunc
	Logger     ConsoleLogger
	connection net.Conn
}

/*
Generates random string
*/
func GenerateRandomString(lenght int) string {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	result := ""
	for i := 0; i < lenght; i++ {
		result += string(letters[rand.IntN(len(letters))])
	}
	return result
}

/*
Starts basic WebSocket client without using frames but with handshake support
*/
func MakeWebSocketClient(address string, readFunc TCPReadFunc) WebSocketClient {
	return WebSocketClient{address: address, readFunc: readFunc, Logger: MakeConsoleLogger("WebSocketClient")}
}

/*
Starts basic WebSocket client without using frames but with handshake support with custom prefix
*/
func (client *WebSocketClient) Connect() net.Conn {
	//Connect to server
	conn, err := net.Dial("tcp", client.address)
	if err != nil {
		client.Logger.Log(3, "Error connecting to: "+client.address+" | Error: "+err.Error())
		return nil
	}

	//Get host
	host, _ := strings.CutSuffix(client.address, "/websocket")
	host = strings.SplitN(host, ":", 2)[0]

	//Generate random key
	key := base64.StdEncoding.EncodeToString([]byte(GenerateRandomString(24)))

	//Make handshake GET
	request := "GET /websocket HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	_, err2 := conn.Write([]byte(request))
	if err2 != nil {
		conn.Close()
		client.Logger.Log(3, "Error writing to connection! | Error: "+err2.Error())
		return nil
	}

	//Get response
	buffer := make([]byte, 1024)
	_, err3 := conn.Read(buffer)
	if err3 != nil {
		conn.Close()
		client.Logger.Log(3, "Error reading response! | Error: "+err3.Error())
		return nil
	}
	response := string(buffer)

	//Check if switching protocols
	if !strings.Contains(response, "HTTP/1.1 101 Switching Protocols") {
		conn.Close()
		client.Logger.Log(3, "Error handshaking response!")
		return nil
	}

	//Check if handshake key is correct
	wsKey := computeWebSocketKey(key)
	if !strings.Contains(response, "Sec-Websocket-Accept: "+wsKey) {
		conn.Close()
		println(response)
		println(computeWebSocketKey(key))
		client.Logger.Log(3, "Invalid handshake key!")
		return nil
	}

	//Handle connection
	client.Logger.Log(2, "Connected to "+client.address+"!")
	client.connection = conn
	go HandleWebSocketRead(conn, &client.Logger, client.readFunc)
	return conn
}

/*
Writes to WebSocket server
*/
func (client *WebSocketClient) WriteToServer(message string) {
	frame := PackWebSocketFrame(message, 1)
	client.Logger.Log(1, "Sending data to server | Data: "+message)
	_, err := client.connection.Write(frame)
	if err != nil {
		client.Logger.Log(3, "Error sending data to server | Error: "+err.Error())
	}
}

func (client *WebSocketClient) Close() error {
	return client.connection.Close()
}

///*
//Pings server and waits for 10 seconds for responce
//*/
//func (client *WebSocketClient) Ping() bool {
//	//Send
//	client.gotPing = false
//	client.pingMessage = ""
//	frame := PackWebSocketFrame("PING", 9)
//	client.Logger.Log(1, "Sending ping to server.")
//	_, err := client.connection.Write(frame)
//	if err != nil {
//		client.Logger.Log(3, "Error sending ping to server | Error: "+err.Error())
//	}
//
//	for i := 0; i < 10; i++ {
//		//Await for ping
//		if client.gotPing {
//			break
//		}
//		time.Sleep(1 * time.Second)
//	}
//
//	return client.gotPing && client.pingMessage == "PING"
//}
//
//func (client *WebSocketClient) readPing(_ net.Conn, msg string, _ bool) {
//	client.gotPing = true
//	client.pingMessage = msg
//}
