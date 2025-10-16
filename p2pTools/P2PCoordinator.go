package p2pTools

import (
	"strconv"
	"time"
	"webtools"
	"webtools/httpTools"
	"webtools/udpTools"
)

var P2P_FRAMER *udpTools.UDPFramer = udpTools.NewUDPFramerSimple(50, 5, true, 50)

const P2P_TIMEOUT_START = 5

// Gets request for new ID, has parameter port for port on UDP punchhole server -> Sends new ID in parameter id
const P2P_CMD_NEW_ID = "newId"

// Gets request for connection, has parameter id and targetId -> Sends request to start sending UDP requests, returns paramerers: targetId, targetIP, connId and time
const P2P_CMD_CONNECT = "connect"

// Sends info to client to switch to relay
const P2P_CMD_START_RELAY = "startRelay"

// Used for relaying traffic, needs parameter id and targetId and data, returns sourceId
const P2P_CMD_RELAY = "relay"

// Gets info about connect status, has parameters id, connId and status -> Sends to both clients about status of each client, has parameters id and status
const P2P_CMD_CONNECT_STATUS = "connectStatus"

type P2PCoordinator struct {
	udpServer          *udpTools.UDPServer
	peersToConns       webtools.SafeMap[string, webtools.KeyValuePair[*udpTools.UDPServerConn, int]]
	pendingConnections webtools.SafeMap[string, webtools.FiveValuePair[bool, string, bool, string, bool]]
	allowRelay         bool
	peerToId           webtools.SafeMap[*udpTools.UDPServerConn, string]
	idToPeer           webtools.SafeMap[string, *udpTools.UDPServerConn]
}

/*
Creates new UDP P2P Server but does not starts it
5
*/
func NewP2PCoordinator(address string, allowRelay bool, reportTraffic bool) (*P2PCoordinator, error) {
	p2p := &P2PCoordinator{peersToConns: webtools.MakeSafeMap[string, webtools.KeyValuePair[*udpTools.UDPServerConn, int]](), peerToId: webtools.MakeSafeMap[*udpTools.UDPServerConn, string](), pendingConnections: webtools.MakeSafeMap[string, webtools.FiveValuePair[bool, string, bool, string, bool]](), allowRelay: allowRelay}
	var err error
	p2p.udpServer, err = udpTools.NewUDPServer(address, p2p.readFuncLocal, reportTraffic)
	p2p.udpServer.Logger.Prefix = "P2P - " + p2p.udpServer.Logger.Prefix
	if err != nil {
		return nil, err
	}
	return p2p, nil
}

