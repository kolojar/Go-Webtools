package udp

import (
	"context"
	"encoding/binary"
	"fmt"
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
type StabilizerReadFunc[sequenceNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowWordType ~uint8 | ~uint16 | ~uint32 | ~uint64] func(conn *connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType], data []byte)

type stableFrameType uint8

// stablePingFrame is internal ping frame code
const stablePingFrame stableFrameType = 1

// stablePongFrame is internal pong frame code
const stablePongFrame stableFrameType = 2

// stableDataRecievedFrame is frame code with information about success recieve of packet (ACK)
const stableDataRecievedFrame stableFrameType = 3

// StableDataFrame is pure data frame code
const StableDataFrame stableFrameType = 4

// StableDataWithResendFrame is data frame code with checking for delivery
const StableDataWithResendFrame stableFrameType = 5

// StableDataWithOrderInstantFrame is data frame code with checking for order of packets, instantly drops out of order packets.
// Example: If packet 2 arrives before packet 1, it processes packet 2 and packet 1 gets dropped.
//
// For more stable see stableDataWithOrderTimeoutFrame
const StableDataWithOrderInstantFrame stableFrameType = 6

// stableDataWithOrderFrame is data frame code with checking for order of packets, drops out of order packets after timeout.
// Example: When packet 2 arrieves, it waits timeout (if there are packets before that have not arrived) before processing the packet 2 and dropping all older packets.
//
// For instant processing see: stableDataWithOrderInstantFrame.
//
// Note: Timeout is set via setting in connection stabilizer
const StableDataWithOrderTimeoutFrame stableFrameType = 7

// StableDataWithOrderResendFrame is data frame code with checking for order of packets and for delivery. It works like TCP.
//
// Warning: This type can introduce big latency or infinite waiting for packets, if they get lost. Packets are waiting for strict order.
// Example: If packet 2 arrives before packet 1, it waits until packet 1 is recieved.
//
// Note: For this frame type ResendRetries is set to 0
const StableDataWithOrderResendFrame stableFrameType = 8

// ConnectionStabilizerSettings are settings used in connectionStabilizer
type ConnectionStabilizerSettings struct {
	// KeepAliveIntervalSeconds sets how long it takes before keepAlive (ping packet) is send, set it to 0 to disable.
	//
	// Warning: Cant be changed in runtime
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

	// TimeBetweenACKPackets is maximum duration between two ACK-Compatible packets. Also used as timeout for delayed ACK.
	//
	// Recommendation: Set this to 10 ms
	TimeBetweenACKPackets time.Duration

	// WindoWordCount is count of words of window.
	//
	// Warning: Cant be changed in runtime
	WindowWordCount uint8

	// HeapSizeSimple is initial size of heap of orderer for type stableDataWithOrderInstantFrame and stableDataWithOrderTimeoutFrame
	//
	// Recommendation: Set this to 0 (if you use instant) or 16 (if you use timeout)
	HeapSizeSimple uint

	// HeapSizePrecise is initial size of heap of orderer for type stableDataWithOrderResendFrame
	//
	// Recommendation: Set this to 16
	HeapSizePrecise uint
}

// SetRecommended sets recommended values to settings.
//
// Warning: Wipes all your settings
func (settings *ConnectionStabilizerSettings) SetRecommended() {
	settings.KeepAliveInterval = 20 * time.Second
	settings.KeepAliveTriesBeforeError = 10
	settings.KeepAliveResendOnNoPong = true
	settings.ResendRetries = 10
	settings.OrderDelay = 25 * time.Millisecond
	settings.FallbackRTT = 100 * time.Millisecond
	settings.MinimumRTT = 10 * time.Millisecond
	settings.MaximumRTT = 1500 * time.Millisecond
	settings.TimeBetweenACKPackets = 10 * time.Millisecond
	settings.HeapSizeSimple = 16
	settings.HeapSizePrecise = 16
}

// connectionStabilizer is internal struct for universal handeling of reads and writes from/to UDP
type connectionStabilizer[connType UniversalConn, sequenceNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowWordType ~uint8 | ~uint16 | ~uint32 | ~uint64] struct {
	settings                *ConnectionStabilizerSettings
	readFunc                StabilizerReadFunc[sequenceNumberType, orderNumberType, windowWordType]
	conns                   helpertools.SafeMap[connType, *connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType]]
	keepAliveTickerStopFunc context.CancelFunc
}

