package webrtc

import "io"

type DTLSHandshakeFragmentProcessor struct {
	fragments []DTLSHandshakeFragment
	//Packet max size is 1500 bytes -> Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-3.2.3
	recievedBytes ReplayWindow[uint16]
}

func MakeDTLSHandshakeFragmentProcessor(totalLength int) DTLSHandshakeFragmentProcessor {
	return DTLSHandshakeFragmentProcessor{fragments: make([]DTLSHandshakeFragment, 0), recievedBytes: MakeReplayWindow[uint16](totalLength)}
}

/*
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.2
Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.2
Type: 22
*/
type DTLSHandshakeFragment struct {
	handshakeType   uint8  //Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.2
	length          uint32 //uint24
	messageSequence uint16
	fragmentOffset  uint32 //uint24
	fragmentLength  uint32 //uint24
	fragmentData    []byte
}

func UnpackDTLSHandshakeFragment(reader io.Reader) (handshake DTLSHandshakeFragment, err error) {

}
