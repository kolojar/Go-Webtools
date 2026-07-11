package udp

import "github.com/kolojar/Go-Webtools/helpertools"

type stableFrameType uint8

// stablePingFrame is internal ping frame code
const stablePingFrame stableFrameType = 1

// stablePongFrame is internal pong frame code
const stablePongFrame stableFrameType = 2

// stableDataRecievedFrame is frame code with information about success recieve of packet
const stableDataRecievedFrame stableFrameType = 3

// stableDataFrame is pure data frame code
const stableDataFrame stableFrameType = 4

// stableDataWithResendFrame is data frame code with checking for delivery
const stableDataWithResendFrame stableFrameType = 5

// stableDataWithOrderFrame is data frame code with checking for order of packets
const stableDataWithOrderFrame stableFrameType = 6

// stableDataWithResendOrderFrame is data frame code with checking for order of packets and for delivery
const stableDataWithResendOrderFrame stableFrameType = 7

// ConnectionStabilizerSettings are settings used in connectionStabilizer
type ConnectionStabilizerSettings struct {
	// KeepAliveIntervalSeconds sets how long it takes before keepAlive (ping packet) is send, set it to -1 to disable
	KeepAliveIntervalSeconds uint8

	// KeepAliveTriesBeforeError sets how many ping packets can be send without getting responce (no respoce must be right after each other)
	KeepAliveTriesBeforeError uint8

	// ResendDelayMiliseconds sets how long does it takes before resending the packet
	ResendDelayMiliseconds uint16

	//OrderDelayMiliseconds sets how long does stabilizer wait for other packets to arrive before ordering them and passing them to read function.
	//
	//Warning: This setting introduces latency.
	OrderDelayMiliseconds uint16

	//OrderDelayProcessDelayedPackets sets if delayed packets, that should be delivered before the OrderDelayMiliseconds, shoud be passed to read function.
	//
	//Warning: This setting can introduce data loss but can eliminate wrong order of packets.
	OrderDelayProcessDelayedPackets bool

	//WaitForAllPackets sets if stabilizer respects and waits like TCP for precise order of packets before passing them to read function.
	//
	//Difference between OrderDelayMiliseconds: WaitForAllPackets waits infinite time until required packet is recieved, OrderDelayMiliseconds some time and then continues.
	//
	// Warning: This setting can introduce latency or in worst scenario infinite waitng.
	//
	// Note: This setting overwrites OrderDelayMiliseconds and OrderDelayProcessDelayedPackets
	WaitForAllPackets bool

	// DefaultSendUseResend sets if Send() function should use resend
	DefaultSendUseResend bool

	// DefaultSendUseOrder sets if Send() functions should use preserve order
	DefaultSendUseOrder bool
}

// connectionStabilizer is internal struct for universal handeling of reads and writes from/to UDP
type connectionStabilizer struct {
	settings                ConnectionStabilizerSettings
	sendedPacketsACKsWindow helpertools.ReplayWindow[uint32]
	incomingPacketsWindow   helpertools.ReplayWindow[uint32]
	sendPacketNumber        uint32
}

// newConnectionStabilizer creates new connectionStabilizer
func newConnectionStabilizer(settings ConnectionStabilizerSettings) *connectionStabilizer {
	return &connectionStabilizer{
		settings:                settings,
		sendedPacketsACKsWindow: helpertools.MakeReplayWindow[uint32](),
	}
}

func processRead(framedData []byte) (frameType stableFrameType) {

}
