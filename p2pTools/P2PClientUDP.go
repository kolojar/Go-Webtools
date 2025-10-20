package p2pTools

import (
	"bytes"
	"encoding/binary"
	"net"
	"slices"
	"strconv"
	"time"
	"webtools"
	"webtools/udpTools"
)

// Retry count for punching
const P2P_PUNCH_RETRY_COUNT = 10

// Used for sending punch, in id there is origin id and in data is targetId - server does not modifies frame, just sends it back
const P2P_CMD_PUNCH uint8 = 50

type P2PClientUDPReadFunc func(client *P2PClientUDP, sourceId []byte, data []byte, ended bool)

type P2PClientUDP struct {
	//Coordinator
	udpClientCoordinator      *udpTools.UDPClient
	id                        []byte
	isConnecting              bool
	targetIdThisConnecting    []byte
	targetIdsConnectingStatus webtools.SafeMap[string, bool]
	reportTraffic             bool
	allowRelay                webtools.SafeMap[string, bool]
	readFunc                  P2PClientUDPReadFunc

	//Server for incomming conns
	udpIncommingConnsSv *udpTools.UDPServer
	udpIncommingConns   webtools.SafeMap[string, webtools.KeyValuePair[*udpTools.UDPServerConn, bool]]

	//Clients for outcomming connections
	udpOutcommingConnsCls webtools.SafeMap[string, webtools.KeyValuePair[*udpTools.UDPClient, bool]]
}

/*
Creates new P2P Client for UDP but does not starts it
*/
func NewP2PClientUDP(address string, portForIncommingConns int, readFunc P2PClientUDPReadFunc, reportTraffic bool) (*P2PClientUDP, error) {
	//New P2P
	p2p := &P2PClientUDP{
		id:                        nil,
		isConnecting:              false,
		reportTraffic:             reportTraffic,
		udpOutcommingConnsCls:     webtools.MakeSafeMap[string, webtools.KeyValuePair[*udpTools.UDPClient, bool]](),
		udpIncommingConns:         webtools.MakeSafeMap[string, webtools.KeyValuePair[*udpTools.UDPServerConn, bool]](),
		targetIdsConnectingStatus: webtools.MakeSafeMap[string, bool](),
		allowRelay:                webtools.MakeSafeMap[string, bool](),
		readFunc:                  readFunc,
	}

	//New client for Coordinator
	var err error
	p2p.udpClientCoordinator, err = udpTools.NewUDPClient(address, p2p.readFuncCoordinator, reportTraffic)
	if err != nil {
		return nil, err
	}
	p2p.udpClientCoordinator.Logger.Prefix = "P2PClientUDP - CoordinatorClient"
	p2p.udpClientCoordinator.SetupFraming(udpTools.NewUDPFramerSimpleFromConfig(P2P_FRAMER_CONFIG))

	//Setup server
	p2p.udpIncommingConnsSv, err = udpTools.NewUDPServer("0.0.0.0:"+strconv.Itoa(portForIncommingConns), p2p.readFuncIncommingServer, reportTraffic)
	if err != nil {
		return nil, err
	}
	p2p.udpIncommingConnsSv.Logger.Prefix = "P2PClientUDP - IncommingServer"
	return p2p, nil
}

