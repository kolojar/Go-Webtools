/*
Package udp provides tools for handeling UDP traffic
*/
package udp

import (
	"math/rand/v2"
	"net"
	"time"

	"github.com/kolojar/Go-Webtools/helpertools"
)

/*
Bridge copies data from one port to other in UDP protocol
*/
type Bridge struct {
	udpSourceServerAdress     string
	udpServer                 *Server
	connetionUDPLocalToRemote helpertools.SafeMap[*Client, *ServerConn]
	connetionUDPRemoteToLocal helpertools.SafeMap[*ServerConn, *Client]
	reportTraffic             bool
}

/*
Read data Handler for local UDP (original server - source server)
*/
func (br *Bridge) readFuncUDPLocal(client *Client, _ *net.UDPAddr, data []byte, ended bool) {
	if br.connetionUDPLocalToRemote.Get(client) == nil {
		br.udpServer.Logger.Log(3, "Error writing to UDP Client - Connection does not exist!")
		return
	}
	if !ended {
		time.Sleep(time.Millisecond * time.Duration(rand.Int32N(50))) //Fake latency test
		br.connetionUDPLocalToRemote.Get(client).Send(data)
	} else {
		//conn := proxySv.connetionWebSocketToTCPTranslator[ws].connection
		//delete(proxySv.connetionTCPToWebSocketTranslator, conn)
		//delete(proxySv.connetionWebSocketToTCPTranslator, ws)
		//bridge.connetionUDP1To2[conn].Close()
		conn2 := br.connetionUDPLocalToRemote.Get(client)
		br.connetionUDPLocalToRemote.Delete(client)
		br.connetionUDPRemoteToLocal.Delete(conn2)
		conn2.Close()
		client.Stop()
	}
}

/*
Read data Handler for bridget UDP (new server - virtual target server)
*/
func (br *Bridge) readFuncUDPRemote(conn *ServerConn, data []byte, ended bool) {
	if br.connetionUDPRemoteToLocal.Get(conn) == nil {
		if ended {
			return
		}
		udpClient, err := NewClient(br.udpSourceServerAdress, br.readFuncUDPLocal, br.reportTraffic)
		if err != nil {
			br.udpServer.Logger.Log(3, "Error connecting to: "+br.udpSourceServerAdress+" | Error: "+err.Error())
		}
		udpClient.Logger.Prefix = "UDPBridge - " + udpClient.Logger.Prefix
		udpClient.Connect()
		br.connetionUDPRemoteToLocal.Set(conn, udpClient)
		br.connetionUDPLocalToRemote.Set(udpClient, conn)
	}
	if !ended {
		br.connetionUDPRemoteToLocal.Get(conn).Send(data)
	} else {
		conn2 := br.connetionUDPRemoteToLocal.Get(conn)
		br.connetionUDPRemoteToLocal.Delete(conn)
		br.connetionUDPLocalToRemote.Delete(conn2)
		conn2.Stop()
		conn.Close()
		//conn := proxySv.connetionWebSocketToTCPTranslator[ws].connection
		//delete(proxySv.connetionTCPToWebSocketTranslator, conn)
		//delete(proxySv.connetionWebSocketToTCPTranslator, ws)
	}
}

/*
NewBridge creates new instance of UDP Bridge but does not start it
*/
func NewBridge(udpSourceServerAdress string, udpNewVirtualAddress string, reportTraffic bool) (*Bridge, error) {
	udpBridge := &Bridge{udpSourceServerAdress: udpSourceServerAdress, connetionUDPLocalToRemote: helpertools.MakeSafeMap[*Client, *ServerConn](), connetionUDPRemoteToLocal: helpertools.MakeSafeMap[*ServerConn, *Client](), reportTraffic: reportTraffic}
	udpServer, err := NewServer(udpNewVirtualAddress, udpBridge.readFuncUDPRemote, reportTraffic)
	if err != nil {
		return nil, err
	}
	udpServer.Logger.Prefix = "UDPBridge - " + udpServer.Logger.Prefix
	udpBridge.udpServer = udpServer
	return udpBridge, nil
}

/*
Start starts bridge, locks execution thread
*/
func (br *Bridge) Start() {
	br.udpServer.Logger.Log(2, "Started bridging from "+br.udpSourceServerAdress+" to "+br.udpServer.GetAddress().String())
	br.udpServer.Start()
}

/*
Stop stops bridge
*/
func (br *Bridge) Stop() {
	br.udpServer.Stop()
	for _, v := range br.connetionUDPLocalToRemote.GetValues() {
		v.Close()
	}
}
