package p2p

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"time"
	"webtools"
	"webtools/tcp"
	"webtools/udp"
)

// Retry count for punching
const p2pPunchRetryCount = 50

// Used for sending punch, in ID there is origin ID and in data is targetID - server does not modifies frame, just sends it back
const p2pCMDPunch uint8 = 50

// Used for sending data, ID is sourceID, data is data
const p2pCMDData uint8 = 51

/*
ClientReadFunc is function definition for reading data from Client
*/
type ClientReadFunc func(client *Client, sourceID []byte, data []byte, ended bool, logger *webtools.ConsoleLogger)

/*
Client is P2P client for accessing WebTools P2P server
*/
type Client struct {
	//Coordinator
	upnpServiceManager        *UPnPServiceManager
	ClientCoordinator         *udp.Client
	tcpClientCoordinator      *tcp.ClientSimple
	ID                        []byte
	isConnecting              bool
	targetIDThisConnecting    []byte
	targetIDsConnectingStatus webtools.SafeMap[string, bool]
	reportTraffic             bool
	allowRelay                webtools.SafeMap[string, bool]
	readFunc                  ClientReadFunc
	port                      int
	loggerPrefix              string
	publicIP                  string
	coordinatorAddress        string

	//Server for incomming conns
	udpIncommingConnsSv *udp.Server
	udpIncommingConns   webtools.SafeMap[string, webtools.KeyValuePair[*udp.ServerConn, bool]]
	tcpIncommingConnsSv *tcp.Server
	tcpIncommingConns   webtools.SafeMap[string, webtools.KeyValuePair[*tcp.ServerConn, bool]]

	//Clients for outcomming connections
	udpFramer             *udp.Framer
	udpOutcommingConnsCls webtools.SafeMap[string, webtools.KeyValuePair[*udp.Client, bool]]
	tcpOutcommingConnsCls webtools.SafeMap[string, webtools.KeyValuePair[*tcp.ClientSimple, bool]]
}

/*
IsAlive gets if server is alive
*/
func (p2p *Client) IsAlive() bool {
	for _, val := range p2p.udpOutcommingConnsCls.GetValues() {
		if val.Key.IsAlive() {
			return true
		}
	}
	return p2p.udpIncommingConnsSv.IsAlive()
}

/*
GetPublicIP gets public IP address of this client
*/
func (p2p *Client) GetPublicIP() string {
	return p2p.publicIP
}

/*
SetLoggerPrefix sets logger prefix
*/
func (p2p *Client) SetLoggerPrefix(prefix string) {
	p2p.loggerPrefix = prefix
	p2p.ClientCoordinator.Logger.Preprefix = prefix
	p2p.udpIncommingConnsSv.Logger.Preprefix = prefix
	for _, val := range p2p.udpOutcommingConnsCls.GetValues() {
		val.Key.Logger.Preprefix = prefix
	}
}

