package p2ptools

import (
	"crypto/rsa"
	"strconv"
	"time"
	"webtools"
	"webtools/encryption"
	httptools "webtools/httpTools"
)

// Anytime client disconnects, it indicates some error (invalid values or connection drop)
const (
	/*
		Structure to client: cmd;svPublicKey;svSignature
		Structure to server - Encrypted: cmd;svPublicKey;clSignature
		Used for initial verification of connection and for sharing public signatures
	*/
	P2P_CMD_VERIFY_SIGNATURES = "verifySignatures"

	/*
		Structure to client: cmd
		Structure to server: cmd;hasServer;allowHolePunchingServer
		Used for initial verification of connection and for sharing public signatures
	*/
	P2P_CMD_GET_PEER_INFO = "getPeerInfo"
	/*
		Structure to client: cmd;hasServer;allowHolePunchingServer;port;allowP2P
		Used after all inforamtion from P2P_CMD_GET_PEER_INFO is supplied corectly
	*/
	P2P_CMD_AWAIT_CONNECTION = "awaitConnection"
	/*
		Sturcure to client: cmd;peerID
	*/
	P2P_CMD_SET_PEER_ID = "setPeerId"
	/*
		Structure to server: cmd;targetPeerId
		Used for requesting connection to other peer
	*/
	P2P_CMD_CONNECT_TO_PEER = "connectToPeer"
	/*
		Structure to client:cmd;time;address;port
		Used to start punch holing
	*/
	P2P_CMD_START_PUNCH_HOLE = "startPunchHole"
	/*
		Structure to client: cmd
		Used for requesting connection to RELAY and for client to request connection to RELAY network
	*/
	P2P_CMD_CONNECT_TO_RELAY = "connectToRelay"
	/*
		Structure to client: cmd
		Structure to server: cmd;peerId
	*/
	RELAY_CMD_GET_RELAY_INFO = "getRelayInfo"
)

/*
Packs P2P Message to standardized frame with signature and encryption
*/
func PackP2PMessage(enc *encryption.AsymmetricEncryption, logger *webtools.ConsoleLogger, destinationPublicKey *rsa.PublicKey, command string, params map[string]string) []byte {
	//Create standard message
	data := httptools.CreateURLFromParameters(command, params)

	//Sign data
	signed, err := enc.SignToJson([]byte(data))
	if err != nil {
		logger.Log(3, "Error signing message: "+err.Error())
		return nil
	}

	//Encrypt data
	encrypted, err := enc.Encrypt(signed, destinationPublicKey)
	if err != nil {
		logger.Log(3, "Error encrypting message: "+err.Error())
		return nil
	}
	return encrypted
}

/*
Unpacks P2P Message to command and parameters of command with validating signature and decrypting
Returns: Command, Parameters, hadError
*/
func UnpackP2PMessage(enc *encryption.AsymmetricEncryption, logger *webtools.ConsoleLogger, sourcePublicKey *rsa.PublicKey, data []byte) (string, map[string]string, bool) {
	// Decrypt
	decrypt, err := enc.Decrypt(data)
	if err != nil {
		logger.Log(3, "Error decrypting message: "+err.Error())
		return "", nil, true
	}

	//Verify data
	verify, err := enc.VerifyFromJson(decrypt, sourcePublicKey)
	if err != nil {
		logger.Log(3, "Error verifing signature: "+err.Error())
		return "", nil, true
	}

	//Unpack standard
	command, params := httptools.CreateParametersFromURL(string(verify))
	return command, params, false
}

type P2PServerConnection struct {
	address                 string
	port                    string
	id                      string
	conn                    *httptools.WebSocketServerConn
	hasServer               bool
	allowHolePunchingServer bool
	publicKey               *rsa.PublicKey
	origin                  *P2PHTTPCoordinatorServer
	allowP2P                bool
	relayConn               *httptools.WebSocketServerConn
}

/*
* Closes connection
 */
func (p2p *P2PServerConnection) Close() {
	p2p.origin.peersByClient.Delete(p2p.conn)
	p2p.conn.Close()
}

/*
Encrypts, signs and sends message to client
*/
func (p2p *P2PServerConnection) SendMessage(command string, params map[string]string) {
	p2p.conn.Send(PackP2PMessage(p2p.origin.asymmetricEncryption, p2p.origin.httpServer.Logger, p2p.publicKey, command, params))
}

type P2PServerPeersConnection struct {
	id     string
	source *P2PServerConnection
	target *P2PServerConnection
	// 0 = No connection, 1 = From source ok, 2 = From target ok, 3 = Final source ok, 4 = Established, should be removed
	phase uint8
}

