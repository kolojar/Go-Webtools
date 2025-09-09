package p2ptools

import (
	"webtools"
	httptools "webtools/httpTools"
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

func (sv *P2PHTTPCoordinatorServer) readFuncRelay(conn *httptools.WebSocketServerConn, data []byte, status uint8, isBinary bool) {
	if sv.relaysByClient.Get(conn) == nil {
		//No connection found, create new
		sv.relaysByClient.Set(conn, &RelayServerConnection{conn: conn})
	}
	sv.readFuncRelayLocal(sv.relaysByClient.Get(conn), data, status, isBinary)
}

/*
Function for handling relay requests
*/
func (sv *P2PHTTPCoordinatorServer) readFuncRelayLocal(conn *RelayServerConnection, data []byte, status uint8, isBinary bool) {
	//No relay allowed
	if !sv.allowRelay {
		sv.httpServer.Logger.Log(3, "No relay allowed.")
		conn.Close()
		return
	}

	//Sort status
	if status == webtools.TCP_CONNECT_STATUS {
		//On connect
		conn.conn.IsBinary = true
		conn.SendCommand(RELAY_CMD_GET_RELAY_INFO, nil)
	}

	if !isBinary {
		//Command
		command, params, hadError := UnpackP2PMessage(sv.asymmetricEncryption, sv.httpServer.Logger, conn..publicKey, data)
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
