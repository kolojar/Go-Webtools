package httptools

import (
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
	"webtools"
)

//go:embed views/WebSocketInstanceServerScript.js
var redirectScriptTemplate string

const invalidWebSocketError = "INVALID_WEB_SOCKET_INSTANCE"

/*
WebSocketInstanceServerInstance is instance of WebSocketInstanceServer
*/
type WebSocketInstanceServerInstance struct {
	owner          *WebSocketInstanceServer
	id             string
	webSocketConns []*WebSocketServerConn
	Parameters     webtools.SafeMap[string, any]
}

/*
GetOwner gets server owner
*/
func (instance *WebSocketInstanceServerInstance) GetOwner() *WebSocketInstanceServer {
	return instance.owner
}

/*
GetID gets ID of instance
*/
func (instance *WebSocketInstanceServerInstance) GetID() string {
	return instance.id
}

/*
FilterConns filters WebSocket connections matching URL parameters
*/
func (instance *WebSocketInstanceServerInstance) FilterConns(filterURLParams map[string]string) []*WebSocketServerConn {
	return FilterWebSocketClients(instance.webSocketConns, filterURLParams)
}

/*
WebSocketInstanceServerReadFunc is function definition for reading data from WebSocketInstanceServer
*/
type WebSocketInstanceServerReadFunc func(instance *WebSocketInstanceServerInstance, conn *WebSocketServerConn, data []byte, status uint8, isBinary bool)

/*
WebSocketInstanceServerAccessFunc is function definition for event when someone wants some resource on server (access), returns true if request was handeled by this method
*/
type WebSocketInstanceServerAccessFunc func(instance *WebSocketInstanceServerInstance, server *Server, w http.ResponseWriter, r *http.Request, params map[string]string) bool

/*
WebSocketInstanceServer is WebSocket server with instancing - users have their own "workspace".
URL "/instanceServerWebsocketNewInstance" is reserved for server communication
*/
type WebSocketInstanceServer struct {
	wsServer                 *WebSocketServer
	instances                webtools.SafeMap[string, *WebSocketInstanceServerInstance]
	id                       string
	readFunc                 WebSocketInstanceServerReadFunc
	accessFunc               WebSocketInstanceServerAccessFunc
	htmlCreatorRedirect      *HTMLCreator
	htmlCreatorRedirectMutex *sync.RWMutex
	htmlRedirectScript       IHTMLElement
}

/*
NewWebSocketInstanceServer creates new WebSocket server with instance support but does not starts it
URL "/instanceServerWebsocketNewInstance" is reserved for server communication -> Sets parameter "nextURLAfterInvalid" on finished setting up
You can use URL "/instanceServerWebsocketNewInstance?action=delete" for instance deletion
*/
func NewWebSocketInstanceServer(address string, readFunc WebSocketInstanceServerReadFunc, accessFunc WebSocketInstanceServerAccessFunc, rootPath string, reportTraffic bool) *WebSocketInstanceServer {
	//Create instance server
	sv := &WebSocketInstanceServer{
		instances:                webtools.MakeSafeMap[string, *WebSocketInstanceServerInstance](),
		id:                       webtools.GenerateRandomID(),
		readFunc:                 readFunc,
		accessFunc:               accessFunc,
		htmlCreatorRedirectMutex: &sync.RWMutex{},
		htmlCreatorRedirect:      NewHTMLCreator(true, "en", "New instance", true),
	}

	//Create HTTP WS server
	sv.wsServer = NewWebSocketServer(address, sv.readFuncLocal, sv.accessFuncLocal, rootPath, reportTraffic)
	sv.wsServer.httpServer.Logger.Prefix = "HTTP-WSInstanceServer"
	//sv.wsServer.AddWebSocketURL("/instanceServerWebsocket", sv.readFuncInstanceManagerLocal)

	//Create redirect HTML
	metaElement := NewHTMLElementBase("meta")
	metaElement.Attributes["name"] = "color-scheme"
	metaElement.Attributes["content"] = "light dark"
	sv.htmlCreatorRedirect.HeadElements = append(sv.htmlCreatorRedirect.HeadElements, metaElement)
	sv.htmlCreatorRedirect.AddBodyElement(NewHTMLElementBaseWithData("pre", "Please wait, you will be redirected in a moment..."))
	sv.htmlRedirectScript = NewHTMLElementBase("script")
	sv.htmlCreatorRedirect.AddBodyElement(sv.htmlRedirectScript)
	return sv
}

/*
GetWSServer gets WebSocket server
*/
func (sv *WebSocketInstanceServer) GetWSServer() *WebSocketServer {
	return sv.wsServer
}