type P2PHTTPCoordinatorServer struct {
	httpServer           *httptools.WebSocketServer
	peersByClient        webtools.SafeMap[*httptools.WebSocketServerConn, *P2PServerConnection]
	peersById            webtools.SafeMap[string, *P2PServerConnection]
	relaysById           webtools.SafeMap[string, *RelayServerConnection]
	relaysByClient       webtools.SafeMap[*httptools.WebSocketServerConn, *RelayServerConnection]
	serverId             string
	peersConnections     webtools.SafeMap[string, *P2PServerPeersConnection]
	asymmetricEncryption *encryption.AsymmetricEncryption
	allowRelay           bool
	allowP2P             bool
}

func (sv *P2PHTTPCoordinatorServer) CloseConnection(conn *P2PServerPeersConnection) {
	conn.source.conn.Send([]byte(P2P_CMD_CONNECT_PEER_CANCEL))
	conn.target.conn.Send([]byte(P2P_CMD_CONNECT_PEER_CANCEL))
}

/*
Creates new P2p HTTP Coordinator Server that is running on top of websocketServer. Leave p2pUrl for default /p2p url.
*/
func NewP2PHTTPCoordinatorServer(websocketServer *httptools.WebSocketServer, allowP2P bool, p2pUrl string, allowRelay bool, relayUrl string) *P2PHTTPCoordinatorServer {
	// Default URL when empty
	if p2pUrl == "" {
		p2pUrl = "/p2p"
	}
	if relayUrl == "" {
		relayUrl = "/relay"
	}

	//Check if addresses match
	if p2pUrl == relayUrl {
		websocketServer.Logger.Log(3, "Can not have same P2P and relay address")
		return nil
	}

	// Create P2P server
	p2p := &P2PHTTPCoordinatorServer{httpServer: websocketServer, peersByClient: webtools.MakeSafeMap[*httptools.WebSocketServerConn, *P2PServerConnection](), serverId: webtools.GenerateRandomId(), allowRelay: allowRelay, allowP2P: allowP2P, peersById: webtools.MakeSafeMap[string, *P2PServerConnection](), relaysById: webtools.MakeSafeMap[string, *RelayServerConnection](), relaysByClient: webtools.MakeSafeMap[*httptools.WebSocketServerConn, *RelayServerConnection]()}

	// Add P2P to HTTP websocketServer
	websocketServer.AddWebSocketURL(p2pUrl, p2p.readFunc)
	websocketServer.AddWebSocketURL(relayUrl, p2p.readFuncRelay)
	return p2p
}

func (sv *P2PHTTPCoordinatorServer) readFunc(conn *httptools.WebSocketServerConn, data []byte, status uint8, isBinary bool) {
	if sv.peersByClient.Get(conn) == nil {
		//No connection found, create new
		sv.peersByClient.Set(conn, &P2PServerConnection{conn: conn})
	}
	sv.readFuncLocal(sv.peersByClient.Get(conn), data, status, isBinary)
}

