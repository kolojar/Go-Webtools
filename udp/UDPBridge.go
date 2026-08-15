/*
Package udp provides tools for handeling UDP traffic
*/
package udp

import (
	"math/rand/v2"
	"net"
	"time"

	webtools "github.com/kolojar/Go-Webtools"
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
func (br *Bridge) readFuncUDPLocal(client *Client, _ *net.UDPAddr, data []byte, status webtools.NetworkStatus) {
	remoteConn := br.connetionUDPLocalToRemote.Get(client)
	if remoteConn == nil {
		br.udpServer.Logger.Log(helpertools.LogError, "Error writing to UDP Client - Connection does not exist!")
		return
	}
	switch status {
	case webtools.ReadDataStatus:
		{
			if rand.Int32N(50) < 40 {
				time.AfterFunc(time.Millisecond*time.Duration(rand.Int32N(50)), func() {
					remoteConn.Send(data)
				}) //Fake latency test
			} //Fake loss test
		}
	case webtools.DisconnectStatus:
		{
			//conn := proxySv.connetionWebSocketToTCPTranslator[ws].connection
			//delete(proxySv.connetionTCPToWebSocketTranslator, conn)
			//delete(proxySv.connetionWebSocketToTCPTranslator, ws)
			//bridge.connetionUDP1To2[conn].Close()
			br.connetionUDPLocalToRemote.Delete(client)
			br.connetionUDPRemoteToLocal.Delete(remoteConn)
			remoteConn.Close()
			client.Stop()
		}
	}
}

/*
Read data Handler for bridget UDP (new server - virtual target server)
*/
func (br *Bridge) readFuncUDPRemote(conn *ServerConn, data []byte, status webtools.NetworkStatus) {
	switch status {
	case webtools.ConnectStatus:
		{
			udpClient, err := NewClient(br.udpSourceServerAdress, br.readFuncUDPLocal, br.reportTraffic)
			if err != nil {
				br.udpServer.Logger.Log(helpertools.LogError, "Error connecting to: "+br.udpSourceServerAdress+" | Error: "+err.Error())
			}
			udpClient.Logger.Prefix = "UDPBridge - " + udpClient.Logger.Prefix
			udpClient.Connect()
			br.connetionUDPRemoteToLocal.Set(conn, udpClient)
			br.connetionUDPLocalToRemote.Set(udpClient, conn)
		}
	case webtools.ReadDataStatus:
		{
			br.connetionUDPRemoteToLocal.Get(conn).Send(data)
		}
	case webtools.DisconnectStatus:
		{
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
	br.udpServer.Logger.Log(helpertools.LogWarning, "Started bridging from "+br.udpSourceServerAdress+" to "+br.udpServer.GetAddress().String())
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
