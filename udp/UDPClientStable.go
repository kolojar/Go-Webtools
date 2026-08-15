package udp

import (
	"net"
	"strconv"

	webtools "github.com/kolojar/Go-Webtools"
	"github.com/kolojar/Go-Webtools/helpertools"
)

// ClientReadFunc is function definition for reading data from Client
type ClientStableReadFunc[sequenceNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowWordType ~uint8 | ~uint16 | ~uint32 | ~uint64] func(client *ClientStable[sequenceNumberType, orderNumberType, windowWordType], sourceAddress *net.UDPAddr, data []byte, status webtools.NetworkStatus)

// ClientStable is struct for client stable
type ClientStable[sequenceNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowWordType ~uint8 | ~uint16 | ~uint32 | ~uint64] struct {
	client        *Client
	stabilizer    connectionStabilizer[*Client, sequenceNumberType, orderNumberType, windowWordType]
	readFunc      ClientStableReadFunc[sequenceNumberType, orderNumberType, windowWordType]
	sourceAddress *net.UDPAddr
}

// NewClientStable creates new UDP Stable client but does not starts it
func NewClientStable[sequenceNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, windowWordType ~uint8 | ~uint16 | ~uint32 | ~uint64](address string, readFunc ClientStableReadFunc[sequenceNumberType, orderNumberType, windowWordType], settings *ConnectionStabilizerSettings, reportTraffic bool) (*ClientStable[sequenceNumberType, orderNumberType, windowWordType], error) {
	//Create stable client
	sClient := &ClientStable[sequenceNumberType, orderNumberType, windowWordType]{
		readFunc: readFunc,
	}

	//Create client
	client, err := NewClient(address, sClient.readFuncLocal, reportTraffic)
	if err != nil {
		return nil, err
	}
	sClient.client = client

	//Create connection stabilizer
	sClient.stabilizer = *newConnectionStabilizer[*Client](settings, sClient.stabilizerReadFunc, false)
	return sClient, nil
}

// stabilizerReadFunc is local function that handles reading from stabilizer
func (cl *ClientStable[sequenceNumberType, orderNumberType, windowWordType]) stabilizerReadFunc(_ *connectionStabilizerConn[sequenceNumberType, orderNumberType, windowWordType], data []byte) {
	if cl.readFunc != nil {
		cl.readFunc(cl, cl.sourceAddress, data, webtools.ReadDataStatus)
	}
}

// readFuncLocal is local function that handles reading from UDP client
func (cl *ClientStable[sequenceNumberType, orderNumberType, windowWordType]) readFuncLocal(_ *Client, sourceAddress *net.UDPAddr, data []byte, status webtools.NetworkStatus) {
	cl.sourceAddress = sourceAddress
	switch status {
	case webtools.ConnectStatus:
		{
			//Handle connect
			cl.stabilizer.HandleConnect(cl.client)
			if cl.readFunc != nil {
				cl.readFunc(cl, sourceAddress, nil, status)
			}
		}
	case webtools.DisconnectStatus:
		{
			//Handle disconnect
			cl.stabilizer.CleanupConnection(cl.client)
			if cl.readFunc != nil {
				cl.readFunc(cl, sourceAddress, nil, status)
			}
		}
	case webtools.ReadDataStatus:
		{
			//Handle read
			cl.stabilizer.HandleRead(cl.client, data)
		}
	default:
		{
			cl.client.Logger.Log(4, "Invalid connection status: "+strconv.FormatUint(uint64(status), 10)+" for connection: "+sourceAddress.String())
		}
	}
}

// Connect connects to UDP server and start reading loop, does not locks execution thread
func (cl *ClientStable[sequenceNumberType, orderNumberType, windowWordType]) Connect() error {
	return cl.client.Connect()
}

// Send sends data to server
func (cl *ClientStable[sequenceNumberType, orderNumberType, windowWordType]) Send(data []byte) {
	cl.stabilizer.HandleWriteDefault(cl.client, data)
}

// SendAdvanced writes data to connection using specific frame settings
//
// Note: When useResend is true, any orderLevel not equal to 0 will enable ordering
func (cl *ClientStable[sequenceNumberType, orderNumberType, windowWordType]) SendAdvanced(data []byte, useResend bool, orderLevel StabilizerOrderLevel) {
	if useResend {
		if orderLevel == StabilizerNoOrdering {
			cl.stabilizer.HandleWrite(cl.client, StableDataWithResendFrame, data)
		} else {
			cl.stabilizer.HandleWrite(cl.client, StableDataWithOrderResendFrame, data)
		}
	} else {
		switch orderLevel {
		case StabilizerNoOrdering:
			cl.stabilizer.HandleWrite(cl.client, StableDataFrame, data)
		case StabilizerInstantOrdering:
			cl.stabilizer.HandleWrite(cl.client, StableDataWithOrderInstantFrame, data)
		case StabilizerTimeoutOrdering:
			cl.stabilizer.HandleWrite(cl.client, StableDataWithOrderTimeoutFrame, data)
		default:
			panic("unknown orderLevel: " + strconv.FormatUint(uint64(orderLevel), 10))
		}
	}
}

// Stop stops UDP client
func (cl *ClientStable[sequenceNumberType, orderNumberType, windowWordType]) Stop() {
	cl.stabilizer.HandleDisconnect(cl.client)
	cl.stabilizer.Stop()
	cl.client.Stop()
}

// Close is alias for Stop
func (cl *ClientStable[sequenceNumberType, orderNumberType, windowWordType]) Close() {
	cl.Stop()
}

// GetLogger gets logger of client
func (cl *ClientStable[sequenceNumberType, orderNumberType, windowWordType]) GetLogger() *helpertools.ConsoleLogger {
	return cl.client.Logger
}

// IsAlive checks if client is alive
func (cl *ClientStable[sequenceNumberType, orderNumberType, windowWordType]) IsAlive() bool {
	return cl.client.isAlive
}
