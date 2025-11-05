package p2p

//FIX NOT RUNNING TCP SERVER!

import (
	"bytes"
	"encoding/binary"
	"net"
	"strconv"
	"strings"
	"time"
	"webtools"
	"webtools/tcp"
	"webtools/udp"
)

// Standard framing
var P2P_FRAMER_CONFIG = &udp.FramerConfig{IsOrganised: true, OrganisedTimeoutInMs: 50, TimeoutForResendInMs: 50, ResendMaxLimit: 5, UseKeepAlive: true}

// Wait time in seconds for P2P puncholing to start
const P2P_TIMEOUT_START = 5

// Request new ID for client, port value in data -> Returns frame with ID as ID data as public IP
const P2P_CMD_NEW_ID uint8 = 1

// Requests for start of new connection between peers, in data there is targetID -> Starts connection setting -> Returns if connection was successfull in data: server/client/relay/error
const P2P_CMD_CONNECT_TO_PEER uint8 = 2

// Cancels current command on client with ID if afected ID, with message in data
const P2P_CMD_CANCEL_CLIENT uint8 = 3

// Informs clients to start punchholding - ID is targetID, data it startTime;ipAddress
const P2P_CMD_START_PUNCHING uint8 = 4

// Informs server about success connection of UDP - is is sourceID, data is targetID -> Sends info about ID and it connection status as data - server / client
const P2P_CMD_CONNECT_STATUS_UDP uint8 = 5

// Informs client about connection done, sends ID and in data if relay is available
const P2P_CMD_CONNECT_DONE uint8 = 6

// Request of send relay - ID is source ID, data is targetID;data -> Sends to other peer in format: ID - sourceID, data - data
const P2P_CMD_RELAY uint8 = 7

// Informs server about success connection of TCP - ID is sourceID, data is targetID -> Sends info about ID and it connection status as data - server / client
const P2P_CMD_CONNECT_STATUS_TCP uint8 = 8

// Requires server to associate this TCP connection to ID, requires ID as sourceID
const P2P_CMD_ASSOCIATE_TCP uint8 = 9

type P2PCoordinatorConn struct {
	conn          *udp.ServerConn
	connTCP       *tcp.ServerConn
	ID            []byte
	port          int
	sourceAddress string
}

func (conn *P2PCoordinatorConn) Send(data []byte) {
	if conn.conn != nil {
		conn.conn.Send(data)
	} else {
		conn.connTCP.Send(data)
	}
}

type P2PCoordinator struct {
	udpServer        *udp.Server
	tcpServer        *tcp.Server
	IDsToConns       webtools.SafeMap[string, *P2PCoordinatorConn]
	punchingConnsUDP webtools.SafeMap[string, webtools.SafeMap[string, webtools.ThreeValuePair[bool, bool, bool]]]
	punchingConnsTCP webtools.SafeMap[string, webtools.SafeMap[string, webtools.ThreeValuePair[bool, bool, bool]]]
	AllowRelay       bool
}

/*
Creates new P2P Coordinator server but does not start it
*/
func NewP2PCoordinator(address string, allowRelay bool, reportTraffic bool) (*P2PCoordinator, error) {
	//New coordinator
	p2p := &P2PCoordinator{
		IDsToConns:       webtools.MakeSafeMap[string, *P2PCoordinatorConn](),
		punchingConnsUDP: webtools.MakeSafeMap[string, webtools.SafeMap[string, webtools.ThreeValuePair[bool, bool, bool]]](),
		punchingConnsTCP: webtools.MakeSafeMap[string, webtools.SafeMap[string, webtools.ThreeValuePair[bool, bool, bool]]](),
		AllowRelay:       allowRelay,
	}

	//New UDP server
	var err error
	p2p.udpServer, err = udp.NewServer(address, p2p.readFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	p2p.udpServer.SetupFraming(udp.NewUDPFramerSimpleFromConfig(P2P_FRAMER_CONFIG, p2p.sendFailFunc))
	p2p.udpServer.Logger.Prefix = "P2PCoordinator - " + p2p.udpServer.Logger.Prefix

	//New TCP server
	p2p.tcpServer, err = tcp.NewServer(address, p2p.readFuncTCP, reportTraffic, true)
	if err != nil {
		return nil, err
	}
	p2p.tcpServer.Logger.Prefix = "P2PCoordinator - " + p2p.tcpServer.Logger.Prefix

	return p2p, nil
}

func (p2p *P2PCoordinator) sendFailFunc(address *net.UDPAddr, data []byte, _ bool) {
	//Failed to send traffic to client using UDP, switch to TCP
	for _, v := range p2p.IDsToConns.GetValues() {
		if v.conn.Address == address {
			v.connTCP.Send(data)
			break
		}
	}

}

func (p2p *P2PCoordinator) generateNewID(portByte []byte, address string, sourceUDP *udp.ServerConn, sourceTCP *tcp.ServerConn) string {
	ID := "p2pConn-" + webtools.GenerateRandomID()
	port, err := strconv.Atoi(string(portByte))
	if err != nil {
		p2p.udpServer.Logger.Log(3, "InvalID port number from: "+address)
		if sourceUDP != nil {
			sourceUDP.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, []byte("0"), []byte("InvalID port number")))
		} else {
			sourceTCP.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, []byte("0"), []byte("InvalID port number")))
		}
		return ""
	}
	p2p.IDsToConns.Set(ID, &P2PCoordinatorConn{conn: sourceUDP, connTCP: sourceTCP, ID: []byte(ID), port: port, sourceAddress: address})
	p2p.IDsToConns.Get(ID).Send(webtools.PackWebtoolsFrame(P2P_CMD_NEW_ID, []byte(ID), []byte(address)))
	p2p.udpServer.Logger.Log(1, "Connection at: "+address+" has this new ID: "+ID)
	return ID
}

