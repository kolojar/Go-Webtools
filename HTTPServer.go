package webtools

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

/*
Standardized type of function
*HTTPServer = Server
http.ResponseWriter = Responce
*http.Request = Request
params = map[string]string
Returns bool = got handled
*/
type HTTPAccessFunc func(*HTTPServer, http.ResponseWriter, *http.Request, map[string]string) bool

/*
Struct of HTTP server
*/
type HTTPServer struct {
	//Key is url on server and value is real path in file system, they are not relative to rootPath. They are handeled automatically
	HostPaths map[string]string
	//This path is not handeled automatically
	rootPath        string
	address         string
	logger          ConsoleLogger
	server          http.Server
	onAccessFunc    HTTPAccessFunc
	startWebBrowser bool
	isAlive         bool
}

/*
Creates new HTTP server but does not starts it. Adds new host path to HTTP server (used for shared scripts, css, images)
*/
func NewHTTPServer(address string, onAccessFunc HTTPAccessFunc, rootPath string, startWebBrowser bool) *HTTPServer {
	return &HTTPServer{address: address, HostPaths: map[string]string{}, logger: MakeConsoleLogger("HTTPServer", 0), onAccessFunc: onAccessFunc, startWebBrowser: startWebBrowser, rootPath: rootPath}
}

/*
Launches HTTP server on specified address.
*/
func (sv *HTTPServer) Start() {
	if sv.isAlive {
		return
	}

	sv.server = http.Server{Addr: sv.address, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sv.httpHandler(w, r)
	})}
	sv.logger.Log(2, "Started listening on: "+sv.address)
	sv.isAlive = true
	err := sv.server.ListenAndServe()
	sv.isAlive = false
	if err != nil {
		sv.logger.Log(3, "Error listening on: "+sv.address+" | Error: "+err.Error())
	}
	sv.logger.Log(2, "Stopped listening on: "+sv.address)
}

/*
Handles and sorts HTTP requests
*/
func (sv *HTTPServer) httpHandler(w http.ResponseWriter, r *http.Request) {
	sv.logger.Log(1, r.RemoteAddr+" - "+r.Method+" - "+r.URL.String())
	if r.Method == http.MethodGet {
		for k, v := range sv.HostPaths {
			//Sort out hostPaths
			if strings.HasPrefix(r.URL.Path, k) {
				err := HandleHTTPDirectoryGet(w, r, v, strings.TrimPrefix(r.URL.Path, k))
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					//Invalid error
					sv.logger.Log(3, "Error in GET request for: "+r.URL.Path+" | Error: "+err.Error())
					return
				}
				if err == nil {
					//Get OK
					return
				}
			}
		}
	}
	if sv.onAccessFunc != nil {
		if sv.onAccessFunc(sv, w, r, CreateParametersFromURL(r.URL.RawQuery)) {
			return
		}
	}

	//Not found
	sv.logger.Log(3, "NOT FOUND - "+r.RemoteAddr+" - "+r.Method+" - "+r.URL.String())
}

/*
Reads file contents
*/
func ReadFile(filePath string) ([]byte, error) {
	//Check file exists
	_, err2 := os.Stat(filePath)
	if errors.Is(err2, os.ErrNotExist) {
		return nil, err2
	}

	//Read data
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return data, nil
}

/*
Joins 2 paths together
*/
func JoinPaths(path1 string, path2 string) string {
	result := strings.TrimSuffix(path1, "/")
	if !strings.HasPrefix(path2, "/") {
		result += "/"
	}
	result += path2
	return result
}

/*
Reads file contents
*/
func TryHandleHTTPFile(w http.ResponseWriter, filePath string, contentType string) error {
	//Read data
	data, err := ReadFile(filePath)
	if err != nil {
		return err
	}

	//Send data
	fmt.Fprint(w, string(data))
	return nil
}

func SortHTTPContentType(path string) string {
	contentType := "text/html"
	if strings.HasSuffix(path, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(path, ".js") {
		contentType = "text/javascript"
	} else if strings.HasSuffix(path, ".map") {
		contentType = "text/json" //JS Map
	} else if strings.HasSuffix(path, ".ts") {
		contentType = "text/x.typescript"
	} else if strings.HasSuffix(path, ".svg") {
		contentType = "image/svg+xml"
	}
	return contentType
}

/*
Handles directory access get request
*/
func HandleHTTPDirectoryGet(w http.ResponseWriter, r *http.Request, rootPath string, getPath string) error {
	return TryHandleHTTPFile(w, JoinPaths(rootPath, getPath), SortHTTPContentType(r.URL.Path))
}

/*
Handles directory access get request relative to HTTP server root
*/
func (sv *HTTPServer) HandleHTTPDirectoryGetRelative(w http.ResponseWriter, r *http.Request) error {
	return HandleHTTPDirectoryGet(w, r, JoinPaths(sv.rootPath, r.URL.Path), r.URL.Path)
}

/*
Handles fire read relative to HTTP server root
*/
func (sv *HTTPServer) ReadFileRelative(path string) ([]byte, error) {
	return ReadFile(JoinPaths(sv.rootPath, path))
}

/*
Creates map from url parameters
*/
func CreateParametersFromURL(text string) map[string]string {
	//Split & parts
	dataSplit := strings.Split(text, "&")
	postArguments := map[string]string{}

	//Go trought all of them
	for i := 0; i < len(dataSplit); i++ {
		//Split by "=" and unescape
		split := strings.SplitN(dataSplit[i], "=", 2)
		unescapeKey, _ := url.QueryUnescape(split[0])
		if len(split) == 1 {
			postArguments[unescapeKey] = ""
		} else if len(split) >= 2 {
			unescapeValue, _ := url.QueryUnescape(split[1])
			postArguments[unescapeKey] = unescapeValue
		}

	}
	return postArguments
}

/*
Creates url like parameters from map
*/
func CreateURLFromParameters(params map[string]string) string {
	result := ""
	//Go trought all parameters in map and escape them
	for k, v := range params {
		result += url.QueryEscape(k) + "=" + url.QueryEscape(v) + "&"
	}
	result = strings.TrimSuffix(result, "&")
	return result
}

/*
Stops HTTP server
*/
func (sv *HTTPServer) Stop() {
	if !sv.isAlive {
		return
	}
	err := sv.server.Close()
	if err != nil {
		sv.logger.Log(3, "Error stopping: "+err.Error())
	}
}
