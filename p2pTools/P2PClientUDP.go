package p2pTools

import (
	"net"
	"slices"
	"strconv"
	"time"
	"webtools"
	"webtools/httpTools"
	"webtools/udpTools"
)

const P2P_RETRY_COUNT = 10
const P2P_CMD_PUNCH = "punch"

type P2PClientUDPReadFunc func(client *P2PClientUDP, id string, data []byte, ended bool)

type P2PClientUDP struct {
	udpClientCoordinator *udpTools.UDPClient
	udpClientOtherFromId webtools.SafeMap[string, *udpTools.UDPClient]
	udpClientOtherToId   webtools.SafeMap[*udpTools.UDPClient, string]
	udpServerOther       *udpTools.UDPServer
	udpServerOtherConns  webtools.SafeMap[*udpTools.UDPServerConn, bool]
	id                   string
	isPreparingId        uint8
	isConnecting         uint8
	isConnected          bool
	pendingData          [][]byte
	isClientRelay        webtools.SafeMap[string, bool]
	reportTraffic        bool
	readFunc             P2PClientUDPReadFunc
}

/*
Creates new P2P UDP client but does not start it
*/
func NewP2PClientUDP(address string, reportTraffic bool) (*P2PClientUDP, error) {
	//Creates new P2P Client
	p2p := &P2PClientUDP{reportTraffic: reportTraffic}
	var err error
	p2p.udpClientCoordinator, err = udpTools.NewUDPClient(address, p2p.readFuncLocal, reportTraffic)
	if err != nil {
		return nil, err
	}
	p2p.udpClientCoordinator.Logger.Prefix = "P2PClientUDP - CoordinatorUDPClient"
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
				p2p.udpClientCoordinator.Logger.Log(3, "Error getting peer id.")
				p2p.isPreparingId = 3
				return
			}
			p2p.id = args["id"]
			p2p.isPreparingId = 2
			break
		}
	case P2P_CMD_CONNECT:
		{
			//Read connect request
			if args["targetId"] == "" {
				p2p.udpClientCoordinator.Logger.Log(3, "Error getting peer id.")
				p2p.isConnecting = 3
				return
			}
			if args["targetIP"] == "" {
				p2p.udpClientCoordinator.Logger.Log(3, "Error getting peer IP.")
				p2p.isConnecting = 3
				return
			}
			if args["connId"] == "" {
				p2p.udpClientCoordinator.Logger.Log(3, "Error getting conn id.")
				p2p.isConnecting = 3
				return
			}
			if args["time"] == "" {
				p2p.udpClientCoordinator.Logger.Log(3, "Error getting peer time.")
				p2p.isConnecting = 3
				return
			}

			//Parse values
			timeNano, err := strconv.ParseInt(args["time"], 36, 64)
			if err != nil {
				p2p.udpClientCoordinator.Logger.Log(3, "Error getting peer time: "+err.Error())
				return
			}
			timeStart := time.Unix(0, timeNano)

			//Create client
			client, err := udpTools.NewUDPClient(args["targetIP"], p2p.readFuncLocalOtherClient, p2p.reportTraffic)
			if err != nil {
				p2p.udpClientCoordinator.Logger.Log(3, "Error creating UDP client: "+err.Error())
				return
			}
			client.Logger.Prefix = "P2PClientUDP - PeerClientUDP for id: " + args["targetId"]
			p2p.udpClientOtherFromId.Set(args["targetId"], client)
			p2p.udpClientOtherToId.Set(client, args["targetId"])

			//Wait for time
			time.Sleep(time.Until(timeStart))

			//Start punching
			for i := 0; i < P2P_RETRY_COUNT; i++ {
				client.Logger.Log(1, "Connecting to target IP: "+args["targetIP"]+" attempt: "+strconv.Itoa(i)+"/"+strconv.Itoa(P2P_RETRY_COUNT))
				err = client.Connect()
				if err == nil {
					client.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_PUNCH, map[string]string{"id": p2p.id})))
				} else {
					client.Logger.Log(3, "Error connecting to target IP: "+args["targetIP"]+" with error: "+err.Error())
				}
				time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
				if p2p.isConnecting == 2 {
					//Connected to server
					break
				}
				p2p.udpClientCoordinator.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_CONNECT_STATUS, map[string]string{"id": p2p.id, "connId": args["connId"], "status": webtools.FormatByBool(p2p.isConnecting == 2, "true", "false")})))
			}
		}

	}
}

func (p2p *P2PClientUDP) readFuncLocalOtherClient(client *udpTools.UDPClient, sourceAddress *net.UDPAddr, data []byte, ended bool) {
	//Reads from client
	if p2p.isConnecting == 1 {
		if slices.Equal(data, []byte(P2P_CMD_PUNCH)) {
			//Got punch
			p2p.isConnecting = 2
			client.Logger.Log(1, "Got punch.")
		}
		return
	}

	//Transport it to read func
	if p2p.readFunc != nil {
		p2p.readFunc(p2p, client, sourceAddress, data, ended)
	}
}

func (p2p *P2PClientUDP) readFuncLocalOtherServer(conn *udpTools.UDPServerConn, data []byte, ended bool) {
	//Reads from other connections
	if !p2p.udpServerOtherConns.Get(conn) {
		p2p.udpServerOther.Logger.Log(1, "Got punch from: "+conn.Address.String())
		conn.Send([]byte(P2P_CMD_PUNCH))
		return
	}
}

/*
Connects to specified id,does not lock execution thread
*/
func (p2p *P2PClientUDP) Connect(targetId string) bool {
	//Start and connect to coordinator
	p2p.udpClientCoordinator.Connect()
	go p2p.udpServerOther.Start()
	p2p.isConnecting = 1
	p2p.isPreparingId = 1
	p2p.udpClientCoordinator.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_NEW_ID, map[string]string{"port": strconv.FormatInt(int64(p2p.udpServerOther.GetAddress().Port), 10)})))

	//Wait for ID
	for p2p.isPreparingId == 1 {
		time.Sleep(100 * time.Millisecond)
	}
	if p2p.isPreparingId == 3 {
		return false
	}

	//Send connect request and try to connect
	p2p.udpClientCoordinator.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_CONNECT, map[string]string{"id": p2p.id, "targetId": targetId})))
}

/*
Stops P2P client
*/
func (p2p *P2PClientUDP) Stop() {
	p2p.udpClientCoordinator.Stop()
	p2p.udpClientOther.Stop()
	p2p.udpServerOther.Stop()
}
