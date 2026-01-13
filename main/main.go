/*
Package main provides example usages
*/
package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	"webtools"
	"webtools/httptools"
	"webtools/p2p"
	"webtools/proxy"
	"webtools/tcp"
	"webtools/udp"
)

func main() {
	fmt.Println("Hello world")
	framer := udp.NewUDPFramerSimple(nil, 50, 5, true, 50, true)
	ip, _ := p2p.GetThisComputerLocalIP()
	upnp := p2p.NewUPnPServiceManager(ip)
	switch os.Args[1] {
	case "ts":
		{
			server, _ := tcp.NewServer("127.0.0.1:7777", readFuncTCPSv, true, true)
			server.SetupEncryption(true, []byte("1234"))
			server.Start()
			break
		}
	case "ts2":
		{
			server, _ := tcp.NewServer("127.0.0.1:8888", readFuncTCPSv, true, true)
			server.SetupEncryption(true, []byte("1234"))
			server.Start()
			break
		}
	case "tc":
		{
			client, _ := tcp.NewClientSimple("127.0.0.1:17777", 0, false, readFuncTCPCl, true)
			client.SetupEncryption(true, []byte("1234"))
			client.Connect()
			//for i := 0; i < 100; i++ {
			//	client.Send([]byte("Test" + strconv.Itoa(i) + "|"))
			//}
			for {
				client.Send([]byte("7777-" + time.Now().Format(time.StampMilli)))
				time.Sleep(time.Millisecond)
			}
			time.Sleep(3 * time.Second)
			fmt.Println(rc)
			//for client.IsAlive() {
			//}
		}
	case "tc2":
		{
			client, _ := tcp.NewClientSimple("127.0.0.1:18888", 0, false, readFuncTCPCl, true)
			client.SetupEncryption(true, []byte("1234"))
			client.Connect()
			//for i := 0; i < 100; i++ {
			//	client.Send([]byte("Test" + strconv.Itoa(i) + "|"))
			//}
			for {
				client.Send([]byte("8888-" + time.Now().Format(time.StampMilli)))
				time.Sleep(time.Millisecond)
			}
			time.Sleep(3 * time.Second)
			fmt.Println(rc)
			//for client.IsAlive() {
			//}
		}
	case "us":
		{
			server, _ := udp.NewServer("127.0.0.1:7777", readFuncUDPSv, true)
			server.SetupFraming(framer)
			server.Start()
			break
		}
	case "uc":
		{
			client, _ := udp.NewClient("127.0.0.1:17777", readFuncUDPCl, true)
			client.SetupFraming(framer)
			client.Connect()
			for i := 0; i < 10; i++ {
				client.Send([]byte("Test" + strconv.Itoa(i) + "|"))
				time.Sleep(time.Millisecond * 5)
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
			sv := httptools.NewServer("0.0.0.0:7777", nil, "../encryption/", false)
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
			sv := proxy.NewHTTPProxyServerTCP("127.0.0.1:8880", "192.168.0.229:7777", true)
			sv.Start()
		}
	case "hpct":
		{
			cl, err := proxy.NewHTTPProxyClientTCP("127.0.0.1:8880/websocket", "127.0.0.1:7777", true)
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
			sv, _ := proxy.NewTCPProxyServerUDP("127.0.0.1:5679", "192.168.0.229:7777", false)
			sv.Start()
		}
	case "tpcu":
		{
			cl, err := proxy.NewTCPProxyClientUDP("127.0.0.1:5679", "127.0.0.1:7777", false)
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
			sv, _ := tcp.NewConnectionMergerServer("127.0.0.1:8882", []string{"127.0.0.1:5679", "192.168.0.229:7777", "192.168.0.229:8888"}, true)
			sv.Start()
		}
	case "tcmc":
		{
			cl, _ := tcp.NewConnectionMergerClient("127.0.0.1:8882", "127.0.0.1", map[string]string{"127.0.0.1:5679": "5681", "192.168.0.229:7777": "7777", "192.168.0.229:8888": "8888"}, true)
			cl.Connect()
			for cl.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	case "hpst2":
		{
			sv := proxy.NewHTTPProxyServerTCP("127.0.0.1:9013", "192.168.0.229:8888", true)
			sv.Start()
		}
	case "hpct2":
		{
			cl, _ := proxy.NewHTTPProxyClientTCP("127.0.0.1:9013/websocket", "127.0.0.1:8888", true)
			cl.Connect()
			for cl.IsAlive() {
				time.Sleep(1 * time.Second)
			}
		}
	case "wss":
		{
			sv := httptools.NewWebSocketServer("127.0.0.1:1234", readFuncHTTPWsSv, nil, "", true)
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
			ub, _ := udp.NewBridge("127.0.0.1:7777", "127.0.0.1:17777", true)
			ub.Start()
		}
	case "p2psv":
		{
			sv, _ := p2p.NewCoordinator("0.0.0.0:1234", true, true)
			sv.Start()
			break
		}
	case "p2pcl":
		{
			cl, _ := p2p.NewP2PClient("127.0.0.1:1234", 5678, p2pReadFunc, true)
			cl.SetupUPnP(upnp)
			cl.ConnectToCoordinator()
			webtools.ReadLineFromConsole("Wait")
		}
	case "p2pcl2":
		{
			println("p2pCl2")
			cl, _ := p2p.NewP2PClient("127.0.0.1:1234", 5679, p2pReadFunc2, true)
			cl.SetupUPnP(upnp)
			cl.ConnectToCoordinator()
			data, _ := webtools.ReadLineFromConsole("Enter target id: ")
			cl.ConnectToPeer([]byte(strings.ReplaceAll(string(data), "\n", "")))
			cl.Send([]byte(strings.ReplaceAll(string(data), "\n", "")), []byte("Hello"))
			webtools.ReadLineFromConsole("wait")
		}
	case "p2pcl3":
		{
			cl, _ := p2p.NewP2PClient("127.0.0.1:1234", 5677, p2pReadFunc2, true)
			cl.SetupUPnP(upnp)
			cl.ConnectToCoordinator()
			data, _ := webtools.ReadLineFromConsole("Enter target id: ")
			cl.ConnectToPeer([]byte(strings.ReplaceAll(string(data), "\n", "")))
			cl.Send([]byte(strings.ReplaceAll(string(data), "\n", "")), []byte("Hello"))
			webtools.ReadLineFromConsole("wait")
		}
	case "upnp":
		{
			localIP, _ := p2p.GetThisComputerLocalIP()
			fmt.Println(localIP)
			//time.Sleep(5 * time.Second)
			upnp := p2p.NewUPnPServiceManager(localIP)
			println(upnp.SetupUPnP().Error())
			upnp.AddUPnPPort(5555, 5555, "TCP", "This it test")
			time.Sleep(10 * time.Second)
			//upnp.RemoveUPnPPort(5555, "TCP")
			upnp.Shutdown()
		}

	case "upnpCleanup":
		{
			upnp.GetRouterPublicIP()
			upnp.RemoveUPnPPort(5677, "UDP")
			upnp.RemoveUPnPPort(5678, "UDP")
			upnp.RemoveUPnPPort(5679, "UDP")
			upnp.RemoveUPnPPort(5677, "TCP")
			upnp.RemoveUPnPPort(5678, "TCP")
			upnp.RemoveUPnPPort(5679, "TCP")
			//upnp.RemoveUPnPPort(5555, "TCP")
			upnp.Shutdown()
		}
	case "p2ppsu":
		{
			proxy, _ := proxy.NewP2PProxyServerUDP("127.0.0.1:1234", 5678, "192.168.0.229:7777", true)
			proxy.Start()
		}
	case "p2ppcu":
		{
			data, _ := webtools.ReadLineFromConsole("Enter target id: ")
			proxy, _ := proxy.NewP2PProxyClientUDP("127.0.0.1:1234", 5679, []byte(strings.ReplaceAll(string(data), "\n", "")), "127.0.0.1:7777", true)
			proxy.Connect()
			for {
				time.Sleep(100 * time.Millisecond)
			}
		}
	case "p2ppst":
		{
			proxy, _ := proxy.NewP2PProxyServerTCP("127.0.0.1:1234", 5680, "192.168.0.229:7777", true)
			proxy.Start()
		}
	case "p2ppct":
		{
			data, _ := webtools.ReadLineFromConsole("Enter target id: ")
			proxy, _ := proxy.NewP2PProxyClientTCP("127.0.0.1:1234", 5681, []byte(strings.ReplaceAll(string(data), "\n", "")), "127.0.0.1:7777", true)
			proxy.Connect()
			for {
				time.Sleep(100 * time.Millisecond)
			}
		}
	case "p2ppst2":
		{
			proxy, _ := proxy.NewP2PProxyServerTCP("127.0.0.1:1234", 5682, "192.168.0.229:8888", true)
			proxy.Start()
		}
	case "p2ppct2":
		{
			data, _ := webtools.ReadLineFromConsole("Enter target id: ")
			proxy, _ := proxy.NewP2PProxyClientTCP("127.0.0.1:1234", 5683, []byte(strings.ReplaceAll(string(data), "\n", "")), "127.0.0.1:8888", true)
			proxy.Connect()
			for {
				time.Sleep(100 * time.Millisecond)
			}
		}
	case "checkcgnat":
		{
			p2p, _ := p2p.NewP2PClient("127.0.0.1:1234", 5678, nil, true)
			p2p.SetupUPnP(upnp)
			if p2p.ConnectToCoordinator() {
				p2p.CheckCGNAT()
			}
			p2p.Stop()
			upnp.Shutdown()
		}
	case "wsis":
		{
			sv := httptools.NewWebSocketInstanceServer("127.0.0.1:1234", readFuncHTTPWsInstanceSv, nil, "", true)
			sv.GetWSServer().GetHTTPServer().HostPaths["/test"] = "./test"
			sv.Start()
		}
	case "p2pps":
		{
			proxy, _ := proxy.NewP2PProxyServerUniversal("127.0.0.1:1234", 5678, true)
			proxy.ProxiedServices["u7777"] = webtools.ThreeValuePair[bool, bool, string]{A: true, B: false, C: "192.168.0.229:7777"}
			//proxy.ProxiedServices["t7777"] = webtools.ThreeValuePair[bool, bool, string]{A: false, B: true, C: "192.168.0.229:7777"}
			//proxy.ProxiedServices["t8888"] = webtools.ThreeValuePair[bool, bool, string]{A: false, B: true, C: "192.168.0.229:8888"}
			proxy.SetupFramingP2PClient(framer)
			proxy.Start()
		}
	case "p2ppc":
		{
			data, _ := webtools.ReadLineFromConsole("Enter target id: ")
			proxy, _ := proxy.NewP2PProxyClientUniversal("127.0.0.1:1234", 5679, []byte(strings.ReplaceAll(string(data), "\n", "")),
				map[string]webtools.KeyValuePair[bool, string]{
					"u7777": {Key: false, Value: "127.0.0.1:7777"},
					"t7777": {Key: true, Value: "127.0.0.1:17777"},
					"t8888": {Key: true, Value: "127.0.0.1:18888"},
				}, true)
			proxy.SetupFramingP2PClient(framer)
			proxy.Connect()
			for {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

func p2pReadFunc(client *p2p.Client, sourceID []byte, data []byte, _ bool, _ *webtools.ConsoleLogger) {
	client.Send(sourceID, data)
}

func p2pReadFunc2(_ *p2p.Client, _ []byte, data []byte, _ bool, _ *webtools.ConsoleLogger) {
	fmt.Println(string(data))
}

var rc = 0

func readFuncTCPSv(conn *tcp.ServerConn, data []byte, status uint8) {
	if status == webtools.ReadDataStatus {
		conn.Send(data)
	}
}

func readFuncTCPCl(_ *tcp.ClientSimple, data []byte, _ uint8) {
	//conn.Send(data)
	//if !ended {
	//	//conn.Stop()
	//}
	fmt.Println(string(data))
	rc += len(strings.Split(string(data), "|")) - 1
}

func readFuncUDPSv(conn *udp.ServerConn, data []byte, ended bool) {
	if !ended {
		conn.Send(data)
	}
	fmt.Println(string(data))
}

func readFuncUDPCl(_ *udp.Client, _ *net.UDPAddr, data []byte, _ bool) {
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

func readFuncHTTPWsSv(conn *httptools.WebSocketServerConn, data []byte, status uint8, _ bool) {
	if status > 1 {
		conn.Send(data)
	}
}

func readFuncHTTPWsCl(conn *httptools.WebSocketClient, _ []byte, status uint8, _ bool) {
	if status == webtools.ReadDataStatus {
		conn.Stop()
	}
}

func readFuncHTTPWsInstanceSv(inst *httptools.WebSocketInstanceServerInstance, conn *httptools.WebSocketServerConn, data []byte, status uint8, _ bool) {
	if status > 1 {
		conn.Send(append([]byte(inst.GetID()+" "), data...))
	}
}
