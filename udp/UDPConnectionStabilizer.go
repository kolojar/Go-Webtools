package udp

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

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

// stableDataWithOrderInstantFrame is data frame code with checking for order of packets, instantly drops out of order packets.
// Example: If packet 2 arrives before packet 1, it processes packet 2 and packet 1 gets dropped.
//
// For more stable see stableDataWithOrderTimeoutFrame
const stableDataWithOrderInstantFrame stableFrameType = 6

// stableDataWithOrderFrame is data frame code with checking for order of packets, drops out of order packets after timeout.
// Example: When packet 2 arrieves, it waits timeout (if there are packets before that have not arrived) before processing the packet 2 and dropping all older packets.
//
// For instant processing see: stableDataWithOrderInstantFrame.
//
// Note: Timeout is set via setting in connection stabilizer
const stableDataWithOrderTimeoutFrame stableFrameType = 7

// stableDataWithOrderResendFrame is data frame code with checking for order of packets and for delivery. It works like TCP.
//
// Warning: This type can introduce big latency or infinite waiting for packets, if they get lost. Packets are waiting for strict order.
// Example: If packet 2 arrives before packet 1, it waits until packet 1 is recieved
const stableDataWithOrderResendFrame stableFrameType = 8

// ConnectionStabilizerSettings are settings used in connectionStabilizer
type ConnectionStabilizerSettings struct {
	// KeepAliveIntervalSeconds sets how long it takes before keepAlive (ping packet) is send, set it to 0 to disable
	KeepAliveInterval time.Duration

	// KeepAliveTriesBeforeError sets how many ping packets can be send without getting responce (no respoce must be right after each other). Set to -1 to disable. This check is applied when sending ping packet.
	//
	// Warning: If KeepAliveIntervalSeconds is too small, it can error out without real reason
	KeepAliveTriesBeforeError int32

	// KeepAliveResendOnNoPong sets if KeepAlive packet should be resend after there is no Pong packet recieved (timeout is RTO)
	KeepAliveResendOnNoPong bool

	// ResendRetries sets how many times can be packet resended. Set to 0 for unlimited. Timeout for resending is RTO
	ResendRetries uint8

	// OrderDelayMiliseconds sets how long does stabilizer wait for other packets to arrive before ordering them and passing them to read function.
	//
	// Only affects: stableDataWithOrderTimeoutFrame.
	//
	// Warning: This setting introduces latency
	OrderDelay time.Duration

	// DefaultSendFrameType sets type for Send() function.
	//
	// Warning: Do not set 0, stablePingFrame, stablePongFrame or stableDataRecievedFrame = application will fallback to stableDataFrame
	DefaultSendFrameType stableFrameType

	// FallbackRTT is duration for RTT when IT cant be calculated yet
	//
	// Recommendation: Set this to 100 ms
	FallbackRTT time.Duration

	// MinimumRTT is minimum duration for RTT
	//
	// Recommendation: Set this to 10 ms
	MinimumRTT time.Duration

	// MinimumRTO is maximum duration for RTT
	//
	// Recommendation: Set this to 1500 ms
	MaximumRTT time.Duration

	// CountOfRecievedPacketsForACK sets after how many recieved packets is an ACK packet send when there was not any ACK-Compatible packet send. Set to 0 to disable.
	//
	// Recommendation: Set this to 16
	CountOfRecievedPacketsForACK uint32

	// TimeBetweenACKPackets is maximum duration between two ACK-Compatible packets. Also used as timeout for delayed ACK.
	//
	// Recommendation: Set this to 10 ms
	TimeBetweenACKPackets time.Duration
}

// connectionStabilizer is internal struct for universal handeling of reads and writes from/to UDP
type connectionStabilizer[T UniversalConn] struct {
	settings *ConnectionStabilizerSettings
	readFunc StabilizerReadFunc[T]
	conns    helpertools.SafeMap[T, *connectionStabilizerConn]
}

// newConnectionStabilizer creates new connectionStabilizer
func newConnectionStabilizer[T UniversalConn](settings *ConnectionStabilizerSettings, readFunc StabilizerReadFunc[T], isServer bool) *connectionStabilizer[T] {
	return &connectionStabilizer[T]{
		settings: settings,
		readFunc: readFunc,
		conns:    helpertools.MakeSafeMap[T, *connectionStabilizerConn](helpertools.FormatByBool(isServer, 0, 1)),
	}
}

