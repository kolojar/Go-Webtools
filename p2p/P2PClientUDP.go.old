package p2pTools

import (
	"net"
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
	//Coordination
	udpClientCoordinator *udpTools.UDPClient
	id                   string
	readFunc             P2PClientUDPReadFunc
	isPreparingId        bool
	reportTraffic        bool

	//Connecting
	//isConnectingToSomething bool
	isConnecting bool
	gotConnected bool
	targetId     string

	//In format: ID - Is connected via Server / Client / Relay
	peerStatuses webtools.SafeMap[string, webtools.ThreeValuePair[bool, bool, bool]]

	//Server connections resolve
	udpServerOther   *udpTools.UDPServer
	serverConnsToIds webtools.SafeMap[*udpTools.UDPServerConn, string]
	serverIdsToConns webtools.SafeMap[string, webtools.KeyValuePair[*udpTools.UDPServerConn, bool]]

	//Client connection resolve
	clientConnsToIds webtools.SafeMap[*udpTools.UDPClient, string]
	clientIdsToConns webtools.SafeMap[string, webtools.KeyValuePair[*udpTools.UDPClient, bool]]

	//Pending data
	//pendingSendData webtools.SafeMap[string, [][]byte]
}

/*
Creates new P2P UDP client but does not start it
*/
func NewP2PClientUDP(address string, serverPort int, readFunc P2PClientUDPReadFunc, reportTraffic bool) (*P2PClientUDP, error) {
	//Creates new P2P Client
	p2p := &P2PClientUDP{
		reportTraffic:    reportTraffic,
		readFunc:         readFunc,
		peerStatuses:     webtools.MakeSafeMap[string, webtools.ThreeValuePair[bool, bool, bool]](),
		serverConnsToIds: webtools.MakeSafeMap[*udpTools.UDPServerConn, string](),
		serverIdsToConns: webtools.MakeSafeMap[string, webtools.KeyValuePair[*udpTools.UDPServerConn, bool]](),
		clientConnsToIds: webtools.MakeSafeMap[*udpTools.UDPClient, string](),
		clientIdsToConns: webtools.MakeSafeMap[string, webtools.KeyValuePair[*udpTools.UDPClient, bool]](),
	}
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
				p2p.isPreparingId = false
				p2p.id = ""
				return
			}
			p2p.id = args["id"]
			client.Logger.Log(2, "This client id is: "+p2p.id)
			p2p.isPreparingId = false
			break
		}
	case P2P_CMD_CONNECT:
		{
			//Read connect request
			if args["targetId"] == "" {
				p2p.udpClientCoordinator.Logger.Log(3, "Error getting peer id.")
				p2p.isConnecting = false
				return
			}
			if args["targetIP"] == "" {
				p2p.udpClientCoordinator.Logger.Log(3, "Error getting peer IP.")
				p2p.isConnecting = false
				return
			}
			if args["connId"] == "" {
				p2p.udpClientCoordinator.Logger.Log(3, "Error getting conn id.")
				p2p.isConnecting = false
				return
			}
			if args["time"] == "" {
				p2p.udpClientCoordinator.Logger.Log(3, "Error getting peer time.")
				p2p.isConnecting = false
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
			p2p.clientIdsToConns.Set(args["targetId"], webtools.KeyValuePair[*udpTools.UDPClient, bool]{Key: client, Value: false})
			p2p.clientConnsToIds.Set(client, args["targetId"])

			//Wait for time
			time.Sleep(time.Until(timeStart))

			//Start punching
			for i := 0; i < P2P_RETRY_COUNT; i++ {
				client.Logger.Log(1, "Connecting to target IP: "+args["targetIP"]+" attempt: "+strconv.Itoa(i)+"/"+strconv.Itoa(P2P_RETRY_COUNT))
				err = client.Connect()
				if err == nil {
					client.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_PUNCH, map[string]string{"id": p2p.id, "connId": args["connId"]})))
				} else {
					client.Logger.Log(3, "Error connecting to target IP: "+args["targetIP"]+" with error: "+err.Error())
				}
				time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
				if p2p.gotConnected {
					//Connected to server
					client.Logger.Log(1, "Connected to other peer.")
					break
				}
			}
			p2p.udpClientCoordinator.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_CONNECT_STATUS, map[string]string{"id": p2p.id, "connId": args["connId"], "status": webtools.FormatByBool(p2p.gotConnected, "true", "false")})))
		}
	case P2P_CMD_CONNECT_STATUS:
		{
			//Review status connect request
			if args["id"] == "" {
				client.Logger.Log(3, "No id from server.")
				return
			}
			if args["connType"] == "" {
				client.Logger.Log(3, "No connType from server.")
				return
			}

			//Decode data
			if !p2p.peerStatuses.Has(args["id"]) {
				println(args["id"])
				p2p.peerStatuses.Set(args["id"], webtools.ThreeValuePair[bool, bool, bool]{A: false, B: false, C: false})
			}
			get := p2p.peerStatuses.Get(args["id"])
			if args["connType"] == "server" {
				get.A = true
				get2 := p2p.serverIdsToConns.Get(args["id"])
				get2.Value = true
				p2p.serverIdsToConns.Set(args["id"], get2)
				//p2p.gotConnected = true
			} else {
				get.B = true
			}
			p2p.peerStatuses.Set(args["id"], get)
			p2p.gotConnected = true
			p2p.isConnecting = false
			break
		}
	case P2P_CMD_CONNECT_DONE:
		{
			//Connecting done
			p2p.isConnecting = false
		}
	case P2P_CMD_RELAY:
		{
			//Relay
			if args["sourceId"] == "" {
				client.Logger.Log(3, "Invalid source id.")
				return
			}
			if p2p.readFunc != nil {
				p2p.readFunc(p2p, args["sourceId"], []byte(args["data"]), args["ended"] == "true")
			}
		}
	}
}