// newConnectionStabilizer creates new connectionStabilizer
func newConnectionStabilizer[connType UniversalConn, sequenceNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowWordType ~uint8 | ~uint16 | ~uint32 | ~uint64](settings *ConnectionStabilizerSettings, readFunc StabilizerReadFunc[sequenceNumberType, orderNumberType, windowWordType], isServer bool) *connectionStabilizer[connType, sequenceNumberType, orderNumberType, windowWordType] {
	//Create stabilizer
	stabilizer := &connectionStabilizer[connType, sequenceNumberType, orderNumberType, windowWordType]{
		settings:                settings,
		readFunc:                readFunc,
		conns:                   helpertools.MakeSafeMap[connType, *connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType]](helpertools.FormatByBool(isServer, 0, 1)),
		keepAliveTickerStopFunc: nil,
	}

	//Setup keep alive
	if settings.KeepAliveInterval != 0 {
		ctx, cancel := context.WithCancel(context.Background())
		go stabilizer.keepAliveSender(ctx)
		stabilizer.keepAliveTickerStopFunc = cancel
	}
	return stabilizer
}

// connectionStabilizerConn is internal conn of stabilizer
type connectionStabilizerConn[sequenceNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowWordType ~uint8 | ~uint16 | ~uint32 | ~uint64] struct {
	// sendPacketResendNumber is sequence number for resend prevention and request
	sendPacketResendNumber atomic.Uint64
	// missingPingPackets is counter of missing and send packets
	missingPingPackets atomic.Uint32
	// lastACKTimestampsUnixNano is timestamp of last send ACK-Compatible packet in UNIX Nano per word
	lastACKTimestampsUnixNano []atomic.Int64
	// sendPacketOrderNumberSimple is sequence number for ordering of packets (for types stableDataWithOrderInstantFrame and stableDataWithOrderTimeoutFrame)
	sendPacketOrderNumberSimple orderNumberType
	// sendPacketOrderNumberPrecise is sequence number for ordering of packets (for type stableDataWithOrderResendFrame)
	sendPacketOrderNumberPrecise orderNumberType

	// sendedPacketsACKsWindow is used for lookup if packets need to be resended
	sendedPacketsACKsWindow helpertools.ReplayWindow[sequenceNumberType, windowWordType]
	// incomingPacketsWindow is used for checking if packet with sequence number already got recieved or not
	incomingPacketsWindow helpertools.ReplayWindow[sequenceNumberType, windowWordType]
	// rttCalculator is rtt calculator for transmition timing
	rttCalculator helpertools.RTTCalculator
	// incomingLatencyRTTCalculator is average of latency for ACK packet
	incomingLatencyRTTCalculator helpertools.RTTCalculator
	// sendPacketOrderSimpleMutex is mutex for ordering packets so original order can be preserved
	sendPacketOrderSimpleMutex sync.Mutex
	// sendPacketOrderPreciseMutex is mutex for ordering packets so original order can be preserved
	sendPacketOrderPreciseMutex sync.Mutex

	// ordererSimple is orderer for types stableDataWithOrderInstantFrame and stableDataWithOrderTimeoutFrame
	ordererSimple helpertools.PacketOrderer[orderNumberType, []byte]
	// ordererSimple is orderer for type stableDataWithOrderResendFrame
	ordererPrecise helpertools.PacketOrderer[orderNumberType, []byte]
}

