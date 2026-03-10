/*
Package udpastcp provides UDP tools that simulates TCP connections syntax
*/
package udpastcp

import (
	"bytes"
	"net"
	"os"
	"time"
	"webtools/udp"
)

type ServerConn struct {
	origin        *Server
	conn          *udp.ServerConn
	localAddress  *net.UDPAddr
	readDeadline  time.Time
	writeDeadline time.Time
	buffer        bytes.Buffer
	ended         bool
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
func (conn *ServerConn) Read(b []byte) (n int, err error) {
	if conn.ended {
		return 0, os.ErrClosed
	}
	if time.Until(conn.readDeadline) < 0 && !conn.readDeadline.IsZero() {
		conn.buffer.Reset()
		return 0, os.ErrDeadlineExceeded
	}
	for conn.buffer.Len() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	return conn.buffer.Read(b)
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (conn *ServerConn) Write(b []byte) (n int, err error) {
	if conn.ended {
		return 0, os.ErrClosed
	}
	if time.Until(conn.writeDeadline) < 0 && !conn.writeDeadline.IsZero() {
		return 0, os.ErrDeadlineExceeded
	}
	return conn.conn.Send(b)
}

// Close closes the connection.
// Always returns nil.
func (conn *ServerConn) Close() error {
	err := conn.Close()
	conn.origin.conns.Delete(conn.conn)
	conn.ended = true
	return err
}

// LocalAddr returns the local network address, if known.
func (conn *ServerConn) LocalAddr() net.Addr {
	return conn.localAddress

}

// RemoteAddr returns the remote network address, if known.
func (conn *ServerConn) RemoteAddr() net.Addr {
	return conn.conn.Address
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
// Always returns nil
func (conn *ServerConn) SetDeadline(t time.Time) error {
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
func (conn *ServerConn) SetReadDeadline(t time.Time) error {
	conn.readDeadline = t
	return nil
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
// Always return nil
func (conn *ServerConn) SetWriteDeadline(t time.Time) error {
	conn.writeDeadline = t
	return nil
}

type ClientConn struct {
	client        *udp.Client
	readDeadline  time.Time
	writeDeadline time.Time
	buffer        bytes.Buffer
	ended         bool
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
func (conn *ClientConn) Read(b []byte) (n int, err error) {
	if conn.ended {
		return 0, os.ErrClosed
	}
	if time.Until(conn.readDeadline) < 0 && !conn.readDeadline.IsZero() {
		conn.buffer.Reset()
		return 0, os.ErrDeadlineExceeded
	}
	for conn.buffer.Len() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	return conn.buffer.Read(b)
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (conn *ClientConn) Write(b []byte) (n int, err error) {
	if conn.ended {
		return 0, os.ErrClosed
	}
	if time.Until(conn.writeDeadline) < 0 && !conn.writeDeadline.IsZero() {
		return 0, os.ErrDeadlineExceeded
	}
	return conn.client.Send(b)
}

// Close closes the connection - client.
// Always returns nil.
func (conn *ClientConn) Close() error {
	err := conn.Close()
	conn.ended = true
	return err
}

// LocalAddr returns the local network address, if known.
func (conn *ClientConn) LocalAddr() net.Addr {
	return conn.client.Conn.RemoteAddr()

}

// RemoteAddr returns the remote network address, if known.
func (conn *ClientConn) RemoteAddr() net.Addr {
	return conn.client.Conn.LocalAddr()
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
// Always returns nil
func (conn *ClientConn) SetDeadline(t time.Time) error {
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
func (conn *ClientConn) SetReadDeadline(t time.Time) error {
	conn.readDeadline = t
	return nil
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
// Always return nil
func (conn *ClientConn) SetWriteDeadline(t time.Time) error {
	conn.writeDeadline = t
	return nil
}
