package udpastcp

import (
	"bytes"
	"encoding/hex"
	"net"
	"strconv"
	"time"
	"webtools/udp"
)

/*
Client is Client that simulates net.Conn (TCP conn) on top of UDP
*/
type Client struct {
	client *udp.Client
	conn   *ClientConn
}

/*
IsAlive gets if client is alive
*/
func (server *Client) IsAlive() bool {
	return server.client.IsAlive()
}

/*
NewServer creates new UDP Client but does not starts it
*/
func NewClient(address string, reportTraffic bool) (*Client, error) {
	cl := &Client{}
	var err error
	cl.client, err = udp.NewClient(address, cl.readFuncLocal, reportTraffic)
	if err != nil {
		return nil, err
	}
	cl.client.Logger.Prefix = "UDPAsTCPClient"
	return cl, nil
}

/*
Connect connects to UDP server, does not lock execution thread
*/
func (client *Client) Connect() (*ClientConn, error) {
	//Connect
	err := client.client.Connect()
	if err != nil {
		return nil, err
	}

	//Make connection
	client.conn = &ClientConn{client: client.client, readDeadline: time.Time{}, writeDeadline: time.Time{}, buffer: *bytes.NewBuffer(make([]byte, 0))}
	return client.conn, nil
}

/*
Handles UDP Read for client
*/
func (server *Client) readFuncLocal(_ *udp.Client, sourceAddress *net.UDPAddr, data []byte, ended bool) {
	if !ended {
		//Process read
		server.client.Logger.Log(0, "Reading from and buffering: "+sourceAddress.String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
		server.conn.buffer.Write(data)
	} else {
		//Process end
		server.conn.Close()
	}
}

/*
Stop stops UDP as TCP client
*/
func (server *Client) Stop() {
	server.client.Stop()
}
