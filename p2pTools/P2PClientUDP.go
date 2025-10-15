package p2pTools

import (
	"net"
	"webtools/httpTools"
	"webtools/udpTools"
)

type P2PClientUDP struct {
	udpClient   *udpTools.UDPClient
	id          string
	isReady     bool
	pendingData [][]byte
}

/*
Creates new P2P UDP client
*/
func NewP2PClientUDP(address string, reportTraffic bool) (*P2PClientUDP, error) {
	p2p := &P2PClientUDP{}
	var err error
	p2p.udpClient, err = udpTools.NewUDPClient(address, p2p.readFuncLocal, reportTraffic)
	if err != nil {
		return nil, err
	}
	return p2p, nil
}

func (p2p *P2PClientUDP) readFuncLocal(client *udpTools.UDPClient, sourceAddress *net.UDPAddr, data []byte, ended bool) {
	//Commands
	command, args := httpTools.CreateParametersFromURL(string(data))
	switch command {
	case P2P_CMD_NEW_ID:
		{
			//Sets new id to client
			if args["id"] == "" {
				p2p.udpClient.Logger.Log(3, "Error getting peer id.")
				return
			}
		}
	}
}
