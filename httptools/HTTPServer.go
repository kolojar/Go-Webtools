package httptools

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/kolojar/Go-Webtools/helpertools"
)

/*
AccessFunc is function definition for event when someone wants some resource on server (access), returns true if request was handeled by this method
*/
type AccessFunc func(server *Server, w http.ResponseWriter, r *http.Request, params map[string]string) bool

var invalidNames = [...]string{"..", "."}

/*
CheckInvalidNames checks if path contains some invalid names (server protection) -> Returns TRUE if value is INVALID
! Must be present in every operation with files on server !
*/
func CheckInvalidNames(path string) error {
	if !strings.HasPrefix(path, "/") {
		return errors.New("path does not have prefix")
	}
	split := strings.Split(path, "/")
	for i := 0; i < len(invalidNames); i++ {
		for k := 0; k < len(split); k++ {
			if strings.EqualFold(split[k], invalidNames[i]) {
				return errors.New("Found invalid name: " + invalidNames[i] + " in: " + path)
			}
		}
	}
	return nil
}

/*
Server is struct of HTTP server
*/
type Server struct {
	// Key is url on server and value is real path in file system, they are not relative to rootPath. They are handeled automatically
	HostPaths map[string]string
	//This path is not handeled automatically
	rootPath            string
	address             string
	Logger              *helpertools.ConsoleLogger
	server              http.Server
	onAccessFunc        AccessFunc
	startWebBrowser     bool
	isAlive             bool
	UseDirectoryListing bool
	// Key is ErrorCode and value is path to errorPage, it is not relative to rootPath, error is placed in {ERROR}
	ErrorPages map[int]string
	// Any extension (.html, .css, ...) set here will be processed before calling onAccessFunc
	HandleExtensionsWithPriority []string
}

/*
GetRootPath gets root path of HTTP server
*/
func (sv *Server) GetRootPath() string {
	return sv.rootPath
}

/*
IsAlive gets if server is alive
*/
func (sv *Server) IsAlive() bool {
	return sv.isAlive
}

/*
GetAddress gets address of server
*/
func (sv *Server) GetAddress() string {
	return sv.address
}

/*
TidyURLPath replaces double slashes with one
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
NewServer creates new HTTP server but does not starts it. Adds new host path to HTTP server (used for shared scripts, css, images)
Set another host paths using HostPaths and own error sites using ErrorPages
*/
func NewServer(address string, onAccessFunc AccessFunc, rootPath string, startWebBrowser bool, reportHTTPTraffic bool) *Server {
	if !strings.HasSuffix(rootPath, "/") {
		rootPath += "/"
	}
	return &Server{address: address, ErrorPages: map[int]string{}, HostPaths: map[string]string{}, Logger: helpertools.NewConsoleLoggerForTraffic("HTTPServer", reportHTTPTraffic), onAccessFunc: onAccessFunc, startWebBrowser: startWebBrowser, rootPath: rootPath, HandleExtensionsWithPriority: make([]string, 0)}
}

