package p2pTools

import (
	"bytes"
	"encoding/binary"
	"net"
	"strconv"
	"time"
	"webtools"
	"webtools/udpTools"
)

// Retry count for punching
const P2P_PUNCH_RETRY_COUNT = 10

// Used for sending punch, in id there is origin id and in data is targetId - server does not modifies frame, just sends it back
const P2P_CMD_PUNCH uint8 = 50

type P2PClientUDP struct {
	//Coordinator
	udpClientCoordinator      *udpTools.UDPClient
	id                        []byte
	isConnecting              bool
	targetIdThisConnecting    []byte
	targetIdsConnectingStatus webtools.SafeMap[string, uint8]
	reportTraffic             bool
	//gotConnected              bool

	//Server for incomming conns
	udpIncommingConnsSv *udpTools.UDPServer
	udpIncommingConns   webtools.SafeMap[string, webtools.KeyValuePair[*udpTools.UDPServerConn, bool]]

	//Clients for outcomming connections
	udpOutcommingConnsCls webtools.SafeMap[string, webtools.KeyValuePair[*udpTools.UDPClient, bool]]
}

/*
Creates new P2P Client for UDP but does not starts it
*/
func NewP2PClientUDP(address string, portForIncommingConns int, reportTraffic bool) (*P2PClientUDP, error) {
	//New P2P
	p2p := &P2PClientUDP{
		id:                        nil,
		isConnecting:              false,
		reportTraffic:             reportTraffic,
		udpOutcommingConnsCls:     webtools.MakeSafeMap[string, webtools.KeyValuePair[*udpTools.UDPClient, bool]](),
		udpIncommingConns:         webtools.MakeSafeMap[string, webtools.KeyValuePair[*udpTools.UDPServerConn, bool]](),
		targetIdsConnectingStatus: webtools.MakeSafeMap[string, uint8](),
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
				p2p.targetIdsConnectingStatus.Set(string(frame.Id), 1)
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
					if p2p.targetIdsConnectingStatus.Get(string(frame.Id)) == 2 {
						//Connected to server
						client.Logger.Log(1, "Connected to other peer, waiting for coordinator.")
						break
					}
				}
				if p2p.targetIdsConnectingStatus.Get(string(frame.Id)) != 2 {
					//Not connected to server
					client.Logger.Log(3, "Could not connect to other peer.")
				}
				break
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
				p2p.targetIdsConnectingStatus.Set(string(frame.Data), 2)
				p2p.udpClientCoordinator.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS, frame.Id, frame.Data))
				break
			}
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
	p2p.gotConnected = true
	p2p.targetIdThisConnecting = []byte(targetId)
	p2p.udpClientCoordinator.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_TO_PEER, p2p.id, []byte(targetId)))

	//Wait for connect
	for p2p.isConnecting {
		time.Sleep(100 * time.Millisecond)
	}
}
