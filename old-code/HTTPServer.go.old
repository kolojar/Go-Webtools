package webtools

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

/*
Handles HTML file with not internal editing: ReadFile -> Send responce
*/
func HandleBasicHTML(w http.ResponseWriter, r *http.Request, path string) {
	//Read site file and return error if happens
	text, err := ReadSiteFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	//Process site
	fmt.Fprint(w, text)
}

/*
Reads file contents
*/
func ReadSiteFile(filePath string) (string, error) {
	//Check file exists
	_, err2 := os.Stat(filePath)
	if errors.Is(err2, os.ErrNotExist) {
		//println("File not exists!")
		return "", err2
	}

	//Read data
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

/*
Handles HTML file with not internal editing: ReadFile -> Send responce; Get file on requested URL - searches in main data folder and in shared views folder
*/
func (sv *HTTPServer) HandleBasicHTMLRelative(w http.ResponseWriter, r *http.Request, pathURL string) {
	//Read site file and return error if happens
	text, err := sv.ReadSiteFileRelative(pathURL)
	if err != nil {
		text, err = sv.ReadSiteFileRelative(pathURL + ".html")
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}

	//Process site
	fmt.Fprint(w, text)
}

/*
Reads file contents based on requested URL - searches in main data folder and in shared views folder
*/
func (sv *HTTPServer) ReadSiteFileRelative(filePathURL string) (string, error) {
	//Found view in dataPath (main)
	data, err := ReadSiteFile(sv.dataPathPrefix + filePathURL)
	if err == nil {
		return data, nil
	}

	//Not found in main, looking in shared if posssible
	if sv.sharedDataPathPrefix != "" {
		data, err = ReadSiteFile(sv.sharedDataPathPrefix + filePathURL)
		if err == nil {
			return data, nil
		}
		//Not found in shared, looking in edited shared if posssible
		editSharedPath := strings.TrimPrefix(sv.sharedDataPathPrefix, "./")
		if editSharedPath != "" {
			data, err = ReadSiteFile(sv.sharedDataPathPrefix + strings.Replace(filePathURL, editSharedPath, "", 1))
		}
	}

	return data, err
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

/* Standardized type of function */
type funcViews func(http.ResponseWriter, *http.Request, map[string]string)

/*
Basic HTTP server
*/
type HTTPServer struct {
	address              string
	dataPathPrefix       string
	sharedDataPathPrefix string
	getViewsFunc         funcViews
	postViewsFunc        funcViews
	startWebBrowser      bool
	Logger               ConsoleLogger
	httpServer           http.Server
	isAlive              bool
}

/*
Returns address of running server
*/
func (sv *HTTPServer) GetAddress() string {
	return sv.address
}

/*
Returns if server is running
*/
func (sv *HTTPServer) IsAlive() bool {
	return sv.isAlive
}

/*
Constructs new instance of HTTP Server but does not start it
*/
func MakeHTTPServer(address string, dataPathPrefix string, sharedDataPathPrefix string, getViewsFunc funcViews, postViewsFunc funcViews, startWebBrowser bool) HTTPServer {
	return HTTPServer{address: address, dataPathPrefix: dataPathPrefix, getViewsFunc: getViewsFunc, postViewsFunc: postViewsFunc, startWebBrowser: startWebBrowser, Logger: MakeConsoleLogger("HTTPServer"), sharedDataPathPrefix: sharedDataPathPrefix}
}

/*
Handles and sorts HTTP requests
*/
func (sv *HTTPServer) httpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		//Sort GET
		sv.Logger.Log(1, "GET: "+r.URL.Path)

		//Sort SSE
		/*if r.URL.Path == "/sse" {
			sv.handleSSE(w, CreateParametersFromURL(r.URL.RawQuery))
			return
		}*/

		//Return if no valid function found
		if sv.getViewsFunc == nil {
			http.NotFound(w, r)
			sv.Logger.Log(2, "GET - Not found: "+r.URL.Path)
			return
		}

		//Get CSS
		if strings.HasSuffix(r.URL.Path, ".css") {
			//Sends CSS using type text/css
			css, err := sv.ReadSiteFileRelative(r.URL.Path)
			if err != nil {
				http.NotFound(w, r)
				sv.Logger.Log(2, "GET - Not found: "+r.URL.Path)
				return
			}
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, css)
			return
		}

		//Get JS
		if strings.HasSuffix(r.URL.Path, ".js") {
			//Sends Javascript using type text/javascript
			js, err := sv.ReadSiteFileRelative(r.URL.Path)
			if err != nil {
				http.NotFound(w, r)
				sv.Logger.Log(2, "GET - Not found: "+r.URL.Path)
				return
			}
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, js)
			return
		}

		//Get JS Map
		if strings.HasSuffix(r.URL.Path, ".map") {
			//Sends Javascript using type text/javascript
			js, err := sv.ReadSiteFileRelative(r.URL.Path)
			if err != nil {
				http.NotFound(w, r)
				sv.Logger.Log(2, "GET - Not found: "+r.URL.Path)
				return
			}
			w.Header().Set("Content-Type", "text/json")
			fmt.Fprint(w, js)
			return
		}

		//Get TS
		if strings.HasSuffix(r.URL.Path, ".ts") {
			//Sends Javascript using type text/javascript
			js, err := sv.ReadSiteFileRelative(r.URL.Path)
			if err != nil {
				http.NotFound(w, r)
				sv.Logger.Log(2, "GET - Not found: "+r.URL.Path)
				return
			}
			w.Header().Set("Content-Type", "text/x.typescript")
			fmt.Fprint(w, js)
			return
		}

		//Get SVG
		if strings.HasSuffix(r.URL.Path, ".svg") {
			//Sends SVG image using type image/svg+xml
			js, err := sv.ReadSiteFileRelative(r.URL.Path)
			if err != nil {
				http.NotFound(w, r)
				sv.Logger.Log(2, "GET - Not found: "+r.URL.Path)
				return
			}
			w.Header().Set("Content-Type", "image/svg+xml")
			fmt.Fprint(w, js)
			return
		}
		sv.getViewsFunc(w, r, CreateParametersFromURL(r.URL.RawQuery))
	} else {
		// Return if no valid function found
		if sv.postViewsFunc == nil {
			http.NotFound(w, r)
			sv.Logger.Log(2, "POST - Not found: "+r.URL.Path)
			return
		}

		//Read post dataPost
		dataPost, err := io.ReadAll(r.Body)
		if err != nil {
			//Return error
			sv.Logger.Log(2, "POST - Not found: "+r.URL.Path)
			http.NotFound(w, r)
			return
		}

		//Unpack post data
		sv.Logger.Log(1, "POST: "+r.URL.Path+"; DATA: "+string(dataPost))
		sv.postViewsFunc(w, r, CreateParametersFromURL(string(dataPost)))
	}
}

