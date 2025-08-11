package main
// tcp_bridge.go
import (
	"io"
	"log"
	"net"
	"time"
	"fmt"
)

func tcp() {
	// Listen on port A (e.g. 9000)
	listener, err := net.Listen("tcp", ":17777")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	log.Println("TCP bridge listening on :9000 and forwarding to :12345")

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}

		// Forward to port B (real backend)
		go bridge(clientConn, "127.0.0.1:7777")
	}
}

func bridge(clientConn net.Conn, targetAddr string) {
	defer clientConn.Close()

	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Println("Target connect error:", err)
		return
	}
	defer targetConn.Close()

	// Start bidirectional copy
	go io.Copy(targetConn, clientConn) // client → backend
	io.Copy(clientConn, targetConn)    // backend → client
}

type session struct {
	clientAddr *net.UDPAddr
	lastSeen   time.Time
}

var sessionMap = make(map[string]*session)

func udp() {
	listenAddr, _ := net.ResolveUDPAddr("udp", ":17777")          // public exposed port
	forwardAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:7777") // actual game server

	listener, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	serverConn, err := net.DialUDP("udp", nil, forwardAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer serverConn.Close()

	go func() {
		// Listen for server responses and map them back to client
		buf := make([]byte, 2048)
		for {
			n, _, err := serverConn.ReadFromUDP(buf)
			if err != nil {
				log.Println("Read from server failed:", err)
				continue
			}
			// Forward to last known client
			for _, s := range sessionMap {
				if time.Since(s.lastSeen) < 30*time.Second {
					listener.WriteToUDP(buf[:n], s.clientAddr)
				}
			}
		}
	}()

	buf := make([]byte, 2048)
	for {
		n, clientAddr, err := listener.ReadFromUDP(buf)
		if err != nil {
			log.Println("Client read failed:", err)
			continue
		}

		key := clientAddr.String()
		fmt.Println(clientAddr.String(),len(sessionMap))
		//sessionMap[key] = &session{clientAddr, time.Now()}
		sess, ok := sessionMap[key]
		if !ok {
			sess = &session{clientAddr: clientAddr}
			sessionMap[key] = sess
		}
		sess.lastSeen = time.Now()

		_, err = serverConn.Write(buf[:n])
		if err != nil {
			log.Println("Write to server failed:", err)
		}
	}
}

func main() {
	//go tcp()
	udp()
}
