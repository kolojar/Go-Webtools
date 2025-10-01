package httpTools

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"webtools"
)

/*
Standardized type of function
*HTTPServer = Server
http.ResponseWriter = Responce
*http.Request = Request
params = map[string]string
Returns bool = got handled
*/
type HTTPAccessFunc func(server *HTTPServer, w http.ResponseWriter, r *http.Request, params map[string]string) bool

var iNVALID_NAMES = [...]string{"..", "."}

/*
Checks if path constains some invalid names (server protection) -> Returns TRUE if value is INVALID
! Must be present in every operation with files on server !
*/
func CheckInvalidNames(path string) error {
	if !strings.HasPrefix(path, "/") {
		return errors.New("path does not have prefix")
	}
	split := strings.Split(path, "/")
	for i := 0; i < len(iNVALID_NAMES); i++ {
		for k := 0; k < len(split); k++ {
			if strings.EqualFold(split[k], iNVALID_NAMES[i]) {
				return errors.New("Found invalid name: " + iNVALID_NAMES[i] + " in: " + path)
			}
		}
	}
	return nil
}

/*
Struct of HTTP server
*/
type HTTPServer struct {
	// Key is url on server and value is real path in file system, they are not relative to rootPath. They are handeled automatically
	HostPaths map[string]string
	//This path is not handeled automatically
	rootPath            string
	address             string
	Logger              *webtools.ConsoleLogger
	server              http.Server
	onAccessFunc        HTTPAccessFunc
	startWebBrowser     bool
	isAlive             bool
	UseDirectoryListing bool
}

func (sv *HTTPServer) GetRootPath() string {
	return sv.rootPath
}

func (sv *HTTPServer) IsAlive() bool {
	return sv.isAlive
}

func (sv *HTTPServer) GetAddress() string {
	return sv.address
}

/*
Replaces double slashes with one
*/
func TidyURLPath(url string) string {
	originalLenght := len(url)
	url = strings.ReplaceAll(url, "//", "/")
	if originalLenght != len(url) {
		return TidyURLPath(url)
	}
	return url
}

/*
Creates new HTTP server but does not starts it. Adds new host path to HTTP server (used for shared scripts, css, images)
*/
func NewHTTPServer(address string, onAccessFunc HTTPAccessFunc, rootPath string, startWebBrowser bool) *HTTPServer {
	return &HTTPServer{address: address, HostPaths: map[string]string{}, Logger: webtools.NewConsoleLogger("HTTPServer", 0), onAccessFunc: onAccessFunc, startWebBrowser: startWebBrowser, rootPath: rootPath}
}

/*
Launches HTTP server on specified address. Locks execution thread
*/
func (sv *HTTPServer) Start() {
	if sv.isAlive {
		return
	}

	sv.server = http.Server{Addr: sv.address, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sv.httpHandler(w, r)
	})}
	sv.Logger.Log(2, "Started listening on: "+sv.address)
	sv.isAlive = true
	err := sv.server.ListenAndServe()
	sv.isAlive = false
	if err != nil {
		sv.Logger.Log(3, "Error listening on: "+sv.address+" | Error: "+err.Error())
	}
	sv.Logger.Log(2, "Stopped listening on: "+sv.address)
}

/*
Resolves relative urls for HTTP server to real OS FileSystem path
Returns list of real urls
*/
func (sv *HTTPServer) ResolvePath(url string) []string {
	result := make([]string, 0)
	for k, v := range sv.HostPaths {
		//Sort out hostPaths
		if strings.HasPrefix(url, k) {
			if !strings.HasSuffix(v, "/") {
				v += "/"
			}
			result = append(result, TidyURLPath(strings.Replace(url, k, v, 1)))
		}
	}
	result = append(result, TidyURLPath(strings.Replace(url, "/", sv.rootPath, 1)))
	return result
}

/*
Handles and sorts HTTP requests
*/
func (sv *HTTPServer) httpHandler(w http.ResponseWriter, r *http.Request) {
	sv.Logger.Log(1, r.RemoteAddr+" - "+r.Method+" - "+r.URL.String())
	//Check name
	err2 := CheckInvalidNames(r.URL.Path)
	if err2 != nil {
		sv.Logger.Log(3, "Error in request: "+r.URL.Path+" | Error: "+err2.Error())
		http.Error(w, "Invalid request", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		urls := sv.ResolvePath(r.URL.Path)
		for i := 0; i < len(urls); i++ {
			//Sort out urls
			url := urls[i]
			err := HandleHTTPGet(w, r, url, sv)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				//Invalid error
				sv.Logger.Log(3, "Error in GET request for: "+r.URL.Path+" | Error: "+err.Error())
				http.Error(w, "Invalid request", http.StatusInternalServerError)
				return
			}
			if err == nil {
				//Get OK
				return
			}
		}
	}
	if sv.onAccessFunc != nil {
		_, params := CreateParametersFromURL(r.URL.RawQuery)
		if sv.onAccessFunc(sv, w, r, params) {
			return
		}
	}

	// Not found
	sv.Logger.Log(3, "NOT FOUND - "+r.RemoteAddr+" - "+r.Method+" - "+r.URL.String())
	http.NotFound(w, r)
}