/*
Start starts HTTP server on specified address. Locks execution thread
*/
func (sv *Server) Start() {
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
ResolvePath resolves relative urls for HTTP server to real OS FileSystem path
Returns list of real urls
*/
func (sv *Server) ResolvePath(url string) []string {
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

func (sv *Server) checkPriorityExtension(url string) bool {
	for _, val := range sv.HandleExtensionsWithPriority {
		if strings.HasSuffix(url, val) {
			return true
		}
	}
	return false
}

/*
Handles and sorts HTTP requests
*/
func (sv *Server) httpHandler(w http.ResponseWriter, r *http.Request) {
	sv.Logger.Log(0, r.RemoteAddr+" - "+r.Method+" - "+r.URL.String())
	//Check name
	err2 := CheckInvalidNames(r.URL.Path)
	if err2 != nil {
		sv.Logger.Log(3, "Error in request: "+r.URL.Path+" | Error: "+err2.Error())
		sv.HandleError(w, "Invalid request", http.StatusInternalServerError)
		return
	}

	//Try GET method for priority
	if r.Method == http.MethodGet {
		if sv.checkPriorityExtension(r.URL.Path) {
			//File has priority
			urls := sv.ResolvePath(r.URL.Path)
			for i := 0; i < len(urls); i++ {
				//Sort out urls
				url := urls[i]
				err := HandleHTTPGet(w, r, url, sv)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					//Invalid error
					sv.Logger.Log(3, "Error in GET request for: "+r.URL.Path+" | Error: "+err.Error())
					sv.HandleError(w, "Invalid request", http.StatusInternalServerError)
					return
				}
				if err == nil {
					//Get OK
					return
				}
			}
		}
	}

	//Try access func
	if sv.onAccessFunc != nil {
		params := CreateParametersFromQuery(r.URL.RawQuery)
		if sv.onAccessFunc(sv, w, r, params) {
			return
		}
	}

	//Try GET method
	if r.Method == http.MethodGet {
		urls := sv.ResolvePath(r.URL.Path)
		for i := 0; i < len(urls); i++ {
			//Sort out urls
			url := urls[i]
			err := HandleHTTPGet(w, r, url, sv)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				//Invalid error
				sv.Logger.Log(3, "Error in GET request for: "+r.URL.Path+" | Error: "+err.Error())
				sv.HandleError(w, "Invalid request", http.StatusInternalServerError)
				return
			}
			if err == nil {
				//Get OK
				return
			}
		}
	}

	// Not found
	sv.Logger.Log(3, "NOT FOUND - "+r.RemoteAddr+" - "+r.Method+" - "+r.URL.String())
	sv.HandleError(w, "Not found", http.StatusNotFound)
}

/*
ReadFile reads file contents
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
ReadFileString reads file contents as string
Returns data, isDirectory, error
*/
func ReadFileString(filePath string) (string, bool, error) {
	data, isDir, err := ReadFile(filePath)
	if err != nil {
		return "", isDir, err
	}
	return string(data), isDir, nil
}

/*
JoinPaths joins 2 paths together
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
TryHandleHTTPFile tries to handle file or folder request
*/
func TryHandleHTTPFile(w http.ResponseWriter, filePath string, urlPath string, sv *Server) error {
	//Read data
	data, isDir, err := ReadFile(filePath)
	if err != nil {
		return err
	}

	// Check dir
	if isDir {
		if sv != nil && sv.UseDirectoryListing {
			HandleDirectoryListingHTTP(w, filePath, urlPath, sv)
			return nil
		}
		//http.Error(w, "Directory listing not supported.", http.StatusForbidden)
		//return errors.New("directory listing not supported")
		return os.ErrNotExist
	}

	//Send data
	cType := SortHTTPContentType(filePath, data)
	//fmt.Println(filePath, cType)
	w.Header().Add("Content-Type", cType)
	fmt.Fprint(w, string(data))
	return nil
}

/*
SortHTTPContentType sorts type of file
*/
func SortHTTPContentType(path string, data []byte) string {
	if strings.HasSuffix(path, ".html") {
		return "text/html"
	} else if strings.HasSuffix(path, ".css") {
		return "text/css"
	} else if strings.HasSuffix(path, ".js") {
		return "text/javascript"
	} else if strings.HasSuffix(path, ".map") {
		return "text/json" // JS Map
	} else if strings.HasSuffix(path, ".ts") {
		return "text/x.typescript"
	} else if strings.HasSuffix(path, ".svg") {
		return "image/svg+xml"
	} else if strings.HasSuffix(path, ".mp4") {
		return "video/mp4"
	} else if strings.HasSuffix(path, ".mp3") {
		return "audio/mpeg"
	}
	if data == nil {
		return "text/plain"
	}
	return http.DetectContentType(data)
}

