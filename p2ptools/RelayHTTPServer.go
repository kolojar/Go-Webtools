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
Unpacks Relay ,
*/
func UnpackRelayMessage(message []byte) (isCommand bool, data []byte) {
	isCommandByte := message[0]
	return string(isCommandByte) == "1", message[1:]
}