// newConnectionStabilizerConn creates new connection for stabilizer
func newConnectionStabilizerConn[sequenceNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowWordType ~uint8 | ~uint16 | ~uint32 | ~uint64](settings *ConnectionStabilizerSettings) *connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType] {
	conn := &connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType]{
		rttCalculator:                *helpertools.NewRTTCalculator(settings.MinimumRTT, settings.MaximumRTT, settings.FallbackRTT),
		incomingLatencyRTTCalculator: *helpertools.NewRTTCalculator(settings.MinimumRTT, settings.MaximumRTT, settings.FallbackRTT),
		sendPacketOrderNumberSimple:  0,
		sendPacketOrderNumberPrecise: 0,
		sendPacketOrderSimpleMutex:   sync.Mutex{},
		lastACKTimestampsUnixNano:    make([]atomic.Int64, settings.WindowWordCount),
		ordererSimple:                *helpertools.NewPacketOrderer[orderNumberType, []byte](helpertools.AllowInDumpAndPush, false, settings.HeapSizeSimple),
		ordererPrecise:               *helpertools.NewPacketOrderer[orderNumberType, []byte](helpertools.AllowInDump, false, settings.HeapSizePrecise),
		incomingPacketsWindow:        helpertools.MakeReplayWindow[sequenceNumberType, windowWordType](settings.WindowWordCount),
		sendedPacketsACKsWindow:      helpertools.MakeReplayWindow[sequenceNumberType, windowWordType](settings.WindowWordCount),
	}
	conn.sendPacketResendNumber.Store(0)
	conn.missingPingPackets.Store(0)
	return conn
}