/*
NewP2PClient creates new P2P Client for UDP but does not starts it
Setup UPnP using SetupUPnP()
*/
func NewP2PClient(coordinatorAddress string, portForIncommingConns int, readFunc ClientReadFunc, reportTraffic bool) (*Client, error) {
	//New P2P
	p2p := &Client{
		ID:                        nil,
		isConnecting:              false,
		reportTraffic:             reportTraffic,
		udpOutcommingConnsCls:     webtools.MakeSafeMap[string, webtools.KeyValuePair[*udp.Client, bool]](),
		udpIncommingConns:         webtools.MakeSafeMap[string, webtools.KeyValuePair[*udp.ServerConn, bool]](),
		targetIDsConnectingStatus: webtools.MakeSafeMap[string, bool](),
		allowRelay:                webtools.MakeSafeMap[string, bool](),
		readFunc:                  readFunc,
		port:                      portForIncommingConns,
		tcpIncommingConns:         webtools.MakeSafeMap[string, webtools.KeyValuePair[*tcp.ServerConn, bool]](),
		tcpOutcommingConnsCls:     webtools.MakeSafeMap[string, webtools.KeyValuePair[*tcp.ClientSimple, bool]](),
		coordinatorAddress:        coordinatorAddress,
	}

	//New client for Coordinator
	var err error
	p2p.ClientCoordinator, err = udp.NewClient(coordinatorAddress, p2p.readFuncCoordinator, reportTraffic)
	if err != nil {
		return nil, err
	}
	p2p.ClientCoordinator.Logger.Prefix = "P2PClientUDP - CoordinatorClient"
	p2p.ClientCoordinator.SetupFraming(udp.NewUDPFramerSimpleFromConfig(p2pFramerConfig, p2p.sendFailFunc))

	//Setup servers
	p2p.udpIncommingConnsSv, err = udp.NewServer("0.0.0.0:"+strconv.Itoa(portForIncommingConns), p2p.readFuncIncommingServerUDP, reportTraffic)
	if err != nil {
		return nil, err
	}
	p2p.udpIncommingConnsSv.Logger.Prefix = "P2PClientUDP - IncommingServerUDP"
	p2p.udpIncommingConnsSv.SetupFraming(p2p.udpFramer)
	p2p.tcpIncommingConnsSv, err = tcp.NewServer("0.0.0.0:"+strconv.Itoa(portForIncommingConns), p2p.readFuncIncommingServerTCP, reportTraffic, true)
	if err != nil {
		return nil, err
	}
	p2p.tcpIncommingConnsSv.Logger.Prefix = "P2PClientUDP - IncommingServerTCP"
	return p2p, nil
}

/*
SetupFraming setups UDP framer for P2P client
*/
func (p2p *Client) SetupFraming(framer *udp.Framer) {
	p2p.udpFramer = framer
	p2p.udpIncommingConnsSv.SetupFraming(framer)
	for _, val := range p2p.udpOutcommingConnsCls.GetValues() {
		val.Key.SetupFraming(framer)
	}
}

/*
SetupUPnP setups UPnP for P2P Client
*/
func (p2p *Client) SetupUPnP(upnp *UPnPServiceManager) error {
	if p2p.upnpServiceManager != nil {
		//Remove old
		err := p2p.upnpServiceManager.RemoveUPnPPort(p2p.port, "UDP")
		if err != nil {
			return err
		}
		err = p2p.upnpServiceManager.RemoveUPnPPort(p2p.port, "TCP")
		if err != nil {
			return err
		}
	}

	//Add UPnP
	if upnp != nil {
		err := upnp.AddUPnPPort(p2p.port, p2p.port, "UDP", "P2P UDP port")
		if err != nil {
			return err
		}
		err = upnp.AddUPnPPort(p2p.port, p2p.port, "TCP", "P2P TCP port")
		if err != nil {
			return err
		}
	}
	p2p.upnpServiceManager = upnp
	return nil
}

func (p2p *Client) sendFailFunc(_ *net.UDPAddr, data []byte, _ bool) {
	//Failed to send traffic to Coordinator using UDP, switch to TCP
	if p2p.tcpClientCoordinator == nil {
		var err error
		p2p.tcpClientCoordinator, err = tcp.NewClientSimple(p2p.coordinatorAddress, 0, false, p2p.readFuncCoordinatorTCP, p2p.reportTraffic)
		if err != nil {
			p2p.ClientCoordinator.Logger.Log(2, "Could not create TCP client: "+err.Error())
			return
		}
		p2p.tcpClientCoordinator.GetLogger().Prefix = "P2PClientTCP - CoordinatorClient"
		p2p.tcpClientCoordinator.Connect()
	}
	p2p.tcpClientCoordinator.Send(data)

}

func (p2p *Client) readFuncCoordinatorTCP(_ *tcp.ClientSimple, data []byte, status uint8) {
	p2p.readFuncCoordinator(nil, nil, data, status == webtools.DisconnectStatus)
}

