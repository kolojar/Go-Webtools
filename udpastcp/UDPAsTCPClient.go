package udpastcp

import (
	"encoding/hex"
	"net"
	"strconv"
	"webtools/udp"
)

/*
Client is Client that simulates net.Conn (TCP conn) on top of UDP
*/
type Client struct {
	client                   *udp.Client
	conn                     *Conn
	preservePacketBoundaries bool
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
func NewClient(address string, preservePacketBoundaries bool, reportTraffic bool) (*Client, error) {
	cl := &Client{preservePacketBoundaries: preservePacketBoundaries}
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
func (client *Client) Connect() (*Conn, error) {
	//Connect
	err := client.client.Connect()
	if err != nil {
		return nil, err
	}

	//Make connection
	client.conn = NewConn(client.client.Conn.LocalAddr(), client.client.Conn.RemoteAddr(), func(data []byte) (n int, err error) {
		//Write func
		return client.client.Send(data)
	}, func() error {
		//Close func
		client.Stop()
		return nil
	}, client.preservePacketBoundaries)
	return client.conn, nil
}

/*
Handles UDP Read for client
*/
func (client *Client) readFuncLocal(_ *udp.Client, sourceAddress *net.UDPAddr, data []byte, ended bool) {
	if !ended {
		//Process read
		client.client.Logger.Log(0, "Reading from and buffering: "+sourceAddress.String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
		err := client.conn.WriteToReadBuffer(data)
		if err != nil {
			client.client.Logger.Log(4, "Error writing to buffer: "+err.Error())
		}
	} else {
		//Process end
		client.conn.Close()
	}
}

/*
Stop stops UDP as TCP client
*/
func (server *Client) Stop() {
	server.client.Stop()
}