func (p2p *P2PCoordinator) readFuncLocal(conn *udpTools.UDPServerConn, data []byte, ended bool) {
	command, args := httpTools.CreateParametersFromURL(string(data))
	if !p2p.peerToId.Has(conn) {
		//Check values
		if args["port"] == "" {
			p2p.udpServer.Logger.Log(3, "Invalid peer from: "+conn.Address.String())
			return
		}
		port, err := strconv.ParseInt(args["port"], 10, 32)
		if err != nil {
			p2p.udpServer.Logger.Log(3, "Invalid peer from: "+conn.Address.String()+" while converting to int: "+err.Error())
			return
		}

		//No conn find, create new id
		id := "p2p-" + webtools.GenerateRandomId()
		p2p.peersToConns.Set(id, webtools.KeyValuePair[*udpTools.UDPServerConn, int]{Key: conn, Value: int(port)})
		p2p.peerToId.Set(conn, id)
		p2p.idToPeer.Set(id, conn)
		p2p.udpServer.Logger.Log(1, "New peer from: "+conn.Address.String()+" with id: "+id)
		conn.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_NEW_ID, map[string]string{"id": id})))
		return
	}

	//Commands
	switch command {
	case P2P_CMD_CONNECT:
		{
			//Request connect to other peer
			if args["targetId"] == "" {
				p2p.udpServer.Logger.Log(3, "Error getting connection id.")
				return
			}
			if args["id"] == "" {
				p2p.udpServer.Logger.Log(3, "Error getting connection id.")
				return
			}
			if args["id"] != p2p.peerToId.Get(conn) {
				p2p.udpServer.Logger.Log(3, "Peer id does not match.")
				return
			}

			//Get target
			if !p2p.peersToConns.Has(args["targetId"]) {
				p2p.udpServer.Logger.Log(3, "Error getting target connection.")
				return
			}
			target := p2p.peersToConns.Get(args["targetId"])

			//Get start time
			tStart := time.Now().Add(time.Second * P2P_TIMEOUT_START)
			connId := webtools.GenerateRandomId()

			//Send start message
			p2p.udpServer.Logger.Log(1, "Started creating connection between: "+args["id"]+" at IP: "+conn.Address.String()+" and: "+args["targetId"]+" at IP: "+target.Key.Address.String())
			conn.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_CONNECT, map[string]string{"targetId": args["targetId"], "targetIP": target.Key.Address.IP.String() + ":" + strconv.FormatInt(int64(target.Value), 10), "time": strconv.FormatInt(tStart.UnixNano(), 36), "connId": connId})))
			target.Key.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_CONNECT, map[string]string{"targetId": args["id"], "targetIP": conn.Address.IP.String() + ":" + strconv.FormatInt(int64(p2p.peersToConns.Get(args["id"]).Value), 10), "time": strconv.FormatInt(tStart.UnixNano(), 36), "connId": connId})))
			p2p.pendingConnections.Set(connId, webtools.FiveValuePair[bool, string, bool, string, bool]{A: false, B: args["id"], C: false, D: args["targetId"], E: false})

			//Wait for responce
			go func() {
				time.Sleep(2 * time.Second * P2P_TIMEOUT_START)

				//Check responce
				get := p2p.pendingConnections.Get(connId)
				p2p.pendingConnections.Delete(connId)
				if !get.A {
					//No connection was successfull
					p2p.udpServer.Logger.Log(2, "Failed to create P2P connection between: "+args["id"]+" at IP: "+conn.Address.String()+" and: "+args["targetId"]+" at IP: "+target.Key.Address.String())
					if p2p.allowRelay {
						//Send relay request
						p2p.udpServer.Logger.Log(2, "Created relay onnection between: "+args["id"]+" and: "+args["targetId"])
						conn.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_START_RELAY, nil)))
						target.Key.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_START_RELAY, nil)))
					}
					return
				}
				if !get.C {
					//One connection was not successfull
					p2p.udpServer.Logger.Log(1, "Peer at "+args["id"]+" at IP: "+conn.Address.String()+" did not connect.")
				}
				if !get.E {
					//One connection was not successfull
					p2p.udpServer.Logger.Log(1, "Peer at "+args["targetId"]+" at IP: "+target.Key.Address.String()+" did not connect.")
				}
				p2p.udpServer.Logger.Log(1, "P2P connection between: "+args["id"]+" and: "+args["targetId"]+" created successfully.")
			}()
			break
		}
	case P2P_CMD_RELAY:
		{
			//Just relays the traffic
			if !p2p.allowRelay {
				p2p.udpServer.Logger.Log(3, "Relay not available on this server.")
				return
			}
			if args["targetId"] == "" {
				p2p.udpServer.Logger.Log(3, "Error getting connection id.")
				return
			}
			if args["id"] == "" {
				p2p.udpServer.Logger.Log(3, "Error getting connection id.")
				return
			}

			//Get target
			if !p2p.peersToConns.Has(args["targetId"]) {
				p2p.udpServer.Logger.Log(3, "Error getting target connection.")
				return
			}
			target := p2p.peersToConns.Get(args["targetId"])

			//Send data
			target.Key.Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_RELAY, map[string]string{"sourceId": args["id"], "data": args["data"]})))
			break
		}
	case P2P_CMD_CONNECT_STATUS:
		{
			//Used for checking status
			if args["connId"] == "" {
				p2p.udpServer.Logger.Log(3, "Error getting connection id.")
				return
			}
			if args["id"] == "" {
				p2p.udpServer.Logger.Log(3, "Error getting connection id.")
				return
			}
			if args["status"] == "" {
				p2p.udpServer.Logger.Log(3, "Error getting connection status.")
				return
			}

			//Check settings
			if !p2p.peersToConns.Has(args["connId"]) {
				p2p.udpServer.Logger.Log(3, "No connection found for this id: "+args["connId"])
				return
			}
			pair := p2p.pendingConnections.Get(args["connId"])
			status, err := strconv.ParseBool(args["status"])
			if err != nil {
				p2p.udpServer.Logger.Log(3, "Error getting connection status: "+err.Error())
				return
			}

			//Set status
			if status {
				pair.A = status
			}
			if pair.B == args["id"] {
				pair.C = status
			}
			if pair.D == args["id"] {
				pair.E = status
			}
			p2p.pendingConnections.Set(args["connId"], pair)

			//Inform clients
			p2p.idToPeer.Get(pair.B).Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_CONNECT_STATUS, map[string]string{"id": args["id"], "status": args["status"]})))
			p2p.idToPeer.Get(pair.D).Send([]byte(httpTools.CreateURLFromParameters(P2P_CMD_CONNECT_STATUS, map[string]string{"id": args["id"], "status": args["status"]})))
			break
		}
	default:
		{
			p2p.udpServer.Logger.Log(3, "Invalid command")
			return
		}
	}
}

/*
Starts P2P coordinator server
*/
func (p2p *P2PCoordinator) Start() {
	p2p.udpServer.Start()
}

/*
Stops P2P coordinator server
*/
func (p2p *P2PCoordinator) Stop() {
	p2p.udpServer.Stop()
}