func (p2p *Client) readFuncCoordinator(_ *udp.Client, _ *net.UDPAddr, data []byte, ended bool) {
	for _, frame := range webtools.UnpackWebtoolsFrame(data, p2p.ClientCoordinator.Logger) {
		switch frame.Operation {
		case p2pCMDNewID:
			{
				//New ID
				p2p.ID = frame.ID
				p2p.publicIP = string(frame.Data)
				p2p.ClientCoordinator.Logger.Log(2, "This client ID is: "+string(p2p.ID)+" and public IP: "+p2p.publicIP)

				//New client for Coordinator
				var err error
				p2p.tcpClientCoordinator, err = tcp.NewClientSimple(p2p.coordinatorAddress, 0, false, p2p.readFuncCoordinatorTCP, p2p.reportTraffic)
				if err != nil {
					p2p.ClientCoordinator.Logger.Log(2, "Could not create TCP client: "+err.Error())
					continue
				}
				p2p.tcpClientCoordinator.GetLogger().Prefix = "P2PClientTCP - CoordinatorClient"
				p2p.tcpClientCoordinator.Connect()
				p2p.tcpClientCoordinator.Send(webtools.PackWebtoolsFrame(p2pCMDAssocateTCP, p2p.ID, []byte("0")))
				continue
			}
		case p2pCMDStartPunching:
			{
				//Start punch holing
				if frame.ID == nil {
					p2p.ClientCoordinator.Logger.Log(3, "InvalID ID in frame.")
					return
				}
				if frame.Data == nil {
					p2p.ClientCoordinator.Logger.Log(3, "InvalID data in frame.")
					return
				}

				//Split data
				fmt.Println(frame.Data)
				split := bytes.SplitN(frame.Data, []byte{webtools.FrameSeparatorChar}, 2)
				fmt.Println(split)
				if len(split) != 2 {
					p2p.ClientCoordinator.Logger.Log(3, "InvalID split data in frame.")
					return
				}
				startTime := time.Unix(0, int64(binary.LittleEndian.Uint64(split[0])))

				//Create new clientUDP UDP
				clientUDP, err := udp.NewClient(string(split[1]), p2p.readFuncOutcommingClientsUDP, p2p.reportTraffic)
				if err != nil {
					p2p.ClientCoordinator.Logger.Log(3, "Error creating UDP client: "+err.Error())
					return
				}
				clientUDP.Logger.Preprefix = p2p.loggerPrefix
				clientUDP.Logger.Prefix = "P2PClientUDP - PeerClientUDP for ID: " + string(frame.ID)

				//p2p.clientIDsToConns.Set(args["targetID"], webtools.KeyValuePair[*udp.Client, bool]{Key: client, Value: false})
				//p2p.clientConnsToIDs.Set(client, args["targetID"])

				//Wait for time
				clientUDP.SetupFraming(p2p.udpFramer) //WARNING: NIL MAY BE NOT ENOUGHT, ADD FUNCTION TO SEND OVER TCP WHEN FAILS, OR USING P2P - COMPLETLY REWORK THIS PUNCHING NONSENCE
				p2p.udpOutcommingConnsCls.Set(string(frame.ID), webtools.KeyValuePair[*udp.Client, bool]{Key: clientUDP, Value: false})
				p2p.tcpOutcommingConnsCls.Set(string(frame.ID), webtools.KeyValuePair[*tcp.ClientSimple, bool]{Key: nil, Value: false})
				p2p.targetIDsConnectingStatus.Set(string(frame.ID), true)
				p2p.ClientCoordinator.Logger.Log(2, "Starting punching to: "+string(frame.ID)+" at: "+string(split[1]))
				time.Sleep(time.Until(startTime))

				//Start punching
				for i := 0; i < p2pPunchRetryCount; i++ {
					clientUDP.Logger.Log(1, "Connecting to target ID: "+string(frame.ID)+" attempt: "+strconv.Itoa(i+1)+"/"+strconv.Itoa(p2pPunchRetryCount))
					err = clientUDP.Connect()
					if err == nil {
						clientUDP.Send(webtools.PackWebtoolsFrame(p2pCMDPunch, p2p.ID, frame.ID))
					} else {
						clientUDP.Logger.Log(3, "Error connecting UDP to target IP: "+string(split[1])+" with error: "+err.Error())
					}
					if i >= 20 || (i < 20 && i%2 == 0) { //Eliminate some repeated connections for TCP
						go func() {
							if p2p.tcpOutcommingConnsCls.Get(string(frame.ID)).Key != nil {
								clientUDP.Logger.Log(0, "TCP already found.")
								return
							}

							clientUDP.Logger.Log(0, "Dialing TCP...")
							tcpConn, err := net.DialTimeout("tcp", string(split[1]), 1000*time.Millisecond)
							if err == nil {
								//Create new client TCP
								clientTCP := tcp.NewClientSimpleFromConnection(tcpConn.(*net.TCPConn), 0, false, p2p.readFuncOutcommingClientsTCP, p2p.reportTraffic)
								clientTCP.GetLogger().Preprefix = p2p.loggerPrefix
								clientTCP.GetLogger().Prefix = "P2PClientUDP - PeerClientTCP for ID: " + string(frame.ID)
								if clientTCP.Connect() {
									clientTCP.Send(webtools.PackWebtoolsFrame(p2pCMDPunch, p2p.ID, frame.ID))
									p2p.tcpOutcommingConnsCls.Set(string(frame.ID), webtools.KeyValuePair[*tcp.ClientSimple, bool]{Key: clientTCP, Value: false})
									p2p.ClientCoordinator.Send(webtools.PackWebtoolsFrame(p2pCMDConnectStatusTCP, p2p.ID, frame.ID))
								}

							} else {
								clientUDP.Logger.Log(3, "Error connecting TCP to target IP: "+string(split[1])+" with error: "+err.Error())
							}
						}()
					}
					time.Sleep(time.Duration(webtools.FormatByBool(i < 10, 5, webtools.FormatByBool(i < 20, 50, 100))) * time.Millisecond)
					if !p2p.targetIDsConnectingStatus.Get(string(frame.ID)) {
						//Connected to server
						clientUDP.Logger.Log(1, "Connected to other peer, waiting for coordinator.")
						break
					}
				}
				if p2p.targetIDsConnectingStatus.Get(string(frame.ID)) {
					//Not connected to server
					clientUDP.Logger.Log(3, "Could not connect to other peer.")
				}
				break
			}
		case p2pCMDConnectStatusUDP:
			{
				//Status about connection
				switch string(frame.Data) {
				case "server":
					{
						//Set true for server
						if !p2p.udpIncommingConns.Has(string(frame.ID)) {
							p2p.ClientCoordinator.Logger.Log(3, "Incomming connection not found: "+string(frame.ID))
							return
						}
						val := p2p.udpIncommingConns.Get(string(frame.ID))
						val.Value = true
						p2p.udpIncommingConns.Set(string(frame.ID), val)
						break
					}
				case "client":
					{
						//Set true for client
						if !p2p.udpOutcommingConnsCls.Has(string(frame.ID)) {
							p2p.ClientCoordinator.Logger.Log(3, "Outcomming connection not found: "+string(frame.ID))
							return
						}
						val := p2p.udpOutcommingConnsCls.Get(string(frame.ID))
						val.Value = true
						p2p.udpOutcommingConnsCls.Set(string(frame.ID), val)
						break
					}
				default:
					{
						p2p.ClientCoordinator.Logger.Log(3, "InvalID connection type: "+string(frame.Data))
						return
					}
				}
				break
			}
		case p2pCMDConnectStatusTCP:
			{
				//Status about connection
				switch string(frame.Data) {
				case "server":
					{
						//Set true for server
						if !p2p.tcpIncommingConns.Has(string(frame.ID)) {
							p2p.ClientCoordinator.Logger.Log(3, "Incomming connection not found: "+string(frame.ID))
							return
						}
						val := p2p.tcpIncommingConns.Get(string(frame.ID))
						val.Value = true
						p2p.tcpIncommingConns.Set(string(frame.ID), val)
						break
					}
				case "client":
					{
						//Set true for client
						if !p2p.tcpOutcommingConnsCls.Has(string(frame.ID)) {
							p2p.ClientCoordinator.Logger.Log(3, "Outcomming connection not found: "+string(frame.ID))
							return
						}
						val := p2p.tcpOutcommingConnsCls.Get(string(frame.ID))
						val.Value = true
						p2p.tcpOutcommingConnsCls.Set(string(frame.ID), val)
						break
					}
				default:
					{
						p2p.ClientCoordinator.Logger.Log(3, "InvalID connection type: "+string(frame.Data))
						return
					}
				}
				break
			}
		case p2pCMDConnectStatusDone:
			{
				//Punching done
				p2p.allowRelay.Set(string(frame.ID), string(frame.Data) == "true")
				if slices.Equal(p2p.targetIDThisConnecting, frame.ID) {
					p2p.isConnecting = false
				}
				p2p.targetIDsConnectingStatus.Delete(string(frame.ID))
				p2p.ClientCoordinator.Logger.Log(2, "Connecting done.")
				break
			}
		case p2pCMDCancelClient:
			{
				//Cancels current operation
				p2p.ClientCoordinator.Logger.Log(3, "Error from server: "+string(frame.Data))
				if p2p.targetIDsConnectingStatus.Has(string(frame.ID)) {
					p2p.targetIDsConnectingStatus.Set(string(frame.Data), false)
				}
				if p2p.udpIncommingConns.Has(string(frame.ID)) {
					val, ok := p2p.udpIncommingConns.GetHas(string(frame.ID))
					if ok {
						val.Value = false
						p2p.udpIncommingConns.Set(string(frame.ID), val)
					}
				}
				if p2p.udpOutcommingConnsCls.Has(string(frame.ID)) {
					val, ok := p2p.udpOutcommingConnsCls.GetHas(string(frame.ID))
					if ok {
						val.Value = false
						p2p.udpOutcommingConnsCls.Set(string(frame.ID), val)
					}
				}
				if p2p.allowRelay.Has(string(frame.ID)) {
					p2p.allowRelay.Set(string(frame.ID), false)
				}
				if bytes.Equal(p2p.targetIDThisConnecting, frame.ID) {
					p2p.isConnecting = false
				}
			}
		case p2pCMDRelay:
			{
				//Get relay data
				if p2p.readFunc != nil {
					p2p.readFunc(p2p, frame.ID, frame.Data, ended, p2p.ClientCoordinator.Logger)
				}
			}
		}
	}
}