func (p2p *P2PCoordinator) readFuncTCP(conn *tcp.ServerConn, data []byte, status uint8) {
	for _, frame := range webtools.UnpackWebtoolsFrame(data, p2p.udpServer.Logger) {
		if frame.Operation == P2P_CMD_ASSOCIATE_TCP {
			//Associate with this conn
			connP2P := p2p.IDsToConns.Get(string(frame.ID))
			connP2P.connTCP = conn
			p2p.IDsToConns.Set(string(frame.ID), connP2P)
			continue
		}
		if frame.Operation == P2P_CMD_NEW_ID {
			//Generates new ID
			p2p.generateNewID(frame.Data, strings.Split(conn.GetConn().RemoteAddr().String(), ":")[0], nil, conn)
			continue
		}
		p2p.readFunc(nil, data, status == webtools.DisconnectStatus)
	}
}

func (p2p *P2PCoordinator) readFunc(conn *udp.ServerConn, data []byte, ended bool) {
	for _, frame := range webtools.UnpackWebtoolsFrame(data, p2p.udpServer.Logger) {
		if frame.Operation == P2P_CMD_NEW_ID {
			//Generates new ID
			p2p.generateNewID(frame.Data, conn.Address.IP.String(), conn, nil)
			continue
		}
		if string(frame.ID) == "" || string(frame.ID) == "0" || frame.ID == nil {
			p2p.udpServer.Logger.Log(3, "InvalID connection from: "+conn.Address.String())
			conn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, frame.ID, []byte("InvalID connection")))
			continue
		}

		//Sort commands
		p2pConn := p2p.IDsToConns.Get(string(frame.ID))
		switch frame.Operation {
		case P2P_CMD_CONNECT_TO_PEER:
			{
				//Check if ID is on server
				target := p2p.IDsToConns.Get(string(frame.Data))
				if target == nil {
					//No ID on server
					p2p.udpServer.Logger.Log(3, "This ID is not on server: "+string(frame.Data))
					p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, frame.Data, []byte("Target ID is not on server")))
					continue
				}

				//Get start time
				var tStartBinary = make([]byte, 8)
				binary.LittleEndian.PutUint64(tStartBinary, uint64(time.Now().Add(time.Second*P2P_TIMEOUT_START).UnixNano()))

				//Make entry in punchingConns
				var mapValue webtools.SafeMap[string, webtools.ThreeValuePair[bool, bool, bool]]
				if p2p.punchingConnsUDP.Has(string(frame.ID)) {
					mapValue = p2p.punchingConnsUDP.Get(string(frame.ID))
				} else {
					mapValue = webtools.MakeSafeMap[string, webtools.ThreeValuePair[bool, bool, bool]]()
				}
				mapValue.Set(string(frame.Data), webtools.ThreeValuePair[bool, bool, bool]{A: false, B: false, C: false})
				p2p.punchingConnsUDP.Set(string(frame.ID), mapValue)
				p2p.punchingConnsTCP.Set(string(frame.ID), mapValue)
				//connID := webtools.GenerateRandomID()

				//Send to clients
				p2p.udpServer.Logger.Log(2, "Starting punching between: "+string(frame.ID)+" at: "+p2pConn.sourceAddress+" and: "+string(frame.Data)+" at: "+target.sourceAddress)
				p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_START_PUNCHING, frame.Data, append(append(tStartBinary, webtools.FrameSeparatorChar), []byte(target.sourceAddress+":"+strconv.Itoa(target.port))...)))
				target.Send(webtools.PackWebtoolsFrame(P2P_CMD_START_PUNCHING, frame.ID, append(append(tStartBinary, webtools.FrameSeparatorChar), []byte(p2pConn.sourceAddress+":"+strconv.Itoa(p2pConn.port))...)))

				//Wait for responce
				go func() {
					time.Sleep(2 * time.Second * P2P_TIMEOUT_START)

					//Send results UDP
					mapValue = p2p.punchingConnsUDP.Get(string(frame.ID))
					connSettings := mapValue.Get(string(frame.Data))
					if connSettings.A {
						p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_UDP, frame.Data, []byte("client")))
						target.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_UDP, frame.ID, []byte("server")))
						p2p.udpServer.Logger.Log(1, "Punching between: "+string(frame.ID)+" and: "+string(frame.Data)+" was successfull.")
					}
					if connSettings.B {
						p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_UDP, frame.Data, []byte("server")))
						target.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_UDP, frame.ID, []byte("client")))
						p2p.udpServer.Logger.Log(1, "Punching between: "+string(frame.ID)+" and: "+string(frame.Data)+" was successfull.")
					}
					connSettings.C = p2p.AllowRelay
					mapValue.Set(string(frame.Data), connSettings)
					p2p.punchingConnsUDP.Set(string(frame.ID), mapValue)

					//Send results TCP
					mapValue = p2p.punchingConnsTCP.Get(string(frame.ID))
					connSettings = mapValue.Get(string(frame.Data))
					if connSettings.A {
						p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_TCP, frame.Data, []byte("client")))
						target.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_TCP, frame.ID, []byte("server")))
						p2p.udpServer.Logger.Log(1, "Punching between: "+string(frame.ID)+" and: "+string(frame.Data)+" was successfull.")
					}
					if connSettings.B {
						p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_TCP, frame.Data, []byte("server")))
						target.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_TCP, frame.ID, []byte("client")))
						p2p.udpServer.Logger.Log(1, "Punching between: "+string(frame.ID)+" and: "+string(frame.Data)+" was successfull.")
					}
					connSettings.C = p2p.AllowRelay
					mapValue.Set(string(frame.Data), connSettings)
					p2p.punchingConnsTCP.Set(string(frame.ID), mapValue)

					//Finish results
					time.Sleep(500 * time.Millisecond)
					p2p.udpServer.Logger.Log(2, "Punching between: "+string(frame.ID)+" and: "+string(frame.Data)+" is done.")
					p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_DONE, frame.Data, []byte(strconv.FormatBool(p2p.AllowRelay))))
					target.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_DONE, frame.ID, []byte(strconv.FormatBool(p2p.AllowRelay))))

				}()
				break
			}
		case P2P_CMD_CONNECT_STATUS_UDP:
			{
				//Set connect status
				success := false
				if p2p.handleConnectStatus(string(frame.ID), string(frame.Data), false) {
					success = true
				}
				if p2p.handleConnectStatus(string(frame.Data), string(frame.ID), true) {
					success = true
				}

				//Handle if not success
				if !success {
					p2p.udpServer.Logger.Log(3, "This ID is not punching list: "+string(frame.ID)+" or this ID: "+string(frame.Data))
					p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, frame.Data, []byte("Source or Target ID is not on punching list")))
					continue
				}
				break
			}
		case P2P_CMD_RELAY:
			{
				//Handle relay
				var success bool = false
				data := bytes.SplitN(frame.Data, []byte{webtools.FrameSeparatorChar}, 2)
				if len(data) != 2 || data[0] == nil {
					p2p.udpServer.Logger.Log(3, "InvalID relay data.")
					p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, frame.ID, []byte("InvalID relay data.")))
					return
				}
				target := p2p.IDsToConns.Get(string(data[0]))
				if target == nil {
					p2p.udpServer.Logger.Log(3, "InvalID target ID: "+string(data[0]))
					p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, data[0], []byte("InvalID target ID.")))
					return
				}

				//Check first type
				mapValue, ok := p2p.punchingConnsUDP.GetHas(string(frame.ID))
				if ok {
					connSettings := mapValue.Get(string(data[0]))
					if connSettings.C {
						//Relay allowed
						success = true
					}
				}

				//Check second value
				mapValue, ok = p2p.punchingConnsUDP.GetHas(string(data[0]))
				if ok {
					connSettings := mapValue.Get(string(frame.ID))
					if connSettings.C {
						//Relay allowed
						success = true
					}
				}

				//Resolve
				if success {
					target.Send(webtools.PackWebtoolsFrame(P2P_CMD_RELAY, frame.ID, data[1]))
				} else {
					p2p.udpServer.Logger.Log(3, "No connection found for ID pair: "+string(frame.ID)+" and: "+string(data[0]))
					p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, data[0], []byte("No connection found.")))
				}
				break
			}
		}
	}
}

func (p2p *P2PCoordinator) handleConnectStatus(sourceID string, targetID string, gotInverted bool) bool {
	//Check if peers are punching
	if !p2p.punchingConnsUDP.Has(sourceID) {
		//No ID on server
		return false
	}
	sourceMap := p2p.punchingConnsUDP.Get(sourceID)
	if !sourceMap.Has(targetID) {
		//No ID on server
		return false
	}
	targetMap := sourceMap.Get(targetID)

	//Set values
	if gotInverted {
		targetMap.B = true
	} else {
		targetMap.A = true
	}
	sourceMap.Set(targetID, targetMap)
	p2p.punchingConnsUDP.Set(sourceID, sourceMap)
	return true
}

/*
Starts P2P coordinator server
*/
func (p2p *P2PCoordinator) Start() {
	go p2p.tcpServer.Start()
	p2p.udpServer.Start()
}

/*
Stops P2P coordinator server
*/
func (p2p *P2PCoordinator) Stop() {
	p2p.tcpServer.Stop()
	p2p.udpServer.Stop()
}