/*
Starts WebBrowser on any system with specified address
*/
func StartWebBrowser(address string, logger *ConsoleLogger) {
	logger.Log(2, "Starting web browser at address: "+address)
	time.Sleep(1000 * time.Microsecond)

	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	default:
		cmd = "xdg-open"
	}
	args = append(args, address)
	exec.Command(cmd, args...).Start()
	logger.Log(1, "Opened web browser!")
}

/*
Launches HTTP server on specified address.
*/
func (sv *HTTPServer) Start() {
	//Create HTTP server and connects functions to handler
	sv.httpServer = http.Server{
		Addr: sv.address,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sv.httpHandler(w, r)
		}),
	}

	//Starts WebBrowser if needed
	sv.Logger.Log(2, "Started HTTP server at "+sv.address)
	if sv.startWebBrowser {
		go StartWebBrowser("http://"+sv.address, &sv.Logger)
	}

	//Start listening
	sv.isAlive = true
	err := sv.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		sv.Logger.Log(3, "Exited with this error: "+err.Error())
	}
	sv.isAlive = false
	sv.Logger.Log(2, "Stopped!")
}

func (sv *HTTPServer) Stop() {
	if !sv.IsAlive() {
		return
	}
	err := sv.httpServer.Close()
	if err != nil {
		sv.Logger.Log(3, "Error stopping HTTP server: "+err.Error())
		return
	}
	//sv.Logger.Log(2, "Stopped!")
}

/*
Creates SSE JSON
*/
func CreateJSONFromParameters(params map[string]string) string {
	data, _ := json.Marshal(params)
	return string(data)
}
