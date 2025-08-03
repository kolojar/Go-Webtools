package main

import (
	"fmt"
	"os"
	"strconv"
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
			client, _ := webtools.NewTCPClient("127.0.0.1:9012", readFuncTCPCl, true)
			client.Connect()
			for i := 0; i < 10; i++ {
				client.Send([]byte("Test" + strconv.Itoa(i)))
				time.Sleep(1 * time.Second)
			}
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
			client, _ := webtools.NewUDPClient("127.0.0.1:5678", readFuncUDPCl, true)
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
	//case "hwts":
	//	{
	//		sv := webtools.NewHTTPWebTransportServer("127.0.0.1:5678", readFuncHTTPWTSv, true)
	//		sv.Start()
	//	}
	//case "hwtc":
	//	{
	//		cl, _ := webtools.NewHTTPWebTransportClient("127.0.0.1:5678", readFuncHTTPWTCl, true)
	//		cl.Connect()
	//		cl.Send([]byte("Test"))
	//		for cl.IsAlive() {
	//			time.Sleep(1 * time.Second)
	//		}
	//	}
	case "hpst":
		{
			sv := webtools.NewHTTPProxyServerTCP("127.0.0.1:5678", "127.0.0.1:7777", true)
			sv.Start()
		}
	case "hpct":
		{
			cl, _ := webtools.NewHTTPProxyClientTCP("127.0.0.1:5678", "127.0.0.1:17777", true)
			cl.Connect()
			for cl.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	case "tub":
		{
			br, _ := webtools.NewTCPToUDPBridge("127.0.0.1:9012", "127.0.0.1:17777", false)
			br.Start()
		}
	case "utb":
		{
			br, _ := webtools.NewUDPToTCPBridge("127.0.0.1:7777", "127.0.0.1:9012", false)
			br.Start()
		}
	case "hpst2":
		{
			sv := webtools.NewHTTPProxyServerTCP("127.0.0.1:9013", "127.0.0.1:9012", false)
			sv.Start()
		}
	case "hpct2":
		{
			cl, _ := webtools.NewHTTPProxyClientTCP("127.0.0.1:9013", "127.0.0.1:9014", false)
			cl.Connect()
			for cl.IsAlive() {
				time.Sleep(1 * time.Second)
			}
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
		//conn.Stop()
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

func readFuncHTTPWTSv(conn *webtools.HTTPWebTransportServerConn, data []byte, ended bool) {
	//conn.Send(data)
	if !ended {
		conn.Send(data)
	}
}

func readFuncHTTPWTCl(conn *webtools.HTTPWebTransportClient, data []byte, ended bool) {
	//conn.Send(data)
	if !ended {
		conn.Stop()
	}
}