func (p2p *Client) readFuncOutcommingClientsUDP(client *udp.Client, _ *net.UDPAddr, data []byte, ended bool) {
	for _, frame := range webtools.UnpackWebtoolsFrame(data, p2p.ClientCoordinator.Logger) {
		switch frame.Operation {
		case p2pCMDPunch:
			{
				//Got punch, process as OK
				p2p.targetIDsConnectingStatus.Set(string(frame.Data), false)
				p2p.udpOutcommingConnsCls.Set(string(frame.ID), webtools.KeyValuePair[*udp.Client, bool]{Key: client, Value: true})
				p2p.ClientCoordinator.Send(webtools.PackWebtoolsFrame(p2pCMDConnectStatusUDP, frame.ID, frame.Data))
				break
			}
		case p2pCMDData:
			{
				if p2p.readFunc != nil {
					p2p.readFunc(p2p, frame.ID, frame.Data, ended, client.Logger)
				}
				break
			}
		default:
			{
				client.Logger.Log(3, "InvalID command: "+strconv.FormatUint(uint64(frame.Operation), 10))
			}
		}
	}
}

func (p2p *Client) readFuncIncommingServerUDP(conn *udp.ServerConn, data []byte, ended bool) {
	for _, frame := range webtools.UnpackWebtoolsFrame(data, p2p.ClientCoordinator.Logger) {
		switch frame.Operation {
		case p2pCMDPunch:
			{
				//Got punch, just resend
				p2p.udpIncommingConnsSv.Logger.Log(2, "Got punch from: "+string(frame.ID)+" at IP: "+conn.Address.String())
				p2p.udpIncommingConns.Set(string(frame.ID), webtools.KeyValuePair[*udp.ServerConn, bool]{Key: conn, Value: false})
				conn.Send(data)
				break
			}
		case p2pCMDData:
			{
				if p2p.readFunc != nil {
					p2p.readFunc(p2p, frame.ID, frame.Data, ended, p2p.udpIncommingConnsSv.Logger)
				}
				break
			}
		default:
			{
				p2p.udpIncommingConnsSv.Logger.Log(3, "InvalID command from: "+string(frame.ID)+" | Command: "+strconv.FormatUint(uint64(frame.Operation), 10))
			}
		}

	}
}