func (p2p *P2PClientUDP) readFuncCoordinator(_ *udpTools.UDPClient, sourceAddress *net.UDPAddr, data []byte, ended bool) {
	for _, frame := range webtools.UnpackWebtoolsFrame(data, p2p.udpClientCoordinator.Logger) {
		switch frame.Operation {
		case P2P_CMD_NEW_ID:
			{
				//New Id
				p2p.id = frame.Data
				p2p.udpClientCoordinator.Logger.Log(2, "This client id is: "+string(p2p.id))
				break
			}
		case P2P_CMD_START_PUNCHING:
			{
				//Start punch holing
				if frame.Id == nil {
					p2p.udpClientCoordinator.Logger.Log(3, "Invalid id in frame.")
					return
				}
				if frame.Data == nil {
					p2p.udpClientCoordinator.Logger.Log(3, "Invalid data in frame.")
					return
				}

				//Split data
				split := bytes.SplitN(data, []byte{webtools.WEBTOOLS_FRAME_SEPARATOR}, 2)
				if len(split) != 2 {
					p2p.udpClientCoordinator.Logger.Log(3, "Invalid split data in frame.")
					return
				}
				startTime := time.Unix(0, int64(binary.LittleEndian.Uint64(split[0])))

				//Create new client
				client, err := udpTools.NewUDPClient(string(split[1]), p2p.readFuncOutcommingClients, p2p.reportTraffic)
				if err != nil {
					p2p.udpClientCoordinator.Logger.Log(3, "Error creating UDP client: "+err.Error())
					return
				}
				client.Logger.Prefix = "P2PClientUDP - PeerClientUDP for id: " + string(frame.Id)
				//p2p.clientIdsToConns.Set(args["targetId"], webtools.KeyValuePair[*udpTools.UDPClient, bool]{Key: client, Value: false})
				//p2p.clientConnsToIds.Set(client, args["targetId"])

				//Wait for time
				p2p.targetIdsConnectingStatus.Set(string(frame.Id), true)
				time.Sleep(time.Until(startTime))

				//Start punching
				for i := 0; i < P2P_PUNCH_RETRY_COUNT; i++ {
					client.Logger.Log(1, "Connecting to target IP: "+string(frame.Id)+" attempt: "+strconv.Itoa(i+1)+"/"+strconv.Itoa(P2P_PUNCH_RETRY_COUNT))
					err = client.Connect()
					if err == nil {
						client.Send(webtools.PackWebtoolsFrame(P2P_CMD_PUNCH, p2p.id, frame.Id))
					} else {
						client.Logger.Log(3, "Error connecting to target IP: "+string(split[1])+" with error: "+err.Error())
					}
					time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
					if !p2p.targetIdsConnectingStatus.Get(string(frame.Id)) {
						//Connected to server
						client.Logger.Log(1, "Connected to other peer, waiting for coordinator.")
						break
					}
				}
				if p2p.targetIdsConnectingStatus.Get(string(frame.Id)) {
					//Not connected to server
					client.Logger.Log(3, "Could not connect to other peer.")
				}
				break
			}
		case P2P_CMD_CONNECT_STATUS:
			{
				//Status about connection
				switch string(frame.Data) {
				case "server":
					{
						//Set true for server
						if !p2p.udpIncommingConns.Has(string(frame.Id)) {
							p2p.udpClientCoordinator.Logger.Log(3, "Incomming connection not found: "+string(frame.Id))
							return
						}
						val := p2p.udpIncommingConns.Get(string(frame.Id))
						val.Value = true
						p2p.udpIncommingConns.Set(string(frame.Id), val)
						break
					}
				case "client":
					{
						//Set true for client
						if !p2p.udpOutcommingConnsCls.Has(string(frame.Id)) {
							p2p.udpClientCoordinator.Logger.Log(3, "Outcomming connection not found: "+string(frame.Id))
							return
						}
						val := p2p.udpOutcommingConnsCls.Get(string(frame.Id))
						val.Value = true
						p2p.udpOutcommingConnsCls.Set(string(frame.Id), val)
						break
					}
				default:
					{
						p2p.udpClientCoordinator.Logger.Log(3, "Invalid connection type: "+string(frame.Data))
						return
					}
				}
				break
			}
		case P2P_CMD_CONNECT_DONE:
			{
				//Punching done
				p2p.allowRelay.Set(string(frame.Id), string(frame.Data) == "true")
				if slices.Equal(p2p.targetIdThisConnecting, frame.Id) {
					p2p.isConnecting = false
				}
				p2p.targetIdsConnectingStatus.Delete(string(frame.Id))
				p2p.udpClientCoordinator.Logger.Log(2, "Connecting done.")
				break
			}
		case P2P_CMD_CANCEL_CLIENT:
			{
				//Cancels current operation
				p2p.udpClientCoordinator.Logger.Log(3, "Error from server: "+string(frame.Data))
				if p2p.targetIdsConnectingStatus.Has(string(frame.Id)) {
					p2p.targetIdsConnectingStatus.Set(string(frame.Data), false)
				}
				if p2p.udpIncommingConns.Has(string(frame.Id)) {
					val, ok := p2p.udpIncommingConns.GetHas(string(frame.Id))
					if ok {
						val.Value = false
						p2p.udpIncommingConns.Set(string(frame.Id), val)
					}
				}
				if p2p.udpOutcommingConnsCls.Has(string(frame.Id)) {
					val, ok := p2p.udpOutcommingConnsCls.GetHas(string(frame.Id))
					if ok {
						val.Value = false
						p2p.udpOutcommingConnsCls.Set(string(frame.Id), val)
					}
				}
				if p2p.allowRelay.Has(string(frame.Id)) {
					p2p.allowRelay.Set(string(frame.Id), false)
				}
				if bytes.Equal(p2p.targetIdThisConnecting, frame.Id) {
					p2p.isConnecting = false
				}
			}
		case P2P_CMD_RELAY:
			{
				//Get relay data
				if p2p.readFunc != nil {
					p2p.readFunc(p2p, frame.Id, frame.Data, ended)
				}
			}
		}
	}
}