// HandleRead is handle function for reading and should be called when packet is recieved
func (stabilizer *connectionStabilizer[connType, sequenceNumberType, orderNumberType, windowWordType]) HandleRead(conn connType, framedData []byte) {
	//Check if has at least one byte
	if len(framedData) == 0 {
		return
	}

	//Get first byte (frame type)
	var frameType stableFrameType = stableFrameType(framedData[0])
	framedData = framedData[1:]

	//Read ACK settings
	if frameType != StableDataFrame && frameType != StableDataWithOrderInstantFrame && frameType != StableDataWithOrderTimeoutFrame && frameType != stableDataRecievedFrame {
		//Valid ACK-compatible data packet
		framedData = framedData[stabilizer.conns.Get(conn).sendedPacketsACKsWindow.JoinWindowDataBytes(framedData, 1):]
	}

	//Sort types
	switch frameType {
	case stablePingFrame:
		{
			//Ping frame = reply with pong
			if len(framedData) == 8 {
				conn.GetLogger().Log(1, "Got ping from: "+conn.GetAddress().String()+", Time: "+strconv.FormatInt(time.Since(time.UnixMicro(int64(binary.LittleEndian.Uint64(framedData)))).Milliseconds(), 10)+" ms")
				stabilizer.HandleWrite(conn, stablePongFrame, framedData)
			} else {
				conn.GetLogger().Log(3, "Got invalid ping from: "+conn.GetAddress().String())
			}
		}
	case stablePongFrame:
		{
			//Pong frame - check frame
			if len(framedData) != 8 {
				conn.GetLogger().Log(3, "Got invalid pong from: "+conn.GetAddress().String())
				return
			}

			//Calculate RTO
			sConn := stabilizer.conns.Get(conn)
			if sConn == nil {
				conn.GetLogger().Log(3, "Invalid connection for: "+conn.GetAddress().String())
				return
			}
			delta := time.Since(time.UnixMicro(int64(binary.LittleEndian.Uint64(framedData))))
			conn.GetLogger().Log(1, "Got pong from: "+conn.GetAddress().String()+", 2*Time: "+strconv.FormatInt(delta.Milliseconds(), 10)+" ms")
			sConn.rttCalculator.CalculateRTT(delta)
			if stabilizer.settings.KeepAliveTriesBeforeError != -1 {
				sConn.missingPingPackets.Store(0)
			}
		}
	case stableDataRecievedFrame:
		{
			//ACK Frame - Calculate RTO and process Window
			sConn := stabilizer.conns.Get(conn)
			if sConn == nil {
				conn.GetLogger().Log(3, "Invalid connection for: "+conn.GetAddress().String())
				return
			}
			delta := time.Since(time.UnixMicro(int64(binary.LittleEndian.Uint64(framedData))))
			conn.GetLogger().Log(1, "Got ACK frame from: "+conn.GetAddress().String()+", Time: "+strconv.FormatInt(delta.Milliseconds(), 10)+" ms")
			sConn.rttCalculator.CalculateRTT(delta)
			fmt.Println("RTO:", sConn.rttCalculator.GetRTO().Milliseconds(), "ms")
			framedData = framedData[8:]

			//Process Window
			stabilizer.conns.Get(conn).sendedPacketsACKsWindow.JoinWindowDataBytes(framedData, 0)
		}
	case StableDataFrame:
		{
			//Data frame - no checking applied = pass to read func
			conn.GetLogger().Log(1, "Got data frame from: "+conn.GetAddress().String())
			if stabilizer.readFunc != nil {
				sConn := stabilizer.conns.Get(conn)
				if sConn == nil {
					conn.GetLogger().Log(3, "Invalid connection for: "+conn.GetAddress().String())
					return
				}
				stabilizer.readFunc(sConn, framedData)
			}
		}
	case StableDataWithResendFrame:
		{
			//Data frame with resend function - calculate latency and apply window and send ACK
			sConn := stabilizer.conns.Get(conn)
			if sConn == nil {
				conn.GetLogger().Log(3, "Invalid connection for: "+conn.GetAddress().String())
				return
			}
			delta := time.Since(time.UnixMicro(int64(binary.LittleEndian.Uint64(framedData))))
			sConn.incomingLatencyRTTCalculator.CalculateRTT(delta)
			framedData = framedData[8:]

			//Apply window
			seqNum, readBytes := helpertools.ParseGenericLitteEndian[sequenceNumberType](framedData)
			conn.GetLogger().Log(1, "Got data frame with resend from: "+conn.GetAddress().String()+" with sequence number: "+strconv.FormatUint(uint64(seqNum), 10)+", Time: "+strconv.FormatInt(delta.Milliseconds(), 10)+" ms")
			if sConn.incomingPacketsWindow.ApplyWindowCheck(seqNum) {
				//Pass to read func
				if stabilizer.readFunc != nil {
					stabilizer.readFunc(sConn, framedData[readBytes:])
				}
			}

			//Request ACK
			stabilizer.handleACKSend(conn, sConn, seqNum, true)
		}
	case StableDataWithOrderInstantFrame:
		{
			//Data frame with instant ordering function - apply simple orderer
			sConn := stabilizer.conns.Get(conn)
			if sConn == nil {
				conn.GetLogger().Log(3, "Invalid connection for: "+conn.GetAddress().String())
				return
			}
			orderNum, readBytes := helpertools.ParseGenericLitteEndian[orderNumberType](framedData)
			conn.GetLogger().Log(1, "Got data frame with instant order from: "+conn.GetAddress().String()+" with order number: "+strconv.FormatUint(uint64(orderNum), 10))
			for _, v := range sConn.ordererSimple.Push(orderNum, framedData[readBytes:]) {
				if stabilizer.readFunc != nil {
					stabilizer.readFunc(sConn, v)
				}
			}
		}
	case StableDataWithOrderTimeoutFrame:
		{
			//Data frame with instant ordering function - apply simple orderer
			sConn := stabilizer.conns.Get(conn)
			if sConn == nil {
				conn.GetLogger().Log(3, "Invalid connection for: "+conn.GetAddress().String())
				return
			}
			orderNum, readBytes := helpertools.ParseGenericLitteEndian[orderNumberType](framedData)
			conn.GetLogger().Log(1, "Got data frame with timeout order from: "+conn.GetAddress().String()+" with order number: "+strconv.FormatUint(uint64(orderNum), 10))
			for _, v := range sConn.ordererSimple.PushWithMissingPacketOption(orderNum, framedData[readBytes:], helpertools.AllowNone) {
				if stabilizer.readFunc != nil {
					stabilizer.readFunc(sConn, v)
				}
			}

			//Start timeout
			time.AfterFunc(stabilizer.settings.OrderDelay, func() {
				for _, v := range sConn.ordererSimple.Dump(orderNum) {
					if stabilizer.readFunc != nil {
						stabilizer.readFunc(sConn, v)
					}
				}
			})
		}
	case StableDataWithOrderResendFrame:
		{
			//Data frame with resend and order function - calculate latency, apply window and send ACK and order packets
			sConn := stabilizer.conns.Get(conn)
			if sConn == nil {
				conn.GetLogger().Log(3, "Invalid connection for: "+conn.GetAddress().String())
				return
			}
			delta := time.Since(time.UnixMicro(int64(binary.LittleEndian.Uint64(framedData))))
			sConn.incomingLatencyRTTCalculator.CalculateRTT(delta)
			framedData = framedData[8:]

			//Apply window
			seqNum, readBytes := helpertools.ParseGenericLitteEndian[sequenceNumberType](framedData)
			framedData = framedData[readBytes:]
			orderNum, readBytes := helpertools.ParseGenericLitteEndian[orderNumberType](framedData)
			conn.GetLogger().Log(1, "Got data frame with resend and order from: "+conn.GetAddress().String()+" with sequence number: "+strconv.FormatUint(uint64(seqNum), 10)+" and order number: "+strconv.FormatUint(uint64(orderNum), 10)+", Time: "+strconv.FormatInt(delta.Milliseconds(), 10)+" ms")
			if sConn.incomingPacketsWindow.ApplyWindowCheck(seqNum) {
				for _, v := range sConn.ordererPrecise.Push(orderNum, framedData[readBytes:]) {
					//Pass to read func
					if stabilizer.readFunc != nil {
						stabilizer.readFunc(sConn, v)
					}
				}
			}

			//Request ACK
			stabilizer.handleACKSend(conn, sConn, seqNum, true)
		}
	default:
		{
			conn.GetLogger().Log(3, "Invalid frame type: "+strconv.FormatUint(uint64(frameType), 10)+" for: "+conn.GetAddress().String())
		}
	}
}