func (p2p *Client) readFuncOutcommingClientsTCP(client *tcp.ClientSimple, data []byte, status uint8) {
	for _, frame := range webtools.UnpackWebtoolsFrame(data, p2p.ClientCoordinator.Logger) {
		if p2p.readFunc != nil {
			p2p.readFunc(p2p, frame.ID, frame.Data, status == webtools.DisconnectStatus, client.GetLogger())
		}
	}
}

func (p2p *Client) readFuncIncommingServerTCP(conn *tcp.ServerConn, data []byte, status uint8) {
	for _, frame := range webtools.UnpackWebtoolsFrame(data, p2p.ClientCoordinator.Logger) {
		switch frame.Operation {
		case p2pCMDPunch:
			{
				//Got punch, just resend
				p2p.tcpIncommingConnsSv.Logger.Log(2, "Got punch from: "+string(frame.ID)+" at IP: "+conn.GetConn().RemoteAddr().String())
				p2p.tcpIncommingConns.Set(string(frame.ID), webtools.KeyValuePair[*tcp.ServerConn, bool]{Key: conn, Value: true})
				p2p.ClientCoordinator.Send(webtools.PackWebtoolsFrame(p2pCMDConnectStatusTCP, frame.ID, frame.Data))
				break
			}
		case p2pCMDData:
			{
				if p2p.readFunc != nil {
					p2p.readFunc(p2p, frame.ID, frame.Data, status == webtools.DisconnectStatus, p2p.udpIncommingConnsSv.Logger)
				}
				break
			}
		default:
			{
				p2p.tcpIncommingConnsSv.Logger.Log(3, "InvalID command from: "+string(frame.ID)+" | Command: "+strconv.FormatUint(uint64(frame.Operation), 10))
			}
		}
	}
}