func (sv *P2PHTTPCoordinatorServer) readFuncLocal(conn *P2PServerConnection, data []byte, status uint8, isBinary bool) {
	if status == webtools.TCP_FINISHED_READ_FUNC_STATUS {
		return
	}
	if status == webtools.TCP_DISCONNECT_STATUS {
		// TODO:HANDLE TCP_DISCONNECT_STATUS
	}
	if status == webtools.TCP_CONNECT_STATUS {
		//Handle connect
		// Only send data Server -> Client
		publicKeyData, err := sv.asymmetricEncryption.EncodePublicKey()
		if err != nil {
			sv.httpServer.Logger.Log(3, "Could not encode public key: "+err.Error())
			conn.Close()
			return
		}

		//Sign
		signedJsonData, err := sv.asymmetricEncryption.SignToJson(publicKeyData)
		if err != nil {
			sv.httpServer.Logger.Log(3, "Could not sing public key: "+err.Error())
			conn.Close()
			return
		}

		//Send
		conn.conn.Send(signedJsonData)
		return
	}

	//Regular read
	if !isBinary {

		//Sort error command
		command, params, hadError := UnpackP2PMessage(sv.asymmetricEncryption, sv.httpServer.Logger, conn.publicKey, data)
		if hadError {
			return
		}
		if params["error"] != "" {
			//Got error from client
			sv.httpServer.Logger.Log(3, "Client returned error: "+params["error"])
			conn.Close()
			return
		}

		//Sort commands
		switch command {
		case P2P_CMD_VERIFY_SIGNATURES:
			{
				//Check returned public key
				verifyPubKey, err := encryption.ParsePublicKey([]byte(params["publicKey"]))
				if err != nil {
					sv.httpServer.Logger.Log(3, "Invalid returned public key: "+err.Error())
					conn.Close()
					return
				}

				//Validate
				if sv.asymmetricEncryption.GetPublicKey().Equal(verifyPubKey) {
					//Store public key of client
					conn.publicKey, err = encryption.ParsePublicKey([]byte(params["publicKeyClient"]))
					if err != nil {
						sv.httpServer.Logger.Log(3, "Could not get public key of client: "+err.Error())
						conn.Close()
						return
					}

					//Request peer info
					conn.SendMessage(P2P_CMD_GET_PEER_INFO, nil)
					return
				} else {
					sv.httpServer.Logger.Log(3, "Invalid returned public key: "+err.Error())
					conn.Close()
					return
				}
			}
		case P2P_CMD_GET_PEER_INFO:
			{
				//Got peer info
				if params["port"] == "" {
					//No port specified
					sv.httpServer.Logger.Log(3, "No port specified in peer info.")
					conn.Close()
					return
				}
				conn.port = params["port"]

				//Get hasServer property
				if params["hasServer"] == "" {
					sv.httpServer.Logger.Log(3, "No hasServer property specified in peer info.")
					conn.Close()
					return
				}

				//Convert to bool
				var err error
				conn.hasServer, err = strconv.ParseBool(params["hasServer"])
				if err != nil {
					sv.httpServer.Logger.Log(3, "Error parsing hasServer property: "+err.Error())
					conn.Close()
					return
				}

				//Get allowHolePunchingServer property
				if params["allowHolePunchingServer"] == "" {
					sv.httpServer.Logger.Log(3, "No allowHolePunchingServer property specified in peer info.")
					conn.Close()
					return
				}

				//Convert to bool
				conn.allowHolePunchingServer, err = strconv.ParseBool(params["allowHolePunchingServer"])
				if err != nil {
					sv.httpServer.Logger.Log(3, "Error parsing allowHolePunchingServer property: "+err.Error())
					conn.Close()
					return
				}

				//Get allowP2P property
				if params["allowP2P"] == "" {
					sv.httpServer.Logger.Log(3, "No allowP2P property specified in peer info.")
					conn.Close()
					return
				}

				//Convert to bool
				conn.allowP2P, err = strconv.ParseBool(params["allowP2P"])
				if err != nil {
					sv.httpServer.Logger.Log(3, "Error parsing allowP2P property: "+err.Error())
					conn.Close()
					return
				}

				//Generate peer ID
				id := webtools.GenerateRandomId()
				id = "p2p-" + id
				conn.id = id

				//All ok, await for more requests
				conn.SendMessage(P2P_CMD_SET_PEER_ID, map[string]string{"id": id})
				time.Sleep(1 * time.Second)
				conn.SendMessage(P2P_CMD_AWAIT_CONNECTION, nil)
				return
			}
		case P2P_CMD_CONNECT_TO_PEER:
			{
				//Find peer
				targetConn := sv.peersById.Get(params["targetPeerId"])
				if targetConn == nil {
					//No peer found
					sv.httpServer.Logger.Log(3, "Target peer with id: "+params["targetPeerId"]+" not found.")
					conn.SendMessage(P2P_CMD_CONNECT_TO_PEER, map[string]string{"error": "peer not found"})
					return
				}

				//Check if peer has server
				if !targetConn.hasServer {
					//No server at target
					sv.httpServer.Logger.Log(3, "Target peer with id: "+targetConn.id+" does not provide any server.")
					conn.SendMessage(P2P_CMD_CONNECT_TO_PEER, map[string]string{"error": "no target server"})
					return
				}

				//Chech if both peers allow P2P connection
				if !conn.allowP2P || !targetConn.allowP2P || !sv.allowP2P {
					//No P2P allowed, use RELAY if possible
					sv.httpServer.Logger.Log(2, "One of peers does not allow P2P connection, switching to RELAY...")
					//conn.SendMessage(P2P_CMD_CONNECT_TO_PEER, map[string]string{"warning": "switching to RELAY"})
					conn.SendMessage(P2P_CMD_CONNECT_TO_RELAY, nil)
					return
				}

				//Check if one of peer have punch hole server
				if !targetConn.allowHolePunchingServer && !conn.allowHolePunchingServer {
					//No punch hole server
					sv.httpServer.Logger.Log(3, "Source peer: "+conn.id+" and target peer: "+targetConn.id+" do not have punch hole server.")
					conn.SendMessage(P2P_CMD_CONNECT_TO_PEER, map[string]string{"error": "no punch hole server"})
					return
				}

				//All ok, start punch holing
				t := time.Now().Add(5 * time.Second)
				if conn.allowHolePunchingServer {
					targetConn.SendMessage(P2P_CMD_START_PUNCH_HOLE, map[string]string{"time": t.GoString(), "address": conn.address, "port": conn.port})
				}
				if targetConn.allowHolePunchingServer {
					conn.SendMessage(P2P_CMD_START_PUNCH_HOLE, map[string]string{"time": t.GoString(), "address": targetConn.address, "port": targetConn.port})
				}
			}
		}
	} else {
		sv.httpServer.Logger.Log(2, "P2P sent data as binary, ignoring.")
	}
}
