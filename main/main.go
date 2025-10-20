package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	"webtools"
	httptools "webtools/httpTools"
	"webtools/p2pTools"
	proxytools "webtools/proxyTools"
	tcptools "webtools/tcpTools"
	"webtools/udpTools"
	udptools "webtools/udpTools"
)

func main() {
	fmt.Println("Hello world")
	framer := udpTools.NewUDPFramerSimple(50, 5, true, 50)
	switch os.Args[1] {
	case "ts":
		{
			server, _ := tcptools.NewTCPServer("127.0.0.1:7777", readFuncTCPSv, true, true)
			server.SetupEncryption(true, []byte("1234"))
			server.Start()
			break
		}
	case "tc":
		{
			client, _ := tcptools.NewTCPClientSimple("127.0.0.1:7777", 0, false, readFuncTCPCl, true)
			client.SetupEncryption(true, []byte("1234"))
			client.Connect()
			for i := 0; i < 100; i++ {
				client.Send([]byte("Test" + strconv.Itoa(i) + "|"))
			}
			time.Sleep(3 * time.Second)
			fmt.Println(rc)
			//for client.IsAlive() {
			//}
		}
	case "us":
		{
			server, _ := udptools.NewUDPServer("127.0.0.1:7777", readFuncUDPSv, true)
			server.SetupFraming(framer)
			server.Start()
			break
		}
	case "uc":
		{
			client, _ := udptools.NewUDPClient("127.0.0.1:7777", readFuncUDPCl, true)
			client.SetupFraming(framer)
			client.Connect()
			for i := 0; i < 10; i++ {
				client.Send([]byte("Test" + strconv.Itoa(i) + "|"))
				time.Sleep(time.Millisecond)
			}
			time.Sleep(30 * time.Second)
			client.Stop()
			fmt.Println(rc)
			for client.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	case "hs":
		{
			sv := httptools.NewHTTPServer("0.0.0.0:7777", nil, "../encryption/", false)
			sv.HostPaths["/test"] = "../test"
			sv.UseDirectoryListing = true
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
			sv := proxytools.NewHTTPProxyServerTCP("127.0.0.1:8880", "127.0.0.1:7777", true)
			sv.Start()
		}
	case "hpct":
		{
			cl, err := proxytools.NewHTTPProxyClientTCP("127.0.0.1:8880/websocket", "127.0.0.1:17777", true)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			cl.Connect()
			for cl.IsAlive() {
				time.Sleep(1 * time.Second)
			}
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
			sv, _ := proxytools.NewTCPProxyServerUDP("127.0.0.1:5679", "127.0.0.1:7777", false)
			sv.Start()
		}
	case "tpcu":
		{
			cl, err := proxytools.NewTCPProxyClientUDP("127.0.0.1:5681", "127.0.0.1:17777", false)
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
			sv, _ := tcptools.NewTCPConnectionMergerServer("127.0.0.1:8882", []string{"127.0.0.1:5679", "127.0.0.1:7777", "127.0.0.1:8888"}, true)
			sv.Start()
		}
	case "tcmc":
		{
			cl, _ := tcptools.NewTCPConnectionMergerClient("127.0.0.1:8882", "127.0.0.1", map[string]string{"127.0.0.1:5679": "5681", "127.0.0.1:7777": "17777", "127.0.0.1:8888": "8888"}, true)
			cl.Connect()
			for cl.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	case "hpst2":
		{
			sv := proxytools.NewHTTPProxyServerTCP("127.0.0.1:9013", "127.0.0.1:9012", true)
			sv.Start()
		}
	case "hpct2":
		{
			cl, _ := proxytools.NewHTTPProxyClientTCP("127.0.0.1:9013", "127.0.0.1:9014", true)
			cl.Connect()
			for cl.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	case "wss":
		{
			sv := httptools.NewHTTPWebSocketServer("127.0.0.1:1234", readFuncHTTPWsSv, nil, "", true)
			sv.GetHTTPServer().HostPaths["/test"] = "./test"
			sv.Start()
		}
	case "wsc":
		{
			cl, err := httptools.NewWebSocketClient("127.0.0.1:1234/websocket", readFuncHTTPWsCl, true)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			cl.Connect()
			cl.Send([]byte("hello"), 1)
			cl.Send([]byte("hi"), 1)
			for cl.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	case "ub":
		{
			ub, _ := udpTools.NewUDPBridge("127.0.0.1:7777", "127.0.0.1:17777", true)
			ub.Start()
		}
	case "p2psv":
		{
			sv, _ := p2pTools.NewP2PCoordinator("127.0.0.1:1234", true, true)
			sv.Start()
		}
	case "p2pcl":
		{
			cl, _ := p2pTools.NewP2PClientUDP("127.0.0.1:1234", 5678, p2pReadFunc, true)
			cl.ConnectToCoordinator()
			webtools.ReadLineFromConsole("Wait")
		}
	case "p2pcl2":
		{
			cl, _ := p2pTools.NewP2PClientUDP("127.0.0.1:1234", 5679, nil, true)
			cl.ConnectToCoordinator()
			data, _ := webtools.ReadLineFromConsole("Enter target id: ")
			cl.ConnectToPeer(strings.ReplaceAll(string(data), "\n", ""))
			cl.Send([]byte(strings.ReplaceAll(string(data), "\n", "")), []byte("Hello"))
			webtools.ReadLineFromConsole("wait")
		}
	case "p2pcl3":
		{
			cl, _ := p2pTools.NewP2PClientUDP("127.0.0.1:1234", 5677, nil, true)
			cl.ConnectToCoordinator()
			data, _ := webtools.ReadLineFromConsole("Enter target id: ")
			cl.ConnectToPeer(strings.ReplaceAll(string(data), "\n", ""))
			cl.Send([]byte(strings.ReplaceAll(string(data), "\n", "")), []byte("Hello"))
			webtools.ReadLineFromConsole("wait")
		}
	}
}

func p2pReadFunc(client *p2pTools.P2PClientUDP, sourceId []byte, data []byte, ended bool) {
	client.Send(sourceId, data)
}

var rc int = 0

func readFuncTCPSv(conn *tcptools.TCPServerConn, data []byte, status uint8) {
	if status == webtools.TCP_READ_DATA_STATUS {
		conn.Send(data)
	}
}

func readFuncTCPCl(conn *tcptools.TCPClientSimple, data []byte, status uint8) {
	//conn.Send(data)
	//if !ended {
	//	//conn.Stop()
	//}
	fmt.Println(string(data))
	rc += len(strings.Split(string(data), "|")) - 1
}

func readFuncUDPSv(conn *udptools.UDPServerConn, data []byte, ended bool) {
	if !ended {
		conn.Send(data)
	}
}

func readFuncUDPCl(conn *udptools.UDPClient, sourceAddress *net.UDPAddr, data []byte, ended bool) {
	//conn.Send(data)
	//if !ended {
	//	conn.Stop()
	//}
	fmt.Println(string(data))
	rc++
}

//func readFuncHTTPWTSv(conn *webtools.HTTPWebTransportServerConn, data []byte, ended bool) {
//	//conn.Send(data)
//	if !ended {
//		conn.Send(data)
//	}
//}
//
//func readFuncHTTPWTCl(conn *webtools.HTTPWebTransportClient, data []byte, ended bool) {
//	//conn.Send(data)
//	if !ended {
//		conn.Stop()
//	}
//}

func readFuncHTTPWsSv(conn *httptools.WebSocketServerConn, data []byte, status uint8, isBinary bool) {
	if status > 1 {
		conn.Send(data)
	}
}

func readFuncHTTPWsCl(conn *httptools.WebSocketClient, data []byte, status uint8, isBinary bool) {
	if status == webtools.TCP_READ_DATA_STATUS {
		conn.Stop()
	}
}