/*
TryHandleHTTPFileRelative handles directory access get request relative to HTTP server root
*/
func (sv *Server) TryHandleHTTPFileRelative(w http.ResponseWriter, _ *http.Request, getPath string) error {
	//Check invalid names
	err := CheckInvalidNames(getPath)
	if err != nil {
		return err
	}
	return TryHandleHTTPFile(w, JoinPaths(sv.rootPath, getPath), getPath, sv)
}

/*
HandleHTTPGet handles directory access get request
*/
func HandleHTTPGet(w http.ResponseWriter, r *http.Request, realPath string, sv *Server) error {
	return TryHandleHTTPFile(w, realPath, r.URL.Path, sv)
}

/*
HandleHTTPGetRelative handles directory access GET request relative to HTTP server
*/
func (sv *Server) HandleHTTPGetRelative(w http.ResponseWriter, r *http.Request) bool {
	urls := sv.ResolvePath(r.URL.Path)
	for i := 0; i < len(urls); i++ {
		//Handle each url
		url := urls[i]
		err := HandleHTTPGet(w, r, url, sv)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			//Invalid error
			sv.Logger.Log(3, "Error in GET request for: "+r.URL.Path+" | Error: "+err.Error())
			sv.HandleError(w, "Invalid request", http.StatusInternalServerError)
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
ReadFileRelative handles file read relative to HTTP server root
Returns data, isDir, error
*/
func (sv *Server) ReadFileRelative(path string) ([]byte, bool, error) {
	return ReadFile(JoinPaths(sv.rootPath, path))
}

/*
CreateParametersFromURL creates map from whole URL
*/
func CreateParametersFromURL(url string) (string, map[string]string) {
	// Split ? parts
	split := strings.SplitN(url, "?", 2)
	if len(split) == 0 {
		return "", nil
	}
	if len(split) == 1 {
		return split[0], nil
	}
	return split[0], CreateParametersFromQuery(split[1])
}

/*
CreateParametersFromQuery creates map from url parameters
*/
func CreateParametersFromQuery(query string) map[string]string {
	// Split & parts
	dataSplit := strings.Split(query, "&")
	postArguments := map[string]string{}

	// Go trought all of them
	for i := 0; i < len(dataSplit); i++ {
		// Split by "=" and unescape
		split := strings.SplitN(dataSplit[i], "=", 2)
		if len(split) == 0 {
			continue
		}
		if split[0] == "" {
			continue
		}
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
CreateURLFromParameters creates url like parameters from map
*/
func CreateURLFromParameters(preURL string, params map[string]string) string {
	result := preURL
	if len(params) > 0 {
		result += "?"
		// Go trought all parameters in map and escape them
		for k, v := range params {
			result += url.QueryEscape(k) + "=" + url.QueryEscape(v) + "&"
		}
	}
	result = strings.TrimSuffix(result, "&")
	result = strings.TrimPrefix(result, "&")
	return result
}

/*
HandleError handles HTTP errors
*/
func (sv *Server) HandleError(w http.ResponseWriter, errText string, code int) {
	//Get page location
	location, has := sv.ErrorPages[code]
	if !has {
		sv.Logger.Log(2, "Error page for code: "+strconv.Itoa(code)+" not found.")
		http.Error(w, errText, code)
		return
	}

	//Read data
	data, isDir, err := ReadFileString(location)
	if err != nil {
		sv.Logger.Log(2, "Error page for code: "+strconv.Itoa(code)+" not found.")
		http.Error(w, errText, code)
		return
	}

	// Check dir
	if isDir {
		sv.Logger.Log(2, "Error page for code: "+strconv.Itoa(code)+" not found.")
		http.Error(w, errText, code)
		return
	}

	//Send data
	data = strings.ReplaceAll(data, "{ERROR}", errText)
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(code)
	fmt.Fprint(w, data)
}

/*
Stop stops HTTP server
*/
func (sv *Server) Stop() {
	if !sv.isAlive {
		return
	}
	err := sv.server.Close()
	if err != nil {
		sv.Logger.Log(3, "Error stopping: "+err.Error())
	}
}
