package p2pTools

//FIX NOT RUNNING TCP SERVER!

import (
	"bytes"
	"encoding/binary"
	"net"
	"strconv"
	"strings"
	"time"
	"webtools"
	"webtools/tcpTools"
	"webtools/udpTools"
)

// Standard framing
var P2P_FRAMER_CONFIG = &udpTools.UDPFramerConfig{IsOrganised: true, OrganisedTimeoutInMs: 50, TimeoutForResendInMs: 50, ResendMaxLimit: 5, UseKeepAlive: true}

// Wait time in seconds for P2P puncholing to start
const P2P_TIMEOUT_START = 5

// Request new id for client, port value in data -> Returns frame with id as id data as public IP
const P2P_CMD_NEW_ID uint8 = 1

// Requests for start of new connection between peers, in data there is targetId -> Starts connection setting -> Returns if connection was successfull in data: server/client/relay/error
const P2P_CMD_CONNECT_TO_PEER uint8 = 2

// Cancels current command on client with id if afected id, with message in data
const P2P_CMD_CANCEL_CLIENT uint8 = 3

// Informs clients to start punchholding - id is targetId, data it startTime;ipAddress
const P2P_CMD_START_PUNCHING uint8 = 4

// Informs server about success connection of UDP - is is sourceId, data is targetId -> Sends info about id and it connection status as data - server / client
const P2P_CMD_CONNECT_STATUS_UDP uint8 = 5

// Informs client about connection done, sends id and in data if relay is available
const P2P_CMD_CONNECT_DONE uint8 = 6

// Request of send relay - id is source id, data is targetId;data -> Sends to other peer in format: id - sourceId, data - data
const P2P_CMD_RELAY uint8 = 7

// Informs server about success connection of TCP - id is sourceId, data is targetId -> Sends info about id and it connection status as data - server / client
const P2P_CMD_CONNECT_STATUS_TCP uint8 = 8

// Requires server to associate this TCP connection to ID, requires id as sourceID
const P2P_CMD_ASSOCIATE_TCP uint8 = 9

type P2PCoordinatorConn struct {
	conn          *udpTools.UDPServerConn
	connTCP       *tcpTools.TCPServerConn
	id            []byte
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
	udpServer        *udpTools.UDPServer
	tcpServer        *tcpTools.TCPServer
	idsToConns       webtools.SafeMap[string, *P2PCoordinatorConn]
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
		idsToConns:       webtools.MakeSafeMap[string, *P2PCoordinatorConn](),
		punchingConnsUDP: webtools.MakeSafeMap[string, webtools.SafeMap[string, webtools.ThreeValuePair[bool, bool, bool]]](),
		punchingConnsTCP: webtools.MakeSafeMap[string, webtools.SafeMap[string, webtools.ThreeValuePair[bool, bool, bool]]](),
		AllowRelay:       allowRelay,
	}

	//New UDP server
	var err error
	p2p.udpServer, err = udpTools.NewUDPServer(address, p2p.readFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	p2p.udpServer.SetupFraming(udpTools.NewUDPFramerSimpleFromConfig(P2P_FRAMER_CONFIG, p2p.sendFailFunc))
	p2p.udpServer.Logger.Prefix = "P2PCoordinator - " + p2p.udpServer.Logger.Prefix

	//New TCP server
	p2p.tcpServer, err = tcpTools.NewTCPServer(address, p2p.readFuncTCP, reportTraffic, true)
	if err != nil {
		return nil, err
	}
	p2p.tcpServer.Logger.Prefix = "P2PCoordinator - " + p2p.udpServer.Logger.Prefix

	return p2p, nil
}

func (p2p *P2PCoordinator) sendFailFunc(address *net.UDPAddr, data []byte, _ bool) {
	//Failed to send traffic to client using UDP, switch to TCP
	for _, v := range p2p.idsToConns.GetValues() {
		if v.conn.Address == address {
			v.connTCP.Send(data)
			break
		}
	}

}

func (p2p *P2PCoordinator) generateNewID(portByte []byte, address string, sourceUDP *udpTools.UDPServerConn, sourceTCP *tcpTools.TCPServerConn) string {
	id := "p2pConn-" + webtools.GenerateRandomId()
	port, err := strconv.Atoi(string(portByte))
	if err != nil {
		p2p.udpServer.Logger.Log(3, "Invalid port number from: "+address)
		if sourceUDP != nil {
			sourceUDP.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, []byte("0"), []byte("Invalid port number")))
		} else {
			sourceTCP.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, []byte("0"), []byte("Invalid port number")))
		}
		return ""
	}
	p2p.idsToConns.Set(id, &P2PCoordinatorConn{conn: sourceUDP, connTCP: sourceTCP, id: []byte(id), port: port, sourceAddress: address})
	p2p.idsToConns.Get(id).Send(webtools.PackWebtoolsFrame(P2P_CMD_NEW_ID, []byte(id), []byte(address)))
	p2p.udpServer.Logger.Log(1, "Connection at: "+address+" has this new ID: "+id)
	return id
}

