package udp

import (
	"encoding/hex"
	"net"

	"github.com/kolojar/Go-Webtools/helpertools"
)

// UniversalConn is an interface that unions ServerConn and Client
type UniversalConn interface {
	*ServerConn | *Client
	// Send sends data to target
	Send(data []byte)
	// Close closes connection with other side
	Close()
	// GetLogger gets logger of origin / owner
	GetLogger() *helpertools.ConsoleLogger
	// GetAddress gets remote address
	GetAddress() *net.UDPAddr
}

// StabilizerReadFunc is definition of function for reading
type StabilizerReadFunc[T UniversalConn] func(conn T, data []byte)

type stableFrameType uint8

// stablePingFrame is internal ping frame code
const stablePingFrame stableFrameType = 1

// stablePongFrame is internal pong frame code
const stablePongFrame stableFrameType = 2

// stableDataRecievedFrame is frame code with information about success recieve of packet (ACK)
const stableDataRecievedFrame stableFrameType = 3

// stableDataFrame is pure data frame code
const stableDataFrame stableFrameType = 4

// stableDataWithResendFrame is data frame code with checking for delivery
const stableDataWithResendFrame stableFrameType = 5

// stableDataWithOrderFrame is data frame code with checking for order of packets
const stableDataWithOrderFrame stableFrameType = 6

// stableDataWithResendOrderFrame is data frame code with checking for order of packets and for delivery
const stableDataWithResendOrderFrame stableFrameType = 7

// stableMissingPacketsFrame is information frame with list of packet numbers that need be resended
const stableMissingPacketsFrame stableFrameType = 8

// ConnectionStabilizerSettings are settings used in connectionStabilizer
type ConnectionStabilizerSettings struct {
	// KeepAliveIntervalSeconds sets how long it takes before keepAlive (ping packet) is send, set it to 0 to disable
	KeepAliveIntervalSeconds uint8

	// KeepAliveTriesBeforeError sets how many ping packets can be send without getting responce (no respoce must be right after each other). Set to -1 to disable
	KeepAliveTriesBeforeError int8

	// KeepAliveResendOnNoPong sets if KeepAlive packet should be resend after there is no Pong packet recieved (timeout is RTO)
	KeepAliveResendOnNoPong bool

	// ResendDelayMiliseconds sets how long does it takes before resending the packet
	ResendDelayMiliseconds uint16

	// OrderDelayMiliseconds sets how long does stabilizer wait for other packets to arrive before ordering them and passing them to read function.
	//
	// Warning: This setting introduces latency.
	OrderDelayMiliseconds uint16

	// OrderDelayProcessDelayedPackets sets if delayed packets, that should be delivered before the OrderDelayMiliseconds, shoud be passed to read function.
	//
	// Warning: This setting can introduce data loss but can eliminate wrong order of packets.
	OrderDelayProcessDelayedPackets bool

	// WaitForAllPackets sets if stabilizer respects and waits like TCP for precise order of packets before passing them to read function.
	//
	// Difference between OrderDelayMiliseconds: WaitForAllPackets waits infinite time until required packet is recieved, OrderDelayMiliseconds some time and then continues.
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
type connectionStabilizer[T UniversalConn] struct {
	settings                ConnectionStabilizerSettings
	sendedPacketsACKsWindow helpertools.ReplayWindow[uint32]
	incomingPacketsWindow   helpertools.ReplayWindow[uint32]
	sendPacketResendNumber  uint32
	sendPacketOrderNumber   uint32
	readFunc                StabilizerReadFunc[T]
}

// newConnectionStabilizer creates new connectionStabilizer
func newConnectionStabilizer[T UniversalConn](settings ConnectionStabilizerSettings, readFunc StabilizerReadFunc[T]) *connectionStabilizer[T] {
	return &connectionStabilizer[T]{
		settings:                settings,
		sendedPacketsACKsWindow: helpertools.MakeReplayWindow[uint32](),
		incomingPacketsWindow:   helpertools.MakeReplayWindow[uint32](),
		sendPacketResendNumber:  0,
		sendPacketOrderNumber:   0,
		readFunc:                readFunc,
	}
}

// processRead is internal function for handeling reads
func (stabilizer *connectionStabilizer[T]) processRead(conn T, framedData []byte) {
	//Check if has at least one byte
	if len(framedData) == 0 {
		return
	}

	//Get first byte (frame type)
	var frameType stableFrameType = stableFrameType(framedData[0])
	switch frameType {
	case stablePingFrame:
		{
			//Ping frame = reply with pong
			conn.GetLogger().Log(1, "Got ping from: "+conn.GetAddress().String())
			stabilizer.processWrite(conn, stablePongFrame, framedData[1:])
			break
		}
	case stableDataFrame:
		{
			//Data frame - no checking applied = pass to read func
			if stabilizer.readFunc != nil {
				stabilizer.readFunc(conn, framedData[1:])
			}
			break
		}
	}
}

// processWrite is internal function for handeling writes
func (stabilizer *connectionStabilizer[T]) processWrite(conn T, frameType stableFrameType, data []byte) {
	if frameType == stablePongFrame {
		//Pong frame
		conn.GetLogger().Log(1, "Sending pong to: "+conn.GetAddress().String())
		conn.Send(append(make([]byte, 0), byte(frameType)))
		return
	}
	if frameType == stableDataRecievedFrame {
		//Data recieved frame - ACK frame
		conn.GetLogger().Log(1, "Sending ACK frame to: "+conn.GetAddress().String()+" for packet [hex]: "+hex.EncodeToString(data))
		conn.Send(append(append(make([]byte, 0), byte(frameType)), data...))
		return
	}
	if frameType == stableDataFrame {
		//Data frame - no checking applied
		conn.GetLogger().Log(1, "Sending pure data frame to: "+conn.GetAddress().String())
		conn.Send(append(append(make([]byte, 0), byte(frameType)), data...))
		return
	}

}