type connectionStabilizerConn struct {
	// sendPacketResendNumber is sequence number for resend prevention and request
	sendPacketResendNumber atomic.Uint32
	// missingPingPackets is counter of missing and send packets
	missingPingPackets atomic.Uint32
	// countOfPacketsSinceLastACK counts recieved packets since last ACK-Compatible packet
	countOfPacketsSinceLastACK atomic.Uint32
	// lastACKTimestampUnixNano is timestamp of last send ACK-Compatible packet in UNIX Nano
	lastACKTimestampUnixNano atomic.Int64
	// sendPacketOrderNumberSimple is sequence number for ordering of packets (for types stableDataWithOrderInstantFrame and stableDataWithOrderTimeoutFrame)
	sendPacketOrderNumberSimple uint32
	// sendPacketOrderNumberPrecise is sequence number for ordering of packets (for type stableDataWithOrderResendFrame)
	sendPacketOrderNumberPrecise uint32

	// sendedPacketsACKsWindow is used for lookup if packets need to be resended
	sendedPacketsACKsWindow helpertools.ReplayWindow[uint32]
	// incomingPacketsWindow is used for checking if packet with sequence number already got recieved or not
	incomingPacketsWindow helpertools.ReplayWindow[uint32]
	// rttCalculator is rtt calculator for transmition timing
	rttCalculator helpertools.RTTCalculator
	sendPacketOrderSimpleMutex sync.Mutex
}

func newConnectionStabilizerConn(settings *ConnectionStabilizerSettings) *connectionStabilizerConn {
	conn := &connectionStabilizerConn{
		sendedPacketsACKsWindow: *helpertools.NewReplayWindow[uint32](),
		incomingPacketsWindow:   *helpertools.NewReplayWindow[uint32](),
		rttCalculator:           *helpertools.NewRTTCalculator(settings.MinimumRTT, settings.MaximumRTT, settings.FallbackRTT),
	}
	conn.sendPacketResendNumber.Store(0)
	conn.sendPacketOrderNumberSimple.Store(0)
	conn.sendPacketOrderNumberPrecise.Store(0)
	conn.missingPingPackets.Store(0)
	conn.countOfPacketsSinceLastACK.Store(0)
	conn.lastACKTimestampUnixNano.Store(time.Now().UnixNano())
	return conn
}

// processRead is internal function for handeling reads
func (stabilizer *connectionStabilizer[T]) processRead(conn T, framedData []byte) {
	//Check if has at least one byte
	if len(framedData) == 0 {
		return
	}

	//Get first byte (frame type)
	var frameType stableFrameType = stableFrameType(framedData[0])
	framedData = framedData[1:]

	//Read ACK settings
	if frameType != stableDataFrame && frameType != stableDataWithOrderInstantFrame && frameType != stableDataWithOrderTimeoutFrame {
		//Valid ACK-compatible packet
		framedData = framedData[stabilizer.conns.Get(conn).sendedPacketsACKsWindow.JoinWindowDataBytes(framedData):]
	}

	//Sort types
	if frameType == stablePingFrame {
		//Ping frame = reply with pong
		if len(framedData) != 8 {
			conn.GetLogger().Log(1, "Got ping from: "+conn.GetAddress().String())
			stabilizer.processWrite(conn, stablePongFrame, framedData)
		} else {
			conn.GetLogger().Log(4, "Got invalid ping from: "+conn.GetAddress().String())
		}
	} else if frameType == stablePongFrame {
		//Pong frame - check frame
		if len(framedData) != 8 {
			conn.GetLogger().Log(4, "Got invalid pong from: "+conn.GetAddress().String())
			return
		}

		//Calculate RTO- 12 byte Replay window
		sconn := stabilizer.conns.Get(conn)
		sconn.rttCalculator.CalculateRTT(time.Since(time.UnixMicro(int64(binary.LittleEndian.Uint64(framedData)))))
		if stabilizer.settings.KeepAliveTriesBeforeError != -1 {
			sconn.missingPingPackets.Store(0)
		}
	} else if frameType == stableDataFrame {
		//Data frame - no checking applied = pass to read func
		if stabilizer.readFunc != nil {
			stabilizer.readFunc(conn, framedData)
		}
	} else if frameType == stableDataWithResendFrame {
		//Data frame with resend function - apply window and send ACK
		sConn := stabilizer.conns.Get(conn)
		if sConn.sendedPacketsACKsWindow.ApplyWindowCheck(binary.LittleEndian.Uint32(framedData)) {
			//Pass to read func
			if stabilizer.readFunc != nil {
				stabilizer.readFunc(conn, framedData[4:])
			}
		}

		//Request ACK
		stabilizer.handleACKSend(conn, sConn, true)
	}
}

// handleACKSend handles sending ACK packets
func (stabilizer *connectionStabilizer[T]) handleACKSend(conn T, sConn *connectionStabilizerConn, loop bool) {
	//Check for count of packets
	if sConn.countOfPacketsSinceLastACK.Load() >= stabilizer.settings.CountOfRecievedPacketsForACK {
		stabilizer.processWrite(conn, stableDataRecievedFrame, nil)
		return
	}

	//Check for duration
	if time.Since(time.Unix(0, sConn.lastACKTimestampUnixNano.Load())) >= stabilizer.settings.TimeBetweenACKPackets {
		stabilizer.processWrite(conn, stableDataRecievedFrame, nil)
		return
	}

	//Wait some time for loop
	if loop {
		go func() {
			time.Sleep(stabilizer.settings.TimeBetweenACKPackets)
			stabilizer.handleACKSend(conn, sConn, false)
		}()
	}
}