// handleACKSend handles sending ACK packets
func (stabilizer *connectionStabilizer[connType, sequenceNumberType, orderNumberType, windowWordType]) handleACKSend(conn connType, sConn *connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType], seqNum sequenceNumberType, loop bool) {
	//Check for validity
	if sConn == nil {
		conn.GetLogger().Log(3, "Invalid connection for: "+conn.GetAddress().String())
		return
	}

	//Calculate word count
	wordIndex := uint8((sConn.incomingPacketsWindow.GetRightEdge() - seqNum) >> uint32(helpertools.GetBitShiftSize[windowWordType]()))
	//fmt.Println(sConn.incomingPacketsWindow.GetRightEdge(), seqNum, sequenceNumberType(helpertools.GetBitSize[windowWordType]()-1), wordIndex)

	//Check for duration
	if time.Since(time.Unix(0, sConn.lastACKTimestampsUnixNano[wordIndex].Load())) >= stabilizer.settings.TimeBetweenACKPackets {
		stabilizer.HandleWrite(conn, stableDataRecievedFrame, []byte{wordIndex + 1})
		return
	}

	//Wait some time for loop
	if loop {
		time.AfterFunc(stabilizer.settings.TimeBetweenACKPackets, func() {
			stabilizer.handleACKSend(conn, sConn, seqNum, false)
		})
	}
}

// HandleWrite handles writes in stabilizer with default settings
func (stabilizer *connectionStabilizer[connType, sequenceNumberType, orderNumberType, windowWordType]) HandleWriteDefault(conn connType, data []byte) {
	if stabilizer.settings.DefaultSendFrameType == stablePingFrame || stabilizer.settings.DefaultSendFrameType == stablePongFrame || stabilizer.settings.DefaultSendFrameType == stableDataRecievedFrame || stabilizer.settings.DefaultSendFrameType == 0 {
		stabilizer.settings.DefaultSendFrameType = StableDataFrame
	}
	stabilizer.HandleWrite(conn, stabilizer.settings.DefaultSendFrameType, data)
}

