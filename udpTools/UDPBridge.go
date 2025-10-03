package udpTools

import (
	"webtools"
)

/*
UDPBridge copies data from one port to other in UDP protocol
*/
type UDPBridge struct {
	tcpSourceServerAdress     string
	udpServer                 *UDPServer
	connetionUDPLocalToRemote webtools.SafeMap[*UDPClient, *UDPServerConn]
	connetionUDPRemoteToLocal webtools.SafeMap[*UDPServerConn, *UDPClient]
	reportTraffic             bool
}

/*
Read data Handler for local UDP (original server - source server)
*/
func (bridge *UDPBridge) readFuncUDPLocal(client *UDPClient, data []byte, ended bool) {
	if bridge.connetionUDPLocalToRemote.Get(client) == nil {
		bridge.udpServer.Logger.Log(3, "Error writing to UDP Client - Connection does not exist!")
		return
	}
	if !ended {
		bridge.connetionUDPLocalToRemote.Get(client).Send(data)
	} else {
		//conn := proxySv.connetionWebSocketToTCPTranslator[ws].connection
		//delete(proxySv.connetionTCPToWebSocketTranslator, conn)
		//delete(proxySv.connetionWebSocketToTCPTranslator, ws)
		//bridge.connetionUDP1To2[conn].Close()
		conn2 := bridge.connetionUDPLocalToRemote.Get(client)
		bridge.connetionUDPLocalToRemote.Delete(client)
		bridge.connetionUDPRemoteToLocal.Delete(conn2)
		conn2.Close()
		client.Stop()
	}
}

/*
Read data Handler for bridget UDP (new server - virtual target server)
*/
func (bridge *UDPBridge) readFuncUDPRemote(conn *UDPServerConn, data []byte, ended bool) {
	if bridge.connetionUDPRemoteToLocal.Get(conn) == nil {
		udpClient, err := NewUDPClient(bridge.tcpSourceServerAdress, bridge.readFuncUDPLocal, bridge.reportTraffic)
		if err != nil {
			bridge.udpServer.Logger.Log(3, "Error connecting to: "+bridge.tcpSourceServerAdress+" | Error: "+err.Error())
		}
		udpClient.Logger.Prefix = "UDPBridge - " + udpClient.Logger.Prefix
		udpClient.Connect()
		bridge.connetionUDPRemoteToLocal.Set(conn, udpClient)
		bridge.connetionUDPLocalToRemote.Set(udpClient, conn)
	}
	if !ended {
		bridge.connetionUDPRemoteToLocal.Get(conn).Send(data)
	} else {
		conn2 := bridge.connetionUDPRemoteToLocal.Get(conn)
		bridge.connetionUDPRemoteToLocal.Delete(conn)
		bridge.connetionUDPLocalToRemote.Delete(conn2)
		conn2.Stop()
		conn.Close()
		//conn := proxySv.connetionWebSocketToTCPTranslator[ws].connection
		//delete(proxySv.connetionTCPToWebSocketTranslator, conn)
		//delete(proxySv.connetionWebSocketToTCPTranslator, ws)
	}
}

/*
Constructs new instance of HTTPProxy Server for TCP but does not start it
*/
func NewUDPBridge(tcpSourceServerAdress string, tcpClientAddress string, reportTraffic bool) (*UDPBridge, error) {
	udpBridge := &UDPBridge{tcpSourceServerAdress: tcpSourceServerAdress, connetionUDPLocalToRemote: webtools.MakeSafeMap[*UDPClient, *UDPServerConn](), connetionUDPRemoteToLocal: webtools.MakeSafeMap[*UDPServerConn, *UDPClient](), reportTraffic: reportTraffic}
	udpServer, err := NewUDPServer(tcpClientAddress, udpBridge.readFuncUDPRemote, reportTraffic)
	if err != nil {
		return nil, err
	}
	udpServer.Logger.Prefix = "UDPBridge - " + udpServer.Logger.Prefix
	udpBridge.udpServer = udpServer
	return udpBridge, nil
}

/*
Starts bridge, locks execution thread
*/
func (bridge *UDPBridge) Start() {
	bridge.udpServer.Logger.Log(2, "Started bridging from "+bridge.tcpSourceServerAdress+" to "+bridge.udpServer.GetAddress().String())
	bridge.udpServer.Start()
}

/*
Stops bridge
*/
func (bridge *UDPBridge) Stop() {
	bridge.udpServer.Stop()
	for _, v := range bridge.connetionUDPLocalToRemote.GetValues() {
		v.Close()
	}
}
