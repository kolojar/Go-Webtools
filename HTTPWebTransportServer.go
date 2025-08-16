package webtools

import (
	"errors"
	"net"
	"net/http"
	"strings"
)

/*
Standardized type of function
*HTTPWebTransportServerConn = Connection
String = message
Bool = is ended
*/
type HTTPWebTransportServerReadFunc func(conn *HTTPWebTransportServerConn, data []byte, status uint8)

/*
HTTP WebTransport server connection object
*/
type HTTPWebTransportServerConn struct {
	origin *HTTPWebTransportServer
	Client *TCPClientSimple
}

func (tcp *HTTPWebTransportServerConn) GetConn() *net.TCPConn {
	return tcp.Client.GetConn()
}

/*
Gets address and
Returns address for TCP and path to HTTP request
*/
func HTTPWebTransportGetAddressAndTarget(completeURL string) (string, string) {
	//Separate protocol and URL
	splitUrl := strings.SplitN(completeURL, "://", 2)
	protocol := ""
	url := splitUrl[len(splitUrl)-1]
	if len(splitUrl) > 1 {
		protocol = splitUrl[0]
	}

	//Separate Web address and path
	urlSplit := strings.SplitN(url, "/", 2)
	webAddress := urlSplit[0]
	path := "/"
	if len(urlSplit) > 1 {
		path += urlSplit[1]
	}

	//Get port by protocol
	portByProtocol := ""
	switch protocol {
	case "http":
		{
			portByProtocol = "80"
		}
	case "https":
		{
			portByProtocol = "443"
		}
	case "ws":
		{
			portByProtocol = "80"
		}
	case "wss":
		{
			portByProtocol = "443"
		}
	}

	//Check if webAddress has protocol + add port if needed
	tcpAddress := ""
	if len(strings.SplitN(webAddress, ":", 2)) == 1 {
		//No port, add from port protocol
		tcpAddress = webAddress + ":" + portByProtocol
	} else {
		tcpAddress = webAddress
	}

	return tcpAddress, path
}

/*
Sends data to client
*/
func (httpConn *HTTPWebTransportServerConn) Send(data []byte) {
	httpConn.Client.Send(data)
}

/*
Closes connection to client
*/
func (httpConn *HTTPWebTransportServerConn) Close() {
	httpConn.Client.Stop()
	//if err != nil {
	//	httpConn.origin.Logger.Log(3, "Error closing connection from: "+httpConn.Conn.RemoteAddr().String()+" connected locally to: "+httpConn.Conn.LocalAddr().String()+" with error: "+err.Error())
	//} else {
	//	httpConn.origin.Logger.Log(0, "Closed connectin on "+httpConn.Conn.RemoteAddr().String()+" connected locally to: "+httpConn.Conn.LocalAddr().String())
	//}
}

/*
Simple HTTP connection hijack server fo switching from HTTP to TCP.
This is NOT WebSocket HTTP server for JavaScript, it is intended for inner communication between Go server (this file) and Go client. It is used for HTTPProxy (TCP and UDP traffic over HTTP)
*/
type HTTPWebTransportServer struct {
	httpServer      *HTTPServer
	Logger          *ConsoleLogger
	conns           SafeMap[*TCPClientSimple, *HTTPWebTransportServerConn]
	readFunc        HTTPWebTransportServerReadFunc
	webtransportURL string
	reportTraffic   bool
}

/*
Creates new HTTP WebTransport Server but does not starts it
*/
func NewHTTPWebTransportServer(address string, readFunc HTTPWebTransportServerReadFunc, reportTraffic bool) *HTTPWebTransportServer {
	sv := &HTTPWebTransportServer{Logger: NewConsoleLoggerForTraffic("HTTP-WTServer", reportTraffic), reportTraffic: reportTraffic, readFunc: readFunc, conns: MakeSafeMap[*TCPClientSimple, *HTTPWebTransportServerConn](), webtransportURL: "/webtransport"}
	sv.httpServer = NewHTTPServer(address, sv.handleHTTPAccess, "", false)
	sv.httpServer.Logger = sv.Logger
	return sv
}

/*
Sets URL of WebTransport
*/
func (sv *HTTPWebTransportServer) SetWebTransportURL(newURL string) error {
	if !strings.HasPrefix(newURL, "/") {
		return errors.New("url must start with /")
	}
	sv.webtransportURL = newURL
	return nil
}

func (sv *HTTPWebTransportServer) handleHTTPAccess(_ *HTTPServer, w http.ResponseWriter, r *http.Request, params map[string]string) bool {
	if r.Method != http.MethodGet {
		//Invalid method
		return false
	}
	if r.URL.Path != sv.webtransportURL {
		//Invalid path
		return false
	}

	//Correct URL and Method
	sv.Logger.Log(1, "Preparing connection from: "+r.RemoteAddr)

	//Verify if connection wants WebTransport
	if !strings.Contains(r.Header.Get("Upgrade"), "websocket") || !strings.Contains(r.Header.Get("Connection"), "Upgrade") {
		http.Error(w, "Invalid WebTransport request", http.StatusBadRequest)
		return false
	}

	//Valid connection
	w.Header().Set("Upgrade", "websocket")
	w.Header().Set("Connection", "Upgrade")

	//Request to switch to Webtransport keep-alive connection
	w.WriteHeader(http.StatusSwitchingProtocols)

	//Hijack connection
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		sv.Logger.Log(3, "Failed to hijact connection from: "+r.RemoteAddr+" | Error: "+err.Error())
		return true
	}

	//Create client
	cl := NewTCPClientSimpleFromConnection(conn.(*net.TCPConn), 0, false, sv.readFuncLocal, sv.reportTraffic)
	cl.SetLogger(sv.Logger)
	cl.Connect()
	//sv.Logger.Log(2, "Connection from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String())
	//go handleTCPReadFramed(conn.(*net.TCPConn), sv.Logger, sv.readFuncLocal)
	return true
}

func (sv *HTTPWebTransportServer) readFuncLocal(client *TCPClientSimple, data []byte, status uint8) {
	var httpConn *HTTPWebTransportServerConn = sv.conns.Get(client)
	if httpConn == nil {
		httpConn = &HTTPWebTransportServerConn{origin: sv, Client: client}
		sv.conns.Set(client, httpConn)
	}
	//Process read
	if sv.readFunc != nil {
		//if status {
		//	sv.Logger.Log(0, "Reading from: "+conn.RemoteAddr().String()+" connected locally to: "+conn.LocalAddr().String()+" | Data lenght: "+strconv.Itoa(len(data))+" | Data in hex: "+hex.EncodeToString(data))
		//}
		sv.readFunc(httpConn, data, status)
	}
}

/*
Writes to Client
*/
//func (sv *HTTPWebTransportServer) WriteToClient(conn *net.TCPConn, data []byte) {
//	writeToTCP(conn, data, sv.Logger)
//}

/*
Starts HTTP Server
*/
func (sv *HTTPWebTransportServer) Start() {
	sv.httpServer.Start()
}

/*
Stops HTTP Server
*/
func (sv *HTTPWebTransportServer) Stop() {
	sv.httpServer.Stop()
}