/*
ConnectToCoordinator connects to coordinator, does not lock execution thread
*/
func (p2p *Client) ConnectToCoordinator() bool {
	if p2p.ID != nil {
		return true
	}

	//Connect
	err := p2p.ClientCoordinator.Connect()
	if err != nil {
		p2p.ClientCoordinator.Logger.Log(3, "Error connecting to coordinator server: "+err.Error())
		return false
	}
	p2p.ClientCoordinator.Send(webtools.PackWebtoolsFrame(p2pCMDNewID, []byte{0}, []byte(strconv.Itoa(p2p.port))))
	go p2p.udpIncommingConnsSv.Start()
	go p2p.tcpIncommingConnsSv.Start()

	//Wait for ID
	for p2p.ID == nil {
		time.Sleep(100 * time.Millisecond)
	}
	return true
}

/*
ConnectToPeer connects to specified ID,does not lock execution thread
*/
func (p2p *Client) ConnectToPeer(targetID []byte) bool {
	for p2p.isConnecting {
		time.Sleep(100 * time.Millisecond)
	}

	//Send request to Coordinator
	p2p.isConnecting = true
	p2p.targetIDThisConnecting = targetID
	p2p.ClientCoordinator.Send(webtools.PackWebtoolsFrame(p2pCMDConnectToPeer, p2p.ID, targetID))

	//Wait for connect
	for p2p.isConnecting {
		time.Sleep(100 * time.Millisecond)
	}

	//Return status
	return p2p.udpIncommingConns.Get(string(targetID)).Value || p2p.udpOutcommingConnsCls.Get(string(targetID)).Value || p2p.allowRelay.Get(string(targetID))
}

