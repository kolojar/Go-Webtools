package webrtc

import (
	"encoding/hex"
	"fmt"
	"webtools/udp"
)

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-6
Specification: https://datatracker.ietf.org/doc/html/rfc8445#section-1
*/
type STUNServer struct {
	udpServer *udp.Server
}

/*
NewSTUNServer creates new STUN Server on UDP but does not starts it
*/
func NewSTUNServer(address string, reportTraffic bool) (*STUNServer, error) {
	stunServer := &STUNServer{}
	var err error
	stunServer.udpServer, err = udp.NewServer(address, stunServer.readFunc, reportTraffic)
	if err != nil {
		return nil, err
	}
	stunServer.udpServer.Logger.Preprefix = "STUNServer"
	return stunServer, err
}

func (stunServer *STUNServer) readFunc(conn *udp.ServerConn, data []byte, ended bool) {
	if ended {
		return
	}
	//Unpack STUN packet
	messageType, messageClass, transactionID, attributes, isSTUNPacket, err := UnpackSTUNPacket(data, conn.Address.IP.To4() != nil)
	if !isSTUNPacket {
		stunServer.udpServer.Logger.Log(2, "Not a STUN packet: "+hex.EncodeToString(data))
		return
	}
	if err != nil {
		stunServer.udpServer.Logger.Log(3, "Error unpacking STUN packet: "+err.Error())
		return
	}

	//Process STUN packet
	fmt.Println(messageType, messageClass, transactionID, attributes)
	if messageType == MessageTypeSTUNBinding && messageClass == MessageClassSTUNRequest {
		//Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-7.3.1.1
		//Binding request
		addressPort := conn.Address.AddrPort()
		xorMappedAttribute := STUNPacketDecodedAttribute{Type: STUNPacketAttributeTypeXORMappedAddress, Data: make(map[string]any)}

		//Get IP family
		if addressPort.Addr().Is4() {
			xorMappedAttribute.Data["family"] = "IPv4"
		} else if addressPort.Addr().Is6() {
			xorMappedAttribute.Data["family"] = "IPv6"
		} else {
			stunServer.udpServer.Logger.Log(3, "Error packing Binding request - invalid IP family")
			return
		}

		//Get port
		xorMappedAttribute.Data["port"] = addressPort.Port()

		//Get IP string
		xorMappedAttribute.Data["address"] = addressPort.Addr().String()
		xorMappedAttribute.Print()

		//Pack and send
		_, packet, err := PackSTUNPacket(MessageTypeSTUNBinding, MessageClassSTUNSuccessResponse, []STUNPacketDecodedAttribute{xorMappedAttribute})
		if err != nil {
			stunServer.udpServer.Logger.Log(3, "Error packing Binding request: "+err.Error())
			return
		}
		stunServer.udpServer.Logger.Log(1, "Sending Binding request for: "+conn.Address.String())
		conn.Send(packet)
	}
}

/*
Start starts STUN Server, locks execution thread
*/
func (stunServer *STUNServer) Start() {
	stunServer.udpServer.Start()
}

/*
Stop stops STUN server
*/
func (stunServer *STUNServer) Stop() {
	stunServer.udpServer.Stop()
}