// processWrite is internal function for handeling writes
func (stabilizer *connectionStabilizer[T]) processWrite(conn T, frameType stableFrameType, data []byte) {
	//Create new byte buffer
	buffer := new(bytes.Buffer)
	buffer.WriteByte(byte(frameType))
	var sConn *connectionStabilizerConn

	//Write ACK is possible
	if frameType != stableDataFrame && frameType != stableDataWithOrderInstantFrame && frameType != stableDataWithOrderTimeoutFrame {
		sConn = stabilizer.conns.Get(conn)
		sConn.countOfPacketsSinceLastACK.Store(0)
		sConn.lastACKTimestampUnixNano.Store(time.Now().UnixNano())
		buffer.Write(stabilizer.conns.Get(conn).incomingPacketsWindow.GetWindowDataBytes())
	}

	//Process specific types
	if frameType == stablePingFrame {
		//Ping frame
		if stabilizer.settings.KeepAliveTriesBeforeError == -1 || sConn.missingPingPackets.Load() < uint32(stabilizer.settings.KeepAliveTriesBeforeError) {
			//Can send ping packet
			conn.GetLogger().Log(1, "Sending ping to: "+conn.GetAddress().String())
			timestamp := make([]byte, 8)
			binary.LittleEndian.PutUint64(timestamp, uint64(time.Now().UnixMicro()))
			buffer.Write(timestamp)
			conn.Send(buffer.Bytes())
			if stabilizer.settings.KeepAliveTriesBeforeError != -1 {
				sConn.missingPingPackets.Add(1)
			}
		} else {
			//To much missing packets
			conn.GetLogger().Log(3, "Too much missing ping packets with: "+conn.GetAddress().String())
			conn.Close()
		}
		return
	} else if frameType == stablePongFrame {
		//Pong frame
		conn.GetLogger().Log(1, "Sending pong to: "+conn.GetAddress().String())
		buffer.Write(data)
		conn.Send(buffer.Bytes())
		return
	} else if frameType == stableDataRecievedFrame {
		//Data recieved frame - ACK frame
		conn.GetLogger().Log(1, "Sending ACK frame to: "+conn.GetAddress().String()+" for packet [hex]: "+hex.EncodeToString(data))
		conn.Send(buffer.Bytes())
		return
	} else if frameType == stableDataFrame {
		//Data frame - no checking applied
		conn.GetLogger().Log(1, "Sending pure data frame to: "+conn.GetAddress().String())
		buffer.Write(data)
		conn.Send(buffer.Bytes())
		return
	} else if frameType == stableDataWithResendFrame {
		//Data frame with resend function - add sequence number and data and pass to writer function
		sequenceNumber := sConn.sendPacketResendNumber.Add(1)-1
		buffer.Write(binary.LittleEndian.AppendUint32([]byte{}, sequenceNumber))
		buffer.Write(data)
		go stabilizer.resendWrite(conn, sConn, sequenceNumber, buffer.Bytes())
	} else if frameType == stableDataWithOrderInstantFrame || frameType == stableDataWithOrderTimeoutFrame {
		//Data frame with order function (instant / timeout) - add order number and data and send
		orderNumber := sConn.
	}

}

// resendWrite is helper function for resendable writing to connection until valid ACK is recieved or timeout is reached
func (stabilizer *connectionStabilizer[T]) resendWrite(conn T, sConn *connectionStabilizerConn, sequenceNumber uint32, data []byte) {
	for sendTry := uint8(0); stabilizer.settings.ResendRetries == 0 || sendTry <= stabilizer.settings.ResendRetries; sendTry++ {
		conn.GetLogger().Log(1, "Sending resend data frame to: "+conn.GetAddress().String()+" with try: "+strconv.FormatUint(uint64(sendTry), 10))
		conn.Send(data)
		time.Sleep(sConn.rttCalculator.GetRTO())
		if sConn.sendedPacketsACKsWindow.CheckValue(sequenceNumber) {
			return
		}
	}
	conn.GetLogger().Log(2, "Failed sending resend data frame to: "+conn.GetAddress().String())
}

/*
CleanupConnection removes specified connection
*/
func (stabilizer *connectionStabilizer[T]) CleanupConnection(conn T) {
	stabilizer.conns.Delete(conn)
}

/*
CleanupConnections removes all connections
*/
func (stabilizer *connectionStabilizer[T]) CleanupConnections() {
	stabilizer.conns.Clear()
}