func (p2p *P2PClientUDP) readFuncOutcommingClients(client *udpTools.UDPClient, sourceAddress *net.UDPAddr, data []byte, ended bool) {
	for _, frame := range webtools.UnpackWebtoolsFrame(data, p2p.udpClientCoordinator.Logger) {
		switch frame.Operation {
		case P2P_CMD_PUNCH:
			{
				//Got punch, process as OK
				p2p.targetIdsConnectingStatus.Set(string(frame.Data), false)
				p2p.udpClientCoordinator.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS, frame.Id, frame.Data))
				break
			}
		}
		if p2p.readFunc != nil {
			p2p.readFunc(p2p, frame.Id, frame.Data, ended)
		}
	}
}

func (p2p *P2PClientUDP) readFuncIncommingServer(conn *udpTools.UDPServerConn, data []byte, ended bool) {
	for _, frame := range webtools.UnpackWebtoolsFrame(data, p2p.udpClientCoordinator.Logger) {
		switch frame.Operation {
		case P2P_CMD_PUNCH:
			{
				//Got punch, just resend
				p2p.udpIncommingConnsSv.Logger.Log(2, "Got punch from: "+string(frame.Id)+" at IP: "+conn.Address.String())
				p2p.udpIncommingConns.Set(string(frame.Id), webtools.KeyValuePair[*udpTools.UDPServerConn, bool]{Key: conn, Value: false})
				conn.Send(data)
				break
			}
		}
		if p2p.readFunc != nil {
			p2p.readFunc(p2p, frame.Id, frame.Data, ended)
		}
	}
}

/*
Connects to coordinator, does not lock execution thread
*/
func (p2p *P2PClientUDP) ConnectToCoordinator() bool {
	if p2p.id != nil {
		return true
	}

	//Connect
	err := p2p.udpClientCoordinator.Connect()
	if err != nil {
		p2p.udpClientCoordinator.Logger.Log(3, "Error connecting to coordinator server: "+err.Error())
		return false
	}

	//Wait for ID
	for p2p.id == nil {
		time.Sleep(100 * time.Millisecond)
	}
	return true
}

/*
Connects to specified id,does not lock execution thread
*/
func (p2p *P2PClientUDP) ConnectToPeer(targetId string) bool {
	for p2p.isConnecting {
		time.Sleep(100 * time.Millisecond)
	}

	//Send request to Coordinator
	p2p.isConnecting = true
	p2p.targetIdThisConnecting = []byte(targetId)
	p2p.udpClientCoordinator.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_TO_PEER, p2p.id, []byte(targetId)))

	//Wait for connect
	for p2p.isConnecting {
		time.Sleep(100 * time.Millisecond)
	}

	//Return status
	return p2p.udpIncommingConns.Get(targetId).Value || p2p.udpOutcommingConnsCls.Get(targetId).Value || p2p.allowRelay.Get(targetId)
}

/*
Sends data to target peer
*/
func (p2p *P2PClientUDP) Send(targetId []byte, data []byte) bool {
	//Handle outcomming
	outcomming, ok := p2p.udpOutcommingConnsCls.GetHas(string(targetId))
	if ok && outcomming.Value {
		//Send using client
		outcomming.Key.Send(data)
		return true
	}

	//Handle intcomming
	incomming, ok := p2p.udpIncommingConns.GetHas(string(targetId))
	if ok && incomming.Value {
		//Send using server
		incomming.Key.Send(data)
		return true
	}

	//Handle relay
	relay, ok := p2p.allowRelay.GetHas(string(targetId))
	if ok && relay {
		//Send using relay
		p2p.udpClientCoordinator.Send(webtools.PackWebtoolsFrame(P2P_CMD_RELAY, p2p.id, append(append([]byte(targetId), webtools.WEBTOOLS_FRAME_SEPARATOR), data...)))
		return true
	}

	p2p.udpClientCoordinator.Logger.Log(3, "Failed to send message to peer Id: "+string(targetId))
	return false
}

/*
Stops P2P client
*/
func (p2p *P2PClientUDP) Stop() {
	p2p.udpClientCoordinator.Stop()
	for _, v := range p2p.udpIncommingConns.GetValues() {
		v.Key.Close()
	}
	p2p.udpIncommingConnsSv.Stop()
}