func (p2p *P2PCoordinator) readFuncTCP(conn *tcpTools.TCPServerConn, data []byte, status uint8) {
	for _, frame := range webtools.UnpackWebtoolsFrame(data, p2p.udpServer.Logger) {
		if frame.Operation == P2P_CMD_ASSOCIATE_TCP {
			//Associate with this conn
			connP2P := p2p.idsToConns.Get(string(frame.Id))
			connP2P.connTCP = conn
			p2p.idsToConns.Set(string(frame.Id), connP2P)
			continue
		}
		if frame.Operation == P2P_CMD_NEW_ID {
			//Generates new Id
			p2p.generateNewID(frame.Data, strings.Split(conn.GetConn().RemoteAddr().String(), ":")[0], nil, conn)
			continue
		}
		p2p.readFunc(nil, data, status == webtools.TCP_DISCONNECT_STATUS)
	}
}

func (p2p *P2PCoordinator) readFunc(conn *udpTools.UDPServerConn, data []byte, ended bool) {
	for _, frame := range webtools.UnpackWebtoolsFrame(data, p2p.udpServer.Logger) {
		if frame.Operation == P2P_CMD_NEW_ID {
			//Generates new Id
			p2p.generateNewID(frame.Data, conn.Address.IP.String(), conn, nil)
			continue
		}
		if string(frame.Id) == "" || string(frame.Id) == "0" || frame.Id == nil {
			p2p.udpServer.Logger.Log(3, "Invalid connection from: "+conn.Address.String())
			conn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, frame.Id, []byte("Invalid connection")))
			continue
		}

		//Sort commands
		p2pConn := p2p.idsToConns.Get(string(frame.Id))
		switch frame.Operation {
		case P2P_CMD_CONNECT_TO_PEER:
			{
				//Check if Id is on server
				target := p2p.idsToConns.Get(string(frame.Data))
				if target == nil {
					//No id on server
					p2p.udpServer.Logger.Log(3, "This ID is not on server: "+string(frame.Data))
					p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, frame.Data, []byte("Target ID is not on server")))
					continue
				}

				//Get start time
				var tStartBinary = make([]byte, 8)
				binary.LittleEndian.PutUint64(tStartBinary, uint64(time.Now().Add(time.Second*P2P_TIMEOUT_START).UnixNano()))

				//Make entry in punchingConns
				var mapValue webtools.SafeMap[string, webtools.ThreeValuePair[bool, bool, bool]]
				if p2p.punchingConnsUDP.Has(string(frame.Id)) {
					mapValue = p2p.punchingConnsUDP.Get(string(frame.Id))
				} else {
					mapValue = webtools.MakeSafeMap[string, webtools.ThreeValuePair[bool, bool, bool]]()
				}
				mapValue.Set(string(frame.Data), webtools.ThreeValuePair[bool, bool, bool]{A: false, B: false, C: false})
				p2p.punchingConnsUDP.Set(string(frame.Id), mapValue)
				p2p.punchingConnsTCP.Set(string(frame.Id), mapValue)
				//connId := webtools.GenerateRandomId()

				//Send to clients
				p2p.udpServer.Logger.Log(2, "Starting punching between: "+string(frame.Id)+" at: "+p2pConn.sourceAddress+" and: "+string(frame.Data)+" at: "+target.sourceAddress)
				p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_START_PUNCHING, frame.Data, append(append(tStartBinary, webtools.WEBTOOLS_FRAME_SEPARATOR), []byte(target.sourceAddress+":"+strconv.Itoa(target.port))...)))
				target.Send(webtools.PackWebtoolsFrame(P2P_CMD_START_PUNCHING, frame.Id, append(append(tStartBinary, webtools.WEBTOOLS_FRAME_SEPARATOR), []byte(p2pConn.sourceAddress+":"+strconv.Itoa(p2pConn.port))...)))

				//Wait for responce
				go func() {
					time.Sleep(2 * time.Second * P2P_TIMEOUT_START)

					//Send results UDP
					mapValue = p2p.punchingConnsUDP.Get(string(frame.Id))
					connSettings := mapValue.Get(string(frame.Data))
					if connSettings.A {
						p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_UDP, frame.Data, []byte("client")))
						target.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_UDP, frame.Id, []byte("server")))
						p2p.udpServer.Logger.Log(1, "Punching between: "+string(frame.Id)+" and: "+string(frame.Data)+" was successfull.")
					}
					if connSettings.B {
						p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_UDP, frame.Data, []byte("server")))
						target.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_UDP, frame.Id, []byte("client")))
						p2p.udpServer.Logger.Log(1, "Punching between: "+string(frame.Id)+" and: "+string(frame.Data)+" was successfull.")
					}
					connSettings.C = p2p.AllowRelay
					mapValue.Set(string(frame.Data), connSettings)
					p2p.punchingConnsUDP.Set(string(frame.Id), mapValue)

					//Send results TCP
					mapValue = p2p.punchingConnsTCP.Get(string(frame.Id))
					connSettings = mapValue.Get(string(frame.Data))
					if connSettings.A {
						p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_TCP, frame.Data, []byte("client")))
						target.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_TCP, frame.Id, []byte("server")))
						p2p.udpServer.Logger.Log(1, "Punching between: "+string(frame.Id)+" and: "+string(frame.Data)+" was successfull.")
					}
					if connSettings.B {
						p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_TCP, frame.Data, []byte("server")))
						target.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_STATUS_TCP, frame.Id, []byte("client")))
						p2p.udpServer.Logger.Log(1, "Punching between: "+string(frame.Id)+" and: "+string(frame.Data)+" was successfull.")
					}
					connSettings.C = p2p.AllowRelay
					mapValue.Set(string(frame.Data), connSettings)
					p2p.punchingConnsTCP.Set(string(frame.Id), mapValue)

					//Finish results
					time.Sleep(500 * time.Millisecond)
					p2p.udpServer.Logger.Log(2, "Punching between: "+string(frame.Id)+" and: "+string(frame.Data)+" is done.")
					p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_DONE, frame.Data, []byte(strconv.FormatBool(p2p.AllowRelay))))
					target.Send(webtools.PackWebtoolsFrame(P2P_CMD_CONNECT_DONE, frame.Id, []byte(strconv.FormatBool(p2p.AllowRelay))))

				}()
				break
			}
		case P2P_CMD_CONNECT_STATUS_UDP:
			{
				//Set connect status
				success := false
				if p2p.handleConnectStatus(string(frame.Id), string(frame.Data), false) {
					success = true
				}
				if p2p.handleConnectStatus(string(frame.Data), string(frame.Id), true) {
					success = true
				}

				//Handle if not success
				if !success {
					p2p.udpServer.Logger.Log(3, "This ID is not punching list: "+string(frame.Id)+" or this ID: "+string(frame.Data))
					p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, frame.Data, []byte("Source or Target ID is not on punching list")))
					continue
				}
				break
			}
		case P2P_CMD_RELAY:
			{
				//Handle relay
				var success bool = false
				data := bytes.SplitN(frame.Data, []byte{webtools.WEBTOOLS_FRAME_SEPARATOR}, 2)
				if len(data) != 2 || data[0] == nil {
					p2p.udpServer.Logger.Log(3, "Invalid relay data.")
					p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, frame.Id, []byte("Invalid relay data.")))
					return
				}
				target := p2p.idsToConns.Get(string(data[0]))
				if target == nil {
					p2p.udpServer.Logger.Log(3, "Invalid target id: "+string(data[0]))
					p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, data[0], []byte("Invalid target id.")))
					return
				}

				//Check first type
				mapValue, ok := p2p.punchingConnsUDP.GetHas(string(frame.Id))
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
					connSettings := mapValue.Get(string(frame.Id))
					if connSettings.C {
						//Relay allowed
						success = true
					}
				}

				//Resolve
				if success {
					target.Send(webtools.PackWebtoolsFrame(P2P_CMD_RELAY, frame.Id, data[1]))
				} else {
					p2p.udpServer.Logger.Log(3, "No connection found for id pair: "+string(frame.Id)+" and: "+string(data[0]))
					p2pConn.Send(webtools.PackWebtoolsFrame(P2P_CMD_CANCEL_CLIENT, data[0], []byte("No connection found.")))
				}
				break
			}
		}
	}
}

func (p2p *P2PCoordinator) handleConnectStatus(sourceId string, targetId string, gotInverted bool) bool {
	//Check if peers are punching
	if !p2p.punchingConnsUDP.Has(sourceId) {
		//No id on server
		return false
	}
	sourceMap := p2p.punchingConnsUDP.Get(sourceId)
	if !sourceMap.Has(targetId) {
		//No id on server
		return false
	}
	targetMap := sourceMap.Get(targetId)

	//Set values
	if gotInverted {
		targetMap.B = true
	} else {
		targetMap.A = true
	}
	sourceMap.Set(targetId, targetMap)
	p2p.punchingConnsUDP.Set(sourceId, sourceMap)
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
