package main

import (
	"fmt"
	"os"
	"time"
	"webtools"
)

func main() {
	fmt.Println("Hello world")
	switch os.Args[1] {
	case "ts":
		{
			server, _ := webtools.NewTCPServer("127.0.0.1:1234", readFuncTCPSv, true)
			server.Start()
			break
		}
	case "tc":
		{
			client, _ := webtools.NewTCPClient("127.0.0.1:1234", readFuncTCPSCl, true)
			client.Connect()
			client.Send([]byte("Test"))
			for client.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	}
}

func readFuncTCPSv(conn *webtools.TCPServerConn, data []byte, ended bool) {
	conn.Send(data)
}

func readFuncTCPSCl(conn *webtools.TCPClient, data []byte, ended bool) {
	//conn.Send(data)
	conn.Stop()
}