func (p2p *P2PClientUDP) readFuncLocalOtherClient(client *udpTools.UDPClient, sourceAddress *net.UDPAddr, data []byte, ended bool) {
	//Reads from client
	get := p2p.clientIdsToConns.Get(p2p.clientConnsToIds.Get(client))
	if !get.Value {
		command, args := httpTools.CreateParametersFromURL(string(data))
		println(command + "|" + webtools.MapToString(args))
		if command == P2P_CMD_PUNCH {
			//Got punch
			client.Logger.Log(1, "Got punch.")
			get.Value = true
			p2p.clientIdsToConns.Set(p2p.clientConnsToIds.Get(client), get)
			p2p.udpClientCoordinator.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_CONNECT_STATUS, map[string]string{"id": p2p.id, "connId": args["connId"], "status": "true"})))
		}
		return
	}

	//Transport it to read func
	if p2p.readFunc != nil {
		p2p.readFunc(p2p, p2p.clientConnsToIds.Get(client), data, ended)
	}
}

func (p2p *P2PClientUDP) readFuncLocalOtherServer(conn *udpTools.UDPServerConn, data []byte, ended bool) {
	//Reads from other connections
	get := p2p.serverIdsToConns.Get(p2p.serverConnsToIds.Get(conn))
	if !get.Value {
		command, args := httpTools.CreateParametersFromURL(string(data))
		println(command + "|" + webtools.MapToString(args))
		if command != P2P_CMD_PUNCH {
			p2p.udpServerOther.Logger.Log(3, "Invalid commmand.")
			return
		}
		p2p.udpServerOther.Logger.Log(1, "Got punch from: "+conn.Address.String())
		conn.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_PUNCH, map[string]string{"connId": args["connId"]})))
		return
	}

	//Transport it to read func
	if p2p.readFunc != nil {
		p2p.readFunc(p2p, p2p.serverConnsToIds.Get(conn), data, ended)
	}
}

/*
Connects to coordinator,does not lock execution thread
*/
func (p2p *P2PClientUDP) ConnectToCoordinator() bool {
	//Start and connect to coordinator
	p2p.udpClientCoordinator.Connect()
	go p2p.udpServerOther.Start()
	p2p.isPreparingId = true
	p2p.udpClientCoordinator.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_NEW_ID, map[string]string{"port": strconv.FormatInt(int64(p2p.udpServerOther.GetAddress().Port), 10)})))

	//Wait for ID
	for p2p.isPreparingId {
		time.Sleep(100 * time.Millisecond)
	}
	return p2p.id != ""

}

/*
Connects to specified id,does not lock execution thread
*/
func (p2p *P2PClientUDP) ConnectToPeer(targetId string) bool {
	for p2p.isConnecting {
		//Wait, still connecting to something else
		time.Sleep(100 * time.Millisecond)
	}

	//Send connect request and try to connect
	p2p.targetId = targetId
	p2p.isConnecting = true
	//p2p.isConnectingToSomething = true
	p2p.gotConnected = false
	p2p.udpClientCoordinator.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_CONNECT, map[string]string{"id": p2p.id, "targetId": targetId})))
	for p2p.isConnecting {
		time.Sleep(100 * time.Millisecond)
	}
	println("Connecting done")
	return p2p.gotConnected
}

/*
Sends data to peer
*/
func (p2p *P2PClientUDP) Send(targetId string, data []byte) {
	get := p2p.peerStatuses.Get(targetId)
	if get.C {
		//Use relay
		p2p.udpClientCoordinator.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_RELAY, map[string]string{"data": string(data), "targetId": targetId, "id": p2p.id})))
		return
	}
	if get.B {
		//Use this as client to write to other side server
		p2p.clientIdsToConns.Get(targetId).Key.Send(data)
		return
	}
	if get.A {
		//Use this as server to write to other side client
		p2p.serverIdsToConns.Get(targetId).Key.Send(data)
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
