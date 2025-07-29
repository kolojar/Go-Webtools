package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
	"webtools"
)

var server webtools.UDPServer
var proxyServer webtools.HTTPProxyServerUDP

func main() {
	fmt.Println("Hello world")
	switch os.Args[1] {
	case "s":
		{
			server = webtools.MakeUDPServer("127.0.0.1:1234", readFuncUDP, false, "")
			server.Start()
			break
		}
	case "ps":
		{
			proxyServer = webtools.MakeHTTPProxyServerUDP("127.0.0.1:7777", "127.0.0.1:5678", "", "", nil, nil, false)
			proxyServer.Start()
		}
	case "pcu7777":
		{
			proxyClient := webtools.MakeHTTPProxyClientUDP("127.0.0.1:17777", "127.0.0.1:5678")
			proxyClient.Start()
		}
	case "pct7777":
		{
			proxyClient := webtools.MakeHTTPProxyClientTCP("127.0.0.1:7777", "192.168.0.229:27777")
			proxyClient.Start()
		}
	case "pct8888":
		{
			proxyClient := webtools.MakeHTTPProxyClientTCP("127.0.0.1:8888", "192.168.0.229:18888")
			proxyClient.Start()
		}
	case "bt7777":
		{
			bridge := webtools.MakeTCPBridge("127.0.0.1:7777", "127.0.0.1:17777")
			bridge.Start()
		}
	case "bu7777":
		{
			bridge := webtools.MakeUDPBridge("127.0.0.1:7777", "127.0.0.1:17777")
			bridge.Start()
		}
	case "c":
		{
			client := webtools.MakeUDPClient("127.0.0.1:9012", nil, false, "")
			client.Connect()
			for i := 0; i < 10; i++ {
				client.WriteToServer("Hello " + strconv.Itoa(i))
				time.Sleep(1 * time.Second)
			}
			break
		}
	}
}

func readFuncUDP(addr *net.UDPAddr, data string, ended bool) {
	server.WriteToClient(addr, data)
}
