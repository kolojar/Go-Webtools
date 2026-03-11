/*
Package udpastcp provides UDP tools that simulates TCP connections syntax
*/
package udpastcp

import (
	"bytes"
	"net"
	"os"
	"sync"
	"time"
	"webtools/database"
)

type UDPAsTCPConnCloseFunc func() error
type UDPAsTCPConnWriteFunc func(data []byte) (n int, err error)

type UDPAsTCPConn struct {
	//Sources
	originServer *Server
	originClient *Client

	//Addresses
	localAddress  net.Addr
	remoteAddress net.Addr

	//Deadlines
	readDeadline  time.Time
	writeDeadline time.Time

	//Buffers
	buffer             *bytes.Buffer
	unpackedReadBuffer *bytes.Buffer

	//Funcs
	writeFunc UDPAsTCPConnWriteFunc
	closeFunc UDPAsTCPConnCloseFunc

	//Statuses
	preservePacketBoundaries bool
	ended                    bool

	//Internals
	mutex     *sync.Mutex
	readReady chan bool
}

func NewUDPAsTCPConn(originServer *Server, originClient *Client, localAddress net.Addr, remoteAddress net.Addr, writeFunc UDPAsTCPConnWriteFunc, closeFunc UDPAsTCPConnCloseFunc, preservePacketBoundaries bool) *UDPAsTCPConn {
	conn := &UDPAsTCPConn{
		originServer:             originServer,
		originClient:             originClient,
		localAddress:             localAddress,
		remoteAddress:            remoteAddress,
		readDeadline:             time.Time{},
		writeDeadline:            time.Time{},
		buffer:                   bytes.NewBuffer(make([]byte, 0)),
		writeFunc:                writeFunc,
		closeFunc:                closeFunc,
		preservePacketBoundaries: preservePacketBoundaries,
		ended:                    false,
		mutex:                    &sync.Mutex{},
		readReady:                make(chan bool),
	}
	if preservePacketBoundaries {
		conn.unpackedReadBuffer = bytes.NewBuffer(make([]byte, 0))
	}
	return conn
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
func (conn *UDPAsTCPConn) Read(b []byte) (n int, err error) {
	//Sort basic errors
	if conn.ended {
		return 0, os.ErrClosed
	}
	if time.Until(conn.readDeadline) < 0 && !conn.readDeadline.IsZero() {
		conn.buffer.Reset()
		return 0, os.ErrDeadlineExceeded
	}

	//Wait for channel
	_, ok := <-conn.readReady
	if !ok {
		return 0, os.ErrInvalid
	}

	//Read data
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	if conn.preservePacketBoundaries {
		//Preserve boundaries
		if conn.unpackedReadBuffer.Len() == 0 {
			//Unpack next message
			data, err := database.ParseSliceDB(conn.buffer, database.ParseUint8DB)
			if err != nil {
				return 0, err
			}
			conn.unpackedReadBuffer.Write(data)
		}

		//Read from unpackedReadBuffer
		n, err = conn.unpackedReadBuffer.Read(b)
	} else {
		//Read direct
		n, err = conn.buffer.Read(b)
	}

	//Read more if possible
	if conn.buffer.Len() > 0 || (conn.unpackedReadBuffer != nil && conn.unpackedReadBuffer.Len() > 0) {
		go func() { conn.readReady <- true }()
	}
	return n, err
}

// WriteToReadBuffer writes data to read buffer
func (conn *UDPAsTCPConn) WriteToReadBuffer(b []byte) error {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	if conn.preservePacketBoundaries {
		err := database.ConvertSliceToBytesDB(conn.buffer, b, database.ConvertUint8ToBytesDB)
		if err != nil {
			return err
		}
		conn.readReady <- true
		return nil
	} else {
		conn.buffer.Write(b)
		return nil
	}
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (conn *UDPAsTCPConn) Write(b []byte) (n int, err error) {
	if conn.ended {
		return 0, os.ErrClosed
	}
	if time.Until(conn.writeDeadline) < 0 && !conn.writeDeadline.IsZero() {
		return 0, os.ErrDeadlineExceeded
	}
	if conn.writeFunc == nil {
		return 0, os.ErrInvalid
	}
	return conn.writeFunc(b)
}

// Close closes the connection.
// Always returns nil.
func (conn *UDPAsTCPConn) Close() error {
	//conn.origin.conns.Delete(conn.conn)
	conn.ended = true
	if conn.closeFunc == nil {
		return os.ErrInvalid
	}
	return conn.closeFunc()
}

// LocalAddr returns the local network address, if known.
func (conn *UDPAsTCPConn) LocalAddr() net.Addr {
	return conn.localAddress

}

// RemoteAddr returns the remote network address, if known.
func (conn *UDPAsTCPConn) RemoteAddr() net.Addr {
	return conn.remoteAddress
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
// Always returns nil
func (conn *UDPAsTCPConn) SetDeadline(t time.Time) error {
	err := conn.SetReadDeadline(t)
	if err != nil {
		return err
	}
	return conn.SetWriteDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
// Always returns nil
func (conn *UDPAsTCPConn) SetReadDeadline(t time.Time) error {
	conn.readDeadline = t
	return nil
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
// Always return nil
func (conn *UDPAsTCPConn) SetWriteDeadline(t time.Time) error {
	conn.writeDeadline = t
	return nil
}