// HandleWrite handles writes in stabilizer
func (stabilizer *connectionStabilizer[connType, sequenceNumberType, orderNumberType, windowWordType]) HandleWrite(conn connType, frameType stableFrameType, data []byte) {
	//Create new byte buffer
	buffer := make([]byte, 1)
	buffer[0] = byte(frameType)
	var sConn *connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType]

	//Write ACK is possible
	if frameType != StableDataFrame && frameType != StableDataWithOrderInstantFrame && frameType != StableDataWithOrderTimeoutFrame && frameType != stableDataRecievedFrame {
		sConn = stabilizer.conns.Get(conn)
		if sConn == nil {
			conn.GetLogger().Log(3, "Invalid connection for: "+conn.GetAddress().String())
			return
		}
		sConn.lastACKTimestampsUnixNano[len(sConn.lastACKTimestampsUnixNano)-1].Store(time.Now().UnixNano())
		buffer = append(buffer, stabilizer.conns.Get(conn).incomingPacketsWindow.GetWindowDataBytes(1, false)...)
	}

	//Process specific types
	switch frameType {
	case stablePingFrame:
		{
			//Ping frame
			if stabilizer.settings.KeepAliveTriesBeforeError == -1 || sConn.missingPingPackets.Load() < uint32(stabilizer.settings.KeepAliveTriesBeforeError) {
				//Can send ping packet
				conn.GetLogger().Log(1, "Sending ping to: "+conn.GetAddress().String())
				buffer = binary.LittleEndian.AppendUint64(buffer, uint64(time.Now().UnixMicro()))
				conn.Send(buffer)
				if stabilizer.settings.KeepAliveTriesBeforeError != -1 {
					sConn.missingPingPackets.Add(1)
				}
			} else {
				//To much missing packets
				conn.GetLogger().Log(3, "Too much missing ping packets with: "+conn.GetAddress().String())
				conn.Close()
			}
		}
	case stablePongFrame:
		{
			//Pong frame
			conn.GetLogger().Log(1, "Sending pong to: "+conn.GetAddress().String())
			conn.Send(append(buffer, data...))
		}
	case stableDataRecievedFrame:
		{
			//Data recieved frame - ACK frame
			conn.GetLogger().Log(1, "Sending ACK frame to: "+conn.GetAddress().String()+" with word limit: "+strconv.FormatUint(uint64(uint8(data[0])), 10))
			sConn = stabilizer.conns.Get(conn)
			for i := range uint8(data[0]) {
				sConn.lastACKTimestampsUnixNano[uint8(len(sConn.lastACKTimestampsUnixNano)-1)-i].Store(time.Now().UnixNano())
			}
			buffer = binary.LittleEndian.AppendUint64(buffer, uint64(time.Now().Add(-1*sConn.incomingLatencyRTTCalculator.GetRTT()).UnixMicro()))
			conn.Send(append(buffer, sConn.incomingPacketsWindow.GetWindowDataBytes(uint8(data[0]), true)...))
		}
	case StableDataFrame:
		{
			//Data frame - no checking applied
			conn.GetLogger().Log(1, "Sending pure data frame to: "+conn.GetAddress().String())
			conn.Send(append(buffer, data...))
		}
	case StableDataWithResendFrame:
		{
			//Data frame with resend function - add sequence number and data and pass to writer function
			sequenceNumber := sequenceNumberType(sConn.sendPacketResendNumber.Add(1) - 1)
			buffer = binary.LittleEndian.AppendUint64(buffer, uint64(time.Now().UnixMicro()))
			buffer, _ = helpertools.AppendGenericLitteEndian(buffer, sequenceNumber)
			go stabilizer.resendWrite(conn, sConn, sequenceNumber, append(buffer, data...), false)
		}
	case StableDataWithOrderInstantFrame, StableDataWithOrderTimeoutFrame:
		{
			//Data frame with order function (instant / timeout) - add order number and data and send
			sConn.sendPacketOrderSimpleMutex.Lock()
			orderNumber := sConn.sendPacketOrderNumberSimple
			sConn.sendPacketOrderNumberSimple++
			sConn.sendPacketOrderSimpleMutex.Unlock()

			//Add all bytes
			conn.GetLogger().Log(1, "Sending ordered data frame with order number: "+strconv.FormatUint(uint64(orderNumber), 10)+" to: "+conn.GetAddress().String())
			buffer, _ = helpertools.AppendGenericLitteEndian(buffer, orderNumber)
			conn.Send(append(buffer, data...))
		}
	case StableDataWithOrderResendFrame:
		{
			//Data frame with order and resend function - add sequence number, order number and send
			sConn.sendPacketOrderPreciseMutex.Lock()
			orderNumber := sConn.sendPacketOrderNumberPrecise
			sConn.sendPacketOrderNumberPrecise++
			sConn.sendPacketOrderPreciseMutex.Unlock()
			sequenceNumber := sequenceNumberType(sConn.sendPacketResendNumber.Add(1) - 1)
			buffer = binary.LittleEndian.AppendUint64(buffer, uint64(time.Now().UnixMicro()))
			buffer, _ = helpertools.AppendGenericLitteEndian(buffer, sequenceNumber)
			buffer, _ = helpertools.AppendGenericLitteEndian(buffer, orderNumber)
			go stabilizer.resendWrite(conn, sConn, sequenceNumber, append(buffer, data...), true)
		}
	default:
		{
			conn.GetLogger().Log(3, "Invalid frame type: "+strconv.FormatUint(uint64(frameType), 10)+" for: "+conn.GetAddress().String())
		}
	}
}

