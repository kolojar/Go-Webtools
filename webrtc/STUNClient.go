package webrtc

import (
	"fmt"
	"net"
	"strconv"
	"time"
	"webtools"
	"webtools/udp"
)

type STUNClient struct {
	client      *udp.Client
	rtt         uint32 //Base time that si waited before resending
	rc          uint8  //Resend count
	sentPackets webtools.SafeMap[string, bool]
	isIPv4      bool
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc5389#section-6
*/
func NewSTUNClient(targetIP string, isIPv4 bool, reportTraffic bool) (*STUNClient, error) {
	//Create new STUN client
	stunClient := &STUNClient{
		rtt:         2000, //Recommended: 500
		sentPackets: webtools.MakeSafeMap[string, bool](),
		isIPv4:      isIPv4,
	}

	//Create UDP client (STUN uses UDP)
	var err error
	stunClient.client, err = udp.NewClient(targetIP, stunClient.readFunc, reportTraffic)
	if err != nil {
		return nil, err
	}

	//Get type of IP
	//addr, err := netip.ParseAddrPort(stunClient.client.Conn.RemoteAddr().String())
	//if err != nil {
	//	return nil, err
	//}
	//stunClient.isIPv4 = addr.Addr().Is4()
	return stunClient, nil

}

/*
Send packs packet and sends it to server
*/
func (stunClient *STUNClient) Send(messageType MessageTypeSTUN, messageClass MessageClassSTUN, message []byte) {
	transactionID, packet, err := PackSTUNPacket(messageType, messageClass, message)
	if err != nil {
		stunClient.client.Logger.Log(3, "Error packing packet: "+err.Error())
		return
	}
	stunClient.sendFunc(packet, transactionID, stunClient.rtt, 0)
}

func (stunClient *STUNClient) sendFunc(packet []byte, transactionID string, timeToResponce uint32, resendCount uint8) {
	//Send to server
	stunClient.sentPackets.Set(transactionID, false)
	stunClient.client.Send(packet)

	//Wait for responce
	go func() {
		time.Sleep(time.Millisecond * time.Duration(timeToResponce))
		if stunClient.sentPackets.Has(transactionID) {
			//Failed to recieve responce
			if resendCount < stunClient.rc {
				stunClient.client.Logger.Log(2, "Failed to get responce for packet: "+transactionID+" at: "+stunClient.client.Conn.RemoteAddr().String()+". Resending: "+strconv.Itoa(int(resendCount)))
				stunClient.sendFunc(packet, transactionID, (timeToResponce-stunClient.rtt)*2+stunClient.rtt, resendCount+1)
			} else {
				stunClient.client.Logger.Log(3, "Failed to get responce for packet: "+transactionID+" at: "+stunClient.client.Conn.RemoteAddr().String())
				stunClient.sentPackets.Delete(transactionID)
			}
		}
	}()
}

func (stunClient *STUNClient) readFunc(_ *udp.Client, _ *net.UDPAddr, data []byte, ended bool) {
	//Check if ended
	if ended {
		return
	}

	//Unpack packet
	messageType, messageClass, transactionID, message, err := UnpackSTUNPacket(data, stunClient.isIPv4)
	if err != nil {
		stunClient.client.Logger.Log(3, "Error unpacking STUN packet: "+err.Error())
		return
	}

	//Mark as delivered
	stunClient.sentPackets.Delete(transactionID)
	fmt.Println(messageType, messageClass, transactionID, message[0])
	decode, err := message[0].DecodeSTUNPacketAttribute()
	fmt.Println("Decoded:")
	for k, v := range decode {
		fmt.Println(" -", k, v)
	}
}

/*
Connect connects to STUN server and start reading loop, does not locks execution thread
*/
func (stunClient *STUNClient) Connect() error {
	return stunClient.client.Connect()
}