/*
Send sends data to target peer, returns if value was send
*/
func (p2p *Client) Send(targetID []byte, data []byte) bool {
	//Handle outcommingUDP UDP
	outcommingUDP, ok := p2p.udpOutcommingConnsCls.GetHas(string(targetID))
	if ok && outcommingUDP.Value {
		//Send using client
		outcommingUDP.Key.Send(webtools.PackWebtoolsFrame(p2pCMDData, p2p.ID, data))
		return true
	}

	//Handle incommingUDP UDP
	incommingUDP, ok := p2p.udpIncommingConns.GetHas(string(targetID))
	if ok && incommingUDP.Value {
		//Send using server
		incommingUDP.Key.Send(webtools.PackWebtoolsFrame(p2pCMDData, p2p.ID, data))
		return true
	}

	//Handle outcomming TCP
	outcommingTCP, ok := p2p.tcpOutcommingConnsCls.GetHas(string(targetID))
	if ok && outcommingTCP.Value {
		//Send using client
		outcommingTCP.Key.Send(webtools.PackWebtoolsFrame(p2pCMDData, p2p.ID, data))
		return true
	}

	//Handle incomming TCP
	incommingTCP, ok := p2p.tcpIncommingConns.GetHas(string(targetID))
	if ok && incommingTCP.Value {
		//Send using server
		incommingTCP.Key.Send(webtools.PackWebtoolsFrame(p2pCMDData, p2p.ID, data))
		return true
	}

	//Handle relay
	relay, ok := p2p.allowRelay.GetHas(string(targetID))
	if ok && relay {
		//Send using relay
		frameBuilder := make([]byte, 0)
		frameBuilder = append(frameBuilder, targetID...)
		frameBuilder = append(frameBuilder, webtools.FrameSeparatorChar)
		frameBuilder = append(frameBuilder, data...)
		p2p.ClientCoordinator.Send(webtools.PackWebtoolsFrame(p2pCMDRelay, p2p.ID, frameBuilder))
		return true
	}

	p2p.ClientCoordinator.Logger.Log(3, "Failed to send message to peer ID: "+string(targetID))
	return false
}

/*
Stop stops P2P client
*/
func (p2p *Client) Stop() {
	if p2p.upnpServiceManager != nil {
		p2p.upnpServiceManager.Shutdown()
	}
	p2p.ClientCoordinator.Stop()
	for _, v := range p2p.udpIncommingConns.GetValues() {
		v.Key.Close()
	}
	p2p.udpIncommingConnsSv.Stop()
	for _, v := range p2p.tcpIncommingConns.GetValues() {
		v.Key.Close()
	}
	p2p.tcpIncommingConnsSv.Stop()
}

/*
CheckCGNAT checks if your connection is CG-NAT, you need to connect to coordinator first
*/
func (p2p *Client) CheckCGNAT() (bool, error) {
	if p2p.upnpServiceManager == nil {
		p2p.ClientCoordinator.Logger.Log(3, "No UPnP manager found.")
		return false, errors.New("no upnp manager active")
	}

	//Get UPnP public IP
	ips, err := p2p.upnpServiceManager.GetRouterPublicIP()
	if err != nil {
		return false, err
	}

	//Run check
	hasCGNAT := true
	for _, ip := range ips {
		if ip == p2p.publicIP {
			hasCGNAT = false
			break
		}
	}
	p2p.ClientCoordinator.Logger.Log(2, "CG-NAT status: "+strconv.FormatBool(hasCGNAT))
	return hasCGNAT, nil
}
