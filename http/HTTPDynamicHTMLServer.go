package http

import (
	_ "embed"
	"fmt"
	"net/http"
	"strings"
	"webtools"
)

//go:embed views/htmlDynamicCreator.js
var dynamicScript string

/*
DynamicEventSetInnerHTML is event for seting inner HTML of some object
*/
const DynamicEventSetInnerHTML = "setInnerHTML"

/*
DynamicHTMLServerOnInstanceChangeFunc is function definition for informing about instance change or new data from instace for DynamicHTMLServer
Status is same as status of WebSocket read
*/
type DynamicHTMLServerOnInstanceChangeFunc func(server *DynamicHTMLServer, instanceConn *WebSocketServerConn, status uint8, values map[string]string)

/*
DynamicHTMLServer is WebSocket server for interactive Websides
*/
type DynamicHTMLServer struct {
	pages                webtools.SafeMap[string, *HTMLCreator]
	websocketServer      *WebSocketServer
	instances            webtools.SafeMap[*WebSocketServerConn, map[string]string]
	onAccessFunc         AccessFunc
	onInstanceChangeFunc DynamicHTMLServerOnInstanceChangeFunc
}

/*
IsAlive gets if server is alive
*/
func (sv *DynamicHTMLServer) IsAlive() bool {
	return sv.websocketServer.IsAlive()
}

/*
GetAddress gets address of server
*/
func (sv *DynamicHTMLServer) GetAddress() string {
	return sv.websocketServer.GetAddress()
}

/*
GetWebSocketServer gets WebSocket server
*/
func (sv *DynamicHTMLServer) GetWebSocketServer() *WebSocketServer {
	return sv.websocketServer
}

/*
GetInstanceValues gets instance values
*/
func (sv *DynamicHTMLServer) GetInstanceValues(instance *WebSocketServerConn) map[string]string {
	return sv.instances.Get(instance)
}

/*
AddPage adds page
*/
func (sv *DynamicHTMLServer) AddPage(url string, page *HTMLCreator) {
	page.MoveScriptsToEnd = true
	script := NewHTMLElementBase("script")
	script.InnerHTML = dynamicScript
	page.AddBodyElement(script)
	sv.pages.Set(url, page)
}

/*
NewDynamicHTMLServer creates new HTTP WebSocket Server with dynamic pages support using DynamicObjects and HTMLCreator but does not starts it
This readFunc is asociated with "/websocket" url
Server creates own "/dynamicHTML" for events for this dynamic events
Keep in mind that default file requests have priority
*/
func NewDynamicHTMLServer(address string, readFunc WebSocketServerReadFunc, onAccessFunc AccessFunc, onInstanceChangeFunc DynamicHTMLServerOnInstanceChangeFunc, rootPath string, reportTraffic bool) *DynamicHTMLServer {
	sv := &DynamicHTMLServer{pages: webtools.MakeSafeMap[string, *HTMLCreator](), onAccessFunc: onAccessFunc, onInstanceChangeFunc: onInstanceChangeFunc, instances: webtools.MakeSafeMap[*WebSocketServerConn, map[string]string]()}
	sv.websocketServer = NewWebSocketServer(address, readFunc, sv.onAccessFuncLocal, rootPath, reportTraffic)
	sv.websocketServer.Logger.Prefix = "DynamicHTML - " + sv.websocketServer.Logger.Prefix
	sv.websocketServer.AddWebSocketURL("/dynamicHTML", sv.readFuncLocal)
	return sv
}

func (sv *DynamicHTMLServer) onAccessFuncLocal(server *Server, w http.ResponseWriter, r *http.Request, params map[string]string) bool {
	//Try to handle page from generator
	page := sv.pages.Get(r.URL.Path)
	if page != nil {
		fmt.Fprint(w, sv.pages.Get(r.URL.Path).ExportHTML())
		return true
	}
	//Fallback to access func
	if sv.onAccessFunc != nil {
		return sv.onAccessFunc(server, w, r, params)
	}
	return false
}

func (sv *DynamicHTMLServer) readFuncLocal(conn *WebSocketServerConn, data []byte, status uint8, _ bool) {
	if status == webtools.FinishedReadFuncStatus {
		return
	}
	if status == webtools.ConnectStatus {
		//New connection from WebSocket = new instance
		sv.instances.Set(conn, map[string]string{})
		if sv.onInstanceChangeFunc != nil {
			sv.onInstanceChangeFunc(sv, conn, status, sv.instances.Get(conn))
		}
		return
	}
	if status == webtools.DisconnectStatus {
		//Closing connection from WebSocket = delete instance
		get := sv.instances.Get(conn)
		sv.instances.Delete(conn)
		if sv.onInstanceChangeFunc != nil {
			sv.onInstanceChangeFunc(sv, conn, status, get)
		}
		return
	}
	//Read data - Message format: ID, operation, data
	split := strings.SplitN(string(data), ";", 3)
	if len(split) != 3 {
		conn.origin.Logger.Log(3, "Invalid source data: "+string(data))
		return
	}

	//Operation sorter
	panic("Operation not implemented " + split[1])
}

/*
Start starts Dynamic HTML server, locks execution thread
*/
func (sv *DynamicHTMLServer) Start() {
	sv.websocketServer.Start()
}

/*
Stop stops Dynamic HTML server
*/
func (sv *DynamicHTMLServer) Stop() {
	sv.websocketServer.Stop()
}

/*
MakeDynamicElement makes specified element dynamic, so it can interact with DynamicHTMLServer and responce to events, IDs can repeat.
Some actions can return values, so keep in mind that if more elements, that return values, can override themselfs
*/
func (sv *DynamicHTMLServer) MakeDynamicElement(element IHTMLElement, ID string) bool {
	if ID == "" {
		return false
	}
	element.GetElementBase().Attributes["dynamic-html-id"] = ID
	return true
}

/*
SendEvent sends event to HTML
*/
func (sv *DynamicHTMLServer) SendEvent(instance *WebSocketServerConn, targetID string, eventType string, eventData string) bool {
	if !sv.instances.Has(instance) {
		return false
	}
	instance.Send([]byte(targetID + "&" + eventType + "&" + eventData))
	return true
}
