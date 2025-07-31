package main

import (
	"fmt"
	"os"
	"webtools"
)

var server *webtools.UDPServer

func main() {
	fmt.Println("Hello world")
	switch os.Args[1] {
	case "s":
		{
			server, _ := webtools.NewUDPServer("127.0.0.1:1234", readFuncUDP, true)
			server.Start()
			break
		}
	}
}

func readFuncUDP(conn *webtools.UDPServerConn, data []byte, ended bool) {
	conn.Send(data)
}
