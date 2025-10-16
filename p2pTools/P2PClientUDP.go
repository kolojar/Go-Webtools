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

type P2PClientUDPReadFunc func(client *P2PClientUDP, sourceId string, data []byte, ended bool)

type P2PClientUDP struct {
	udpClientCoordinator       *udpTools.UDPClient
	udpClientOtherFromId       webtools.SafeMap[string, *udpTools.UDPClient]
	udpClientOtherToId         webtools.SafeMap[*udpTools.UDPClient, string]
	udpServerOther             *udpTools.UDPServer
	udpServerOtherConns        webtools.SafeMap[*udpTools.UDPServerConn, bool]
	id                         string
	targetId                   string
	isPreparingId              uint8
	isConnecting               uint8
	isConnected                bool
	pendingData                [][]byte
	isIdRelay                  webtools.SafeMap[string, bool]
	reportTraffic              bool
	readFunc                   P2PClientUDPReadFunc
	udpServerOtherConnIdToConn webtools.SafeMap[string, *udpTools.UDPServerConn]
	udpServerOtherConnConnToId webtools.SafeMap[*udpTools.UDPServerConn, string]
}

/*
Creates new P2P UDP client but does not start it
*/
func NewP2PClientUDP(address string, serverPort int, reportTraffic bool) (*P2PClientUDP, error) {
	//Creates new P2P Client
	p2p := &P2PClientUDP{reportTraffic: reportTraffic, udpClientOtherFromId: webtools.MakeSafeMap[string, *udpTools.UDPClient](), udpClientOtherToId: webtools.MakeSafeMap[*udpTools.UDPClient, string](), udpServerOtherConns: webtools.MakeSafeMap[*udpTools.UDPServerConn, bool](), udpServerOtherConnIdToConn: webtools.MakeSafeMap[string, *udpTools.UDPServerConn](), udpServerOtherConnConnToId: webtools.MakeSafeMap[*udpTools.UDPServerConn, string](), isIdRelay: webtools.MakeSafeMap[string, bool]()}
	var err error

	//Create coordinator client
	p2p.udpClientCoordinator, err = udpTools.NewUDPClient(address, p2p.readFuncLocal, reportTraffic)
	if err != nil {
		return nil, err
	}
	p2p.udpClientCoordinator.Logger.Prefix = "P2PClientUDP - CoordinatorUDPClient"

	//Create server
	p2p.udpServerOther, err = udpTools.NewUDPServer("0.0.0.0:"+strconv.Itoa(serverPort), p2p.readFuncLocalOtherServer, reportTraffic)
	if err != nil {
		return nil, err
	}
	p2p.udpServerOther.Logger.Prefix = "P2PClientUDP - RemoteUDPServer"
	return p2p, nil
}

func (p2p *P2PClientUDP) readFuncLocal(client *udpTools.UDPClient, sourceAddress *net.UDPAddr, data []byte, ended bool) {
	//Commands
	command, args := httpTools.CreateParametersFromURL(string(data))
	println(command, "|"+webtools.MapToString(args))
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
			client.Logger.Log(2, "This client id is: "+p2p.id)
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
			client.Logger.Log(2, "Starting punch holing...")

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
					client.Logger.Log(1, "Connected to other peer.")
					break
				}
			}
			p2p.udpClientCoordinator.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_CONNECT_STATUS, map[string]string{"id": p2p.id, "connId": args["connId"], "status": webtools.FormatByBool(p2p.isConnecting == 2, "true", "false")})))
		}
	case P2P_CMD_CONNECT_STATUS:
		{
			//Review status connect request
			if args["id"] == p2p.targetId {
				p2p.udpServerOtherConns.Set(p2p.udpServerOtherConnIdToConn.Get(args["id"]), args["status"] == "true")
				p2p.isConnecting = 4
			}
		}
	}
}

func (p2p *P2PClientUDP) readFuncLocalOtherClient(client *udpTools.UDPClient, sourceAddress *net.UDPAddr, data []byte, ended bool) {
	//Reads from client
	if p2p.isConnecting == 1 {
		if slices.Equal(data, []byte(P2P_CMD_PUNCH)) {
			//Got punch
			client.Logger.Log(1, "Got punch.")
			p2p.isConnecting = 2
		}
		return
	}

	//Transport it to read func
	if p2p.readFunc != nil {
		p2p.readFunc(p2p, p2p.udpClientOtherToId.Get(client), data, ended)
	}
}

func (p2p *P2PClientUDP) readFuncLocalOtherServer(conn *udpTools.UDPServerConn, data []byte, ended bool) {
	//Reads from other connections
	if !p2p.udpServerOtherConns.Get(conn) {
		command, args := httpTools.CreateParametersFromURL(string(data))
		if command != P2P_CMD_PUNCH {
			p2p.udpServerOther.Logger.Log(3, "Invalid commmand.")
		}
		p2p.udpServerOther.Logger.Log(1, "Got punch from: "+conn.Address.String())
		p2p.udpServerOtherConnIdToConn.Set(args["id"], conn)
		p2p.udpServerOtherConnConnToId.Set(conn, args["id"])
		conn.Send([]byte(P2P_CMD_PUNCH))
		return
	}

	//Transport it to read func
	if p2p.readFunc != nil {
		p2p.readFunc(p2p, p2p.udpServerOtherConnConnToId.Get(conn), data, ended)
	}
}

/*
Connects to coordinator,does not lock execution thread
*/
func (p2p *P2PClientUDP) ConnectToCoordinator() bool {
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
	return p2p.isPreparingId == 2

}

/*
Connects to specified id,does not lock execution thread
*/
func (p2p *P2PClientUDP) ConnectToPeer(targetId string) bool {
	//Send connect request and try to connect
	p2p.isConnecting = 1
	p2p.udpClientCoordinator.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_CONNECT, map[string]string{"id": p2p.id, "targetId": targetId})))
	for p2p.isConnecting == 1 {
		time.Sleep(100 * time.Millisecond)
	}
	if p2p.isConnecting == 3 {
		return false
	}
	for p2p.isConnecting < 4 {
		time.Sleep(100 * time.Millisecond)
	}
	return p2p.isConnecting == 4
}

/*
Sends data to peer
*/
func (p2p *P2PClientUDP) Send(targetId string, data []byte) {
	if p2p.isIdRelay.Get(targetId) {
		//Use relay
		p2p.udpClientCoordinator.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_RELAY, map[string]string{"data": string(data), "targetId": targetId, "id": p2p.id})))
		return
	}
	client := p2p.udpClientOtherFromId.Get(targetId)
	if client != nil {
		//Use this as client to write to other side server
		client.Send(data)
		return
	}
	server := p2p.udpServerOtherConnIdToConn.Get(targetId)
	if server != nil {
		//Use this as server to write to other side client
		server.Send(data)
		return
	}
	p2p.udpClientCoordinator.Logger.Log(3, "Error sending data to: "+targetId+" - id not found.")
}

/*
Stops P2P client
*/
func (p2p *P2PClientUDP) Stop() {
	p2p.udpClientCoordinator.Stop()
	//p2p.udpClientOther.Stop()
	p2p.udpServerOther.Stop()
}