// resendWrite is helper function for resendable writing to connection until valid ACK is recieved or timeout is reached
func (stabilizer *connectionStabilizer[connType, sequenceNumberType, orderNumberType, windowWordType]) resendWrite(conn connType, sConn *connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType], sequenceNumber sequenceNumberType, data []byte, infiniteSend bool) {
	//Check for validity
	if sConn == nil {
		conn.GetLogger().Log(3, "Invalid connection for: "+conn.GetAddress().String())
		return
	}

	//Send
	for sendTry := uint8(0); stabilizer.settings.ResendRetries == 0 || infiniteSend || sendTry <= stabilizer.settings.ResendRetries; sendTry++ {
		conn.GetLogger().Log(1, "Sending resend data frame with sequence number: "+strconv.FormatUint(uint64(sequenceNumber), 10)+" to: "+conn.GetAddress().String()+" with try: "+strconv.FormatUint(uint64(sendTry), 10))
		conn.Send(data)
		time.Sleep(sConn.rttCalculator.GetRTO())
		if sConn.sendedPacketsACKsWindow.CheckValue(sequenceNumber) {
			return
		}
	}
	conn.GetLogger().Log(2, "Failed sending resend data frame to: "+conn.GetAddress().String()+" for sequence number: "+strconv.FormatUint(uint64(sequenceNumber), 10))
}

// HandleConnect should be called everytime new connection connects to server / client
func (stabilizer *connectionStabilizer[connType, sequenceNumberType, orderNumberType, windowWordType]) HandleConnect(conn connType) (sConn *connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType]) {
	//Create connection
	sConn = newConnectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType](stabilizer.settings)
	stabilizer.conns.Set(conn, sConn)

	//Send 3 KeepAlives
	go func() {
		for range uint8(3) {
			stabilizer.HandleWrite(conn, stablePingFrame, nil)
			time.Sleep(sConn.rttCalculator.GetRTO())
		}
	}()
	return sConn
}

// GetConn retuns stable connection
func (stabilizer *connectionStabilizer[connType, sequenceNumberType, orderNumberType, windowWordType]) GetConn(conn connType) *connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType] {
	return stabilizer.conns.Get(conn)
}

// CleanupConnection removes specified connection
func (stabilizer *connectionStabilizer[connType, sequenceNumberType, orderNumberType, windowWordType]) CleanupConnection(conn connType) {
	stabilizer.conns.Delete(conn)
}

// CleanupConnections removes all connections
func (stabilizer *connectionStabilizer[connType, sequenceNumberType, orderNumberType, windowWordType]) CleanupConnections() {
	stabilizer.conns.Clear()
}

// keepAliveSender is internal function for sending keep alives
func (stabilizer *connectionStabilizer[connType, sequenceNumberType, orderNumberType, windowWordType]) keepAliveSender(ctx context.Context) {
	//Create new ticker
	ticker := time.NewTicker(stabilizer.settings.KeepAliveInterval)
	defer ticker.Stop()

	//Run loop
	for {
		select {
		case <-ctx.Done():
			{
				return
			}
		case <-ticker.C:
			{
				stabilizer.conns.Range(func(conn connType, _ *connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType]) (doBreak bool) {
					stabilizer.HandleWrite(conn, stablePingFrame, nil)
					return false
				})
				return
			}
		}
	}
}

// Stop stops stabilizer
func (stabilizer *connectionStabilizer[connType, sequenceNumberType, orderNumberType, windowWordType]) Stop() {
	if stabilizer.keepAliveTickerStopFunc != nil {
		stabilizer.keepAliveTickerStopFunc()
	}
}
