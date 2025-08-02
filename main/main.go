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
			client, _ := webtools.NewTCPClient("127.0.0.1:1234", readFuncTCPCl, true)
			client.Connect()
			client.Send([]byte("Test"))
			for client.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	case "us":
		{
			server, _ := webtools.NewUDPServer("127.0.0.1:1234", readFuncUDPSv, true)
			server.Start()
			break
		}
	case "uc":
		{
			client, _ := webtools.NewUDPClient("127.0.0.1:1234", readFuncUDPCl, true)
			client.Connect()
			client.Send([]byte("Test"))
			for client.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	case "hs":
		{
			sv := webtools.NewHTTPServer("127.0.0.1:8080", nil, "", false)
			sv.HostPaths["/test"] = "./test"
			sv.Start()
		}
	}
}

func readFuncTCPSv(conn *webtools.TCPServerConn, data []byte, ended bool) {
	if !ended {
		conn.Send(data)
	}
}

func readFuncTCPCl(conn *webtools.TCPClient, data []byte, ended bool) {
	//conn.Send(data)
	if !ended {
		conn.Stop()
	}
}

func readFuncUDPSv(conn *webtools.UDPServerConn, data []byte, ended bool) {
	if !ended {
		conn.Send(data)
	}
}

func readFuncUDPCl(conn *webtools.UDPClient, data []byte, ended bool) {
	//conn.Send(data)
	if !ended {
		conn.Stop()
	}
}
