package p2ptools

import (
	"webtools"
	"webtools/encryption"
	httptools "webtools/httpTools"
	"webtools/universalHttpTcpTools"
)

type RelayServerConnection struct {
	peer   *P2PServerConnection
	conn   *httptools.WebSocketServerConn
	origin *P2PHTTPCoordinatorServer
}

/*
* Closes connection
 */
func (relay *RelayServerConnection) Close() {
	relay.origin.relaysByClient.Delete(relay.conn)
	relay.conn.Close()
	if relay.peer != nil {
		relay.peer.relayConn = nil
	}
}

/*
Encrypts, signs and sends message to client
*/
func (relay *RelayServerConnection) SendCommand(command string, params map[string]string) {
	relay.conn.Send(PackRelayMessage(true, []byte(httptools.CreateURLFromParameters(command, params))))
}

/*
Packs Relay message
*/
func PackRelayMessage(isCommand bool, data []byte) []byte {
	result := []byte{byte(webtools.FormatByBool(isCommand, '1', '0'))}
	result = append(result, data...)
	return result
}

/*
Unpacks Relay
*/
func UnpackRelayMessage(message []byte) (isCommand bool, data []byte) {
	isCommandByte := message[0]
	return string(isCommandByte) == "1", message[1:]
}

type RelayServerForP2P struct {
	universalServer      *universalHttpTcpTools.UniversalHttpTcpServer
	p2pServer            *P2PHTTPCoordinatorServer
	relaysByClient       webtools.SafeMap[*universalHttpTcpTools.UniversalHttpTcpServerConn, *RelayServerConnection]
	asymmetricEncryption *encryption.AsymmetricEncryption
}

func (sv *RelayServerForP2P) readFuncRelay(conn *universalHttpTcpTools.UniversalHttpTcpServerConn, data []byte, status uint8, isBinary bool) {
	if sv.relaysByClient.Get(conn) == nil {
		//No connection found, create new
		sv.relaysByClient.Set(conn, &RelayServerConnection{conn: conn})
	}
	sv.readFuncRelayLocal(sv.relaysByClient.Get(conn), data, status, isBinary)
}

/*
Function for handling relay requests
*/
func (sv *RelayServerForP2P) readFuncRelayLocal(conn *RelayServerConnection, data []byte, status uint8, isBinary bool) {
	//Sort status
	if status == webtools.TCP_CONNECT_STATUS {
		//Handle connect
		// Only send data Server -> Client
		publicKeyData, err := sv.asymmetricEncryption.EncodePublicKey()
		if err != nil {
			sv.universalServer.GetLogger().Log(3, "Could not encode public key: "+err.Error())
			conn.Close()
			return
		}

		//Sign
		signedJsonData, err := sv.asymmetricEncryption.SignToJson(publicKeyData)
		if err != nil {
			sv.universalServer.GetLogger().Log(3, "Could not sing public key: "+err.Error())
			conn.Close()
			return
		}

		//Send
		conn.conn.Send(signedJsonData)
		return
	}

	if !isBinary {
		//Command
		command, params, hadError := UnpackP2PMessage(sv.asymmetricEncryption, sv.httpServer.Logger, conn.publicKey, data)
		if hadError {
			return
		}

		//Sort error command
		if params["error"] != "" {
			//Got error from client
			sv.httpServer.Logger.Log(3, "Client returned error: "+params["error"])
			conn.Close()
			return
		}

		//Sort commands
		switch command {
		case RELAY_CMD_GET_RELAY_INFO:
			{
				if params["peerId"] == "" {
					//No peer id
					sv.httpServer.Logger.Log(3, "No peerId property specified.")
					conn.Close()
					return
				}

				//Get peer connection
				peerConn := sv.peersById.Get(params["peerId"])
				if peerConn == nil {
					//No peer found
					sv.httpServer.Logger.Log(3, "Invalid peerId specified.")
					conn.Close()
					return
				}

				//Associate peer to relay connections
				peerConn.relayConn = conn
			}

		}
	} else {
		//Data
	}
}