func (sv *WebSocketInstanceServer) checkCookies(serverIDCookie *http.Cookie, instanceIDCookie *http.Cookie) bool {
	if serverIDCookie == nil || instanceIDCookie == nil {
		return false
	}
	if serverIDCookie.Valid() != nil || instanceIDCookie.Valid() != nil {
		return false
	}
	return serverIDCookie.Value == sv.id && sv.instances.Has(instanceIDCookie.Value)
}

func (sv *WebSocketInstanceServer) createNewInstance(requestedURL string, w http.ResponseWriter) {
	//Locks HTML creator
	sv.htmlCreatorRedirectMutex.Lock()

	//Create redirect script
	script := redirectScriptTemplate
	script = strings.Replace(script, "{HREF}", requestedURL, 1)
	sv.htmlRedirectScript.GetElementBase().InnerHTML = script

	//Send HTML
	fmt.Fprint(w, sv.htmlCreatorRedirect.ExportHTML())

	//Unlocks HTML creator
	sv.htmlCreatorRedirectMutex.Unlock()
}

func (sv *WebSocketInstanceServer) readFuncLocal(conn *WebSocketServerConn, data []byte, status uint8, isBinary bool) {
	//Get cookies
	serverIDCookie := conn.GetCookie("instanceServerUniqueId")
	instanceIDCookie := conn.GetCookie("instanceServerInstanceId")

	//Check cookies
	if !sv.checkCookies(serverIDCookie, instanceIDCookie) {
		//Invalid
		conn.Send([]byte(invalidWebSocketError))
		conn.Close()
		return
	}

	//Get instance and parse to read function
	instance := sv.instances.Get(instanceIDCookie.Value)
	if !slices.Contains(instance.webSocketConns, conn) {
		instance.webSocketConns = append(instance.webSocketConns, conn)
	}
	if sv.readFunc != nil {
		sv.readFunc(instance, conn, data, status, isBinary)
	}
}

func (sv *WebSocketInstanceServer) accessFuncLocal(server *Server, w http.ResponseWriter, r *http.Request, params map[string]string) bool {
	//Get cookies
	serverIDCookie, err1 := r.Cookie("instanceServerUniqueId")
	instanceIDCookie, err2 := r.Cookie("instanceServerInstanceId")
	if r.URL.Path == "/instanceServerWebsocketNewInstance" && r.Method == http.MethodPost {
		//Generate new ID
		id := webtools.GenerateRandomID()
		sv.wsServer.httpServer.Logger.Log(2, "Creating new instance for: "+r.RemoteAddr+" with id: "+id)

		//Set cookies
		http.SetCookie(w, &http.Cookie{
			Name:     "instanceServerUniqueId",
			Value:    sv.id,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   0,
			SameSite: http.SameSiteLaxMode,
		})
		http.SetCookie(w, &http.Cookie{
			Name:     "instanceServerInstanceId",
			Value:    id,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   0,
			SameSite: http.SameSiteLaxMode,
		})
		time.Sleep(time.Second)
		instance := &WebSocketInstanceServerInstance{Parameters: webtools.MakeSafeMap[string, any](), id: id, owner: sv, webSocketConns: make([]*WebSocketServerConn, 0)}

		//Read URL
		nextURL, err := io.ReadAll(r.Body)
		if err == nil {
			instance.Parameters.Set("nextURLAfterInvalid", string(nextURL))
		}
		sv.instances.Set(id, instance)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, "done")
		return true
	}

	//Check cookies
	if err1 != nil || err2 != nil {
		//Invalid
		sv.createNewInstance(CreateURLFromParameters(r.URL.Path, params), w)
		return true
	}
	if !sv.checkCookies(serverIDCookie, instanceIDCookie) {
		//Invalid
		sv.createNewInstance(CreateURLFromParameters(r.URL.Path, params), w)
		return true
	}

	if r.URL.Path == "/instanceServerWebsocketNewInstance" && r.Method == http.MethodGet && params["action"] == "delete" {
		for _, v := range sv.instances.Get(instanceIDCookie.Value).webSocketConns {
			v.Close()
		}
		sv.instances.Delete(instanceIDCookie.Value)
		sv.wsServer.httpServer.Logger.Log(2, "Removed instance for: "+r.RemoteAddr+" with id: "+instanceIDCookie.Value)
		fmt.Fprint(w, "Instance removed")
		return true
	}

	//Pass to accessFunc
	if sv.accessFunc != nil {
		return sv.accessFunc(sv.instances.Get(instanceIDCookie.Value), server, w, r, params)
	}
	return false
}

/*
Start starts WebSocket HTTP Instance Server. Locks execution thread
*/
func (sv *WebSocketInstanceServer) Start() {
	sv.wsServer.Start()
}

/*
Stop stops WebSocket HTTP Instance Server
*/
func (sv *WebSocketInstanceServer) Stop() {
	sv.wsServer.Stop()
}