/*
Reads file contents
Returns data, isDirectory, error
*/
func ReadFile(filePath string) ([]byte, bool, error) {
	// Check file exists
	stat, err2 := os.Stat(filePath)
	if errors.Is(err2, os.ErrNotExist) {
		return nil, false, err2
	}
	if stat == nil {
		return nil, false, os.ErrNotExist
	}

	// Check for dir
	if stat.IsDir() {
		return nil, true, nil
	}

	// Read data
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false, err
	}

	return data, false, nil
}

/*
Reads file contents as string
Returns data, isDirectory, error
*/
func ReadFileString(filePath string) (string, bool, error) {
	data, isDir, err := ReadFile(filePath)
	if err != nil {
		return "", isDir, err
	} else {
		return string(data), isDir, nil
	}
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
Tries to handle file or folder request
*/
func TryHandleHTTPFile(w http.ResponseWriter, filePath string, contentType string, urlPath string, sv *HTTPServer) error {
	//Read data
	data, isDir, err := ReadFileString(filePath)
	if err != nil {
		return err
	}

	// Check dir
	if isDir {
		if sv != nil && sv.UseDirectoryListing {
			HandleDirectoryListingHTTP(w, filePath, urlPath, sv)
		} else {
			//http.Error(w, "Directory listing not supported.", http.StatusForbidden)
			//return errors.New("directory listing not supported")
			return os.ErrNotExist
		}
		return nil
	}

	//Send data
	w.Header().Add("Content-Type", contentType)
	fmt.Fprint(w, data)
	return nil
}

func SortHTTPContentType(path string) string {
	contentType := "text/html"
	if strings.HasSuffix(path, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(path, ".js") {
		contentType = "text/javascript"
	} else if strings.HasSuffix(path, ".map") {
		contentType = "text/json" // JS Map
	} else if strings.HasSuffix(path, ".ts") {
		contentType = "text/x.typescript"
	} else if strings.HasSuffix(path, ".svg") {
		contentType = "image/svg+xml"
	}
	return contentType
}

/*
Handles directory access get request relative to HTTP server root
*/
func (sv *HTTPServer) TryHandleHTTPFileRelative(w http.ResponseWriter, r *http.Request, getPath string) error {
	//Check invalid names
	err := CheckInvalidNames(getPath)
	if err != nil {
		return err
	}
	return TryHandleHTTPFile(w, JoinPaths(sv.rootPath, getPath), SortHTTPContentType(getPath), getPath, sv)
}

/*
Handles directory access get request
*/
func HandleHTTPGet(w http.ResponseWriter, r *http.Request, realPath string, sv *HTTPServer) error {
	return TryHandleHTTPFile(w, realPath, SortHTTPContentType(r.URL.Path), r.URL.Path, sv)
}

/*
Handles directory access GET request relative to HTTP server
*/
func (sv *HTTPServer) HandleHTTPGetRelative(w http.ResponseWriter, r *http.Request) bool {
	urls := sv.ResolvePath(r.URL.Path)
	for i := 0; i < len(urls); i++ {
		//Handle each url
		url := urls[i]
		err := HandleHTTPGet(w, r, url, sv)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			//Invalid error
			sv.Logger.Log(3, "Error in GET request for: "+r.URL.Path+" | Error: "+err.Error())
			http.Error(w, "Invalid request", http.StatusInternalServerError)
			return false
		}
		if err == nil {
			//Get OK
			return true
		}

	}
	return false
}

/*
Handles file read relative to HTTP server root
Returns data, isDir, error
*/
func (sv *HTTPServer) ReadFileRelative(path string) ([]byte, bool, error) {
	return ReadFile(JoinPaths(sv.rootPath, path))
}

/*
Creates map from url parameters
*/
func CreateParametersFromURL(text string) (string, map[string]string) {
	// Split & parts
	dataSplit := strings.Split(text, "&")
	postArguments := map[string]string{}

	// Go trought all of them
	for i := 0; i < len(dataSplit); i++ {
		// Split by "=" and unescape
		split := strings.SplitN(dataSplit[i], "=", 2)
		unescapeKey, _ := url.QueryUnescape(split[0])
		if len(split) == 1 {
			postArguments[unescapeKey] = ""
		} else if len(split) >= 2 {
			unescapeValue, _ := url.QueryUnescape(split[1])
			postArguments[unescapeKey] = unescapeValue
		}

	}
	return dataSplit[0], postArguments
}

/*
Creates url like parameters from map
*/
func CreateURLFromParameters(preURL string, params map[string]string) string {
	result := preURL + "&"
	// Go trought all parameters in map and escape them
	for k, v := range params {
		result += url.QueryEscape(k) + "=" + url.QueryEscape(v) + "&"
	}
	result = strings.TrimSuffix(result, "&")
	result = strings.TrimPrefix(result, "&")
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
		sv.Logger.Log(3, "Error stopping: "+err.Error())
	}
}
