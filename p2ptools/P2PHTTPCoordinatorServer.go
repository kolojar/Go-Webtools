package p2ptools

import (
	"crypto/rsa"
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
		Structure to server: cmd;
		Used for initial verification of connection and for sharing public signatures
	*/
	P2P_CMD_GET_PEER_INFO = "getPeerInfo"
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
	address   string
	port      string
	id        string
	conn      *httptools.WebSocketServerConn
	hasServer bool
	publicKey *rsa.PublicKey
	origin    *P2PHTTPCoordinatorServer
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
	serverId             string
	peersConnections     webtools.SafeMap[string, *P2PServerPeersConnection]
	asymmetricEncryption *encryption.AsymmetricEncryption
}

func (sv *P2PHTTPCoordinatorServer) CloseConnection(conn *P2PServerPeersConnection) {
	conn.source.conn.Send([]byte(P2P_CMD_CONNECT_PEER_CANCEL))
	conn.target.conn.Send([]byte(P2P_CMD_CONNECT_PEER_CANCEL))
}

/*
Creates new P2p HTTP Coordinator Server that is running on top of websocketServer. Leave p2pUrl for default /p2p url.
*/
func NewP2PHTTPCoordinatorServer(websocketServer *httptools.WebSocketServer, p2pUrl string) *P2PHTTPCoordinatorServer {
	// Default URL when empty
	if p2pUrl == "" {
		p2pUrl = "/p2p"
	}

	// Create P2P server
	p2p := &P2PHTTPCoordinatorServer{httpServer: websocketServer, peersByClient: webtools.MakeSafeMap[*httptools.WebSocketServerConn, *P2PServerConnection](), serverId: webtools.GenerateRandomId()}

	// Add P2P to HTTP websocketServer
	websocketServer.AddWebSocketURL(p2pUrl, p2p.readFunc)
	return p2p
}

func (sv *P2PHTTPCoordinatorServer) readFunc(conn *httptools.WebSocketServerConn, data []byte, status uint8, isBinary bool) {
	if sv.peersByClient.Get(conn) == nil {
		//No connection found, create new
		sv.peersByClient.Set(conn, &P2PServerConnection{conn: conn})
	}
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
					//Request peer info
					conn.SendMessage()
				} else {
					sv.httpServer.Logger.Log(3, "Invalid returned public key: "+err.Error())
					conn.Close()
					return
				}
			}
		}
	} else {
		// Data mostly for RELAY
	}
}
