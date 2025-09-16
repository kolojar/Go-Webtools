package httpTools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

/*
Renders directory listing to this request
Returns if it was handeled
*/
func HandleDirectoryListingHTTP(w http.ResponseWriter, path string, httpServer *HTTPServer) bool {
	http.Error(w, "Directory listing not working", 404)
	return false

	//Check for directory
	path = strings.ReplaceAll(path, "//", "/")
	path = strings.ReplaceAll(httpServer.GetRootPath()+path, "//", "/")
	info, err4 := os.Stat(path)
	if err4 != nil {
		http.Error(w, "Invalid file path. Internal error: "+err4.Error(), http.StatusInternalServerError)
		return true
	}
	if !info.IsDir() {
		return false
	}

	//Fix suffix
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	path = strings.ReplaceAll(path, "//", "/")

	//Read data
	dataBinary, _, err := httpServer.ReadFileRelative(JoinPaths(httpServer.HostPaths["/dirlist"], "/directoryListingViewer.html"))
	if err != nil {
		http.Error(w, "File template not found! Internal error: "+err.Error(), http.StatusInternalServerError)
		return true
	}

	//List folder
	httpList, err3 := getItemsInFolderRelative(path, httpServer)
	if err3 != nil {
		http.Error(w, "Could not get information about folder! Internal error: "+err3.Error(), http.StatusInternalServerError)
		return true
	}

	//Replace data
	data := string(dataBinary)
	data = strings.Replace(data, "[HTTP_ITEMS]", (httpList), 1)
	data = strings.Replace(data, "[HTTP_PATH]", strings.TrimPrefix(path, "."), 1)

	//Process site
	fmt.Fprint(w, data)
	return true
}

type directoryListingEntry struct {
	Path     string
	Name     string
	FileType string
}

/*
Gets items in folder for directory listing
Returns string as JSON
Path must be path in HTTP server
*/
func getItemsInFolderRelative(path string, sv *HTTPServer) (string, error) {
	//Convert to real path
	var realPath string = path
	for k, v := range sv.HostPaths {
		//Sort out hostPaths
		if strings.HasPrefix(path, k) {
			realPath = strings.Replace(path, k, v, 1)
			break
		}
	}

	//Read folder
	entries, err := os.ReadDir(realPath)
	if err != nil {
		return "", err
	}

	//Make entries
	result := make([]directoryListingEntry, 0)
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		newEntry := directoryListingEntry{Path: path + entry.Name(), Name: entry.Name()}
		newEntry.Path = strings.TrimPrefix(newEntry.Path, ".")
		if entry.IsDir() {
			newEntry.FileType = "directory"
		} else {
			//TODO: IANA TYPES
			newEntry.FileType = "file"
		}
		result = append(result, newEntry)
	}

	//Create JSON
	data, err2 := json.Marshal(result)
	return string(data), err2
}

/*
Setups directory listing. Path is located in HTTP Tools as views directory
Resources are registered in /dirlist subfolder
*/
func SetupDirectoryListing(sv *HTTPServer, pathToViewsOfDirectoryListing string) {
	sv.HostPaths["/dirlist"] = pathToViewsOfDirectoryListing
	sv.useDirListing = true
}
