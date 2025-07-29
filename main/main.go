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
			proxyServer = webtools.MakeHTTPProxyServerUDP("127.0.0.1:1234", "127.0.0.1:5678", "", "", nil, nil, false)
			proxyServer.Start()
		}
	case "pc":
		{
			proxyClient := webtools.MakeHTTPProxyClientUDP("127.0.0.1:9012", "127.0.0.1:5678")
			proxyClient.Start()
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
