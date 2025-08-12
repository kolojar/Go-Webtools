package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"webtools"
)

func main() {
	fmt.Println("Hello world")
	switch os.Args[1] {
	case "ts":
		{
			server, _ := webtools.NewTCPServer("127.0.0.1:7777", readFuncTCPSv, true, false)
			server.Start()
			break
		}
	case "tc":
		{
			client, _ := webtools.NewTCPClient("127.0.0.1:17777", readFuncTCPCl, true, false)
			client.Connect()
			for i := 0; i < 1000; i++ {
				client.Send([]byte("Test" + strconv.Itoa(i) + "|"))
			}
			time.Sleep(3 * time.Second)
			fmt.Println(rc)
			//for client.IsAlive() {
			//}
		}
	case "us":
		{
			server, _ := webtools.NewUDPServer("127.0.0.1:7777", readFuncUDPSv, true)
			server.Start()
			break
		}
	case "uc":
		{
			client, _ := webtools.NewUDPClient("127.0.0.1:17777", readFuncUDPCl, true)
			client.Connect()
			for i := 0; i < 500; i++ {
				client.Send([]byte("Test"))
			}
			time.Sleep(3 * time.Second)
			client.Stop()
			fmt.Println(rc)
			for client.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	case "hs":
		{
			sv := webtools.NewHTTPServer("127.0.0.1:7777", nil, "", false)
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
			sv := webtools.NewHTTPProxyServerTCP("127.0.0.1:8880", "127.0.0.1:7777", true)
			sv.Start()
		}
	case "hpct":
		{
			cl, _ := webtools.NewHTTPProxyClientTCP("127.0.0.1:8881", "127.0.0.1:17777", true)
			cl.Connect()
			for cl.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	case "br":
		{
			br := webtools.NewUDPBridge("127.0.0.1:7777", "127.0.0.1:17777")
			br.Start()
		}
	//case "hpsu":
	//	{
	//		sv := webtools.NewHTTPProxyServerUDP("127.0.0.1:5679", "127.0.0.1:7777", true)
	//		sv.Start()
	//	}
	//case "hpcu":
	//	{
	//		cl, err := webtools.NewHTTPProxyClientUDP("127.0.0.1:5679", "127.0.0.1:17777", true)
	//		if err != nil {
	//			fmt.Println(err)
	//			return
	//		}
	//		cl.Connect()
	//		for cl.IsAlive() {
	//			time.Sleep(1 * time.Second)
	//		}
	//	}
	case "tpsu":
		{
			sv, _ := webtools.NewTCPProxyServerUDP("127.0.0.1:5679", "127.0.0.1:7777", false)
			sv.Start()
		}
	case "tpcu":
		{
			cl, err := webtools.NewTCPProxyClientUDP("127.0.0.1:5681", "127.0.0.1:17777", false)
			if err != nil {
				fmt.Println(err)
				return
			}
			cl.Connect()
			for cl.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	case "tcms":
		{
			sv, _ := webtools.NewTCPConnectionMergerServer("127.0.0.1:8880", []string{"127.0.0.1:5679", "127.0.0.1:7777", "127.0.0.1:8888"}, true)
			sv.Start()
		}
	case "tcmc":
		{
			cl, _ := webtools.NewTCPConnectionMergerClient("127.0.0.1:8880", "127.0.0.1", map[string]string{"127.0.0.1:5679": "5681", "127.0.0.1:7777": "17777", "127.0.0.1:8888": "8888"}, true)
			cl.Connect()
			for cl.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	case "tub":
		{
			br, _ := webtools.NewTCPToUDPBridge("127.0.0.1:9012", "127.0.0.1:17777", true)
			br.Start()
		}
	case "utb":
		{
			br, _ := webtools.NewUDPToTCPBridge("127.0.0.1:7777", "127.0.0.1:9012", true)
			br.Start()
		}
	case "hpst2":
		{
			sv := webtools.NewHTTPProxyServerTCP("127.0.0.1:9013", "127.0.0.1:9012", true)
			sv.Start()
		}
	case "hpct2":
		{
			cl, _ := webtools.NewHTTPProxyClientTCP("127.0.0.1:9013", "127.0.0.1:9014", true)
			cl.Connect()
			for cl.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	}
}

var rc int = 0

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
	fmt.Println(string(data))
	rc += len(strings.Split(string(data), "|")) - 1
}

func readFuncUDPSv(conn *webtools.UDPServerConn, data []byte, ended bool) {
	if !ended {
		conn.Send(data)
	}
}

func readFuncUDPCl(conn *webtools.UDPClient, data []byte, ended bool) {
	//conn.Send(data)
	//if !ended {
	//	conn.Stop()
	//}
	fmt.Println(string(data))
	rc++
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
