package webrtc

import (
	"net"
	"net/netip"
	"webtools"
	"webtools/udp"
)

type STUNClient struct {
	client      *udp.Client
	rtt         int
	sentPackets webtools.SafeMap[string, webtools.KeyValuePair[bool, []byte]]
	isIPv4      bool
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-6
*/
func NewSTUNClient(targetIP string, reportTraffic bool) (*STUNClient, error) {
	//Create new STUN client
	stunClient := &STUNClient{
		rtt:         2000, //Recommended: 500
		sentPackets: webtools.MakeSafeMap[string, webtools.KeyValuePair[bool, []byte]](),
	}

	//Get type of IP
	addr, err := netip.ParseAddr(targetIP)
	if err != nil {
		return nil, err
	}
	stunClient.isIPv4 = addr.Is4()

	//Create UDP client (STUN uses UDP)
	stunClient.client, err = udp.NewClient(targetIP, stunClient.readFunc, reportTraffic)
	return stunClient, err

}

func (stunClient *STUNClient) readFunc(_ *udp.Client, _ *net.UDPAddr, data []byte, ended bool) {
	//Check if ended
	if ended {
		return
	}

	//Unpack packet
	messageType, transactionID, message, err := UnpackSTUNPacket(data, stunClient.isIPv4)
	if err != nil {
		stunClient.client.Logger.Log(3, "Error unpacking STUN packet: "+err.Error())
		return
	}

	//Mark as delivered
	stunClient.sentPackets.Delete(transactionID)
}
