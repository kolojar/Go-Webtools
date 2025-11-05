/*
Package http provides tools for working with HTML files and tools for HTTP traffic
*/
package http

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

/*
HandleDirectoryListingHTTP renders directory listing to this request
Returns if it was handeled
*/
func HandleDirectoryListingHTTP(w http.ResponseWriter, realPath string, urlPath string, httpServer *Server) bool {
	//Check for directory
	_, isDir, err := ReadFile(realPath)
	if err != nil {
		http.Error(w, "Invalid file path. Internal error: "+err.Error(), http.StatusInternalServerError)
		return true
	}
	if !isDir {
		return false
	}

	//List folder
	httpList, err3 := getItemsInFolderRelative(realPath, urlPath, httpServer)
	if err3 != nil {
		http.Error(w, "Could not get information about folder! Internal error: "+err3.Error(), http.StatusInternalServerError)
		return true
	}

	//Create HTML
	creator := NewHTMLCreator(true, "en", "directoryListing", true)
	creator.AddBodyElement(NewHTMLHxElement(1, "Current directory: "+urlPath))
	list := NewHTMLListElement()

	//Create up folder
	split := strings.Split(urlPath, "/")
	if len(split) > 1 && len(split[1]) > 0 {
		upPath := ""
		if len(split) == 2 {
			upPath = "/"
		} else {
			split = split[0 : len(split)-1]
			upPath = strings.Join(split, "/")
		}
		upFolder := NewHTMLAElement(upPath, "")
		upFolder.InnerHTML = "Parent folder (..)"
		list.AddItem(upFolder)
	}

	//Add entries
	for i := 0; i < len(httpList); i++ {
		a := NewHTMLAElement(httpList[i].Path, "")
		a.InnerHTML = httpList[i].Name
		list.AddItem(a)
	}
	creator.AddBodyElement(list)

	//Replace data
	//data := LISTING_HTML
	//data = strings.Replace(data, "[HTTP_ITEMS]", (httpList), 1)
	//dapta = strings.Replace(data, "[HTTP_PATH]", strings.TrimPrefix(urlPath, "."), 1)

	//Process site
	fmt.Fprint(w, creator.ExportHTML())
	return true
}

type directoryListingEntry struct {
	Path     string
	Name     string
	FileType string
}

/*
Gets items in folder for directory listing
Path must be path in HTTP server
*/
func getItemsInFolderRelative(realPath string, urlPath string, _ *Server) ([]directoryListingEntry, error) {
	//Read folder
	entries, err := os.ReadDir(realPath)
	if err != nil {
		return nil, err
	}

	//Make entries
	result := make([]directoryListingEntry, 0)
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		newEntry := directoryListingEntry{Path: JoinPaths(urlPath, entry.Name()), Name: entry.Name()}
		newEntry.Path = strings.TrimPrefix(newEntry.Path, ".")
		if entry.IsDir() {
			newEntry.FileType = "directory"
		} else {
			//TODO: IANA TYPES
			newEntry.FileType = "file"
		}
		result = append(result, newEntry)
	}
	return result, nil
}
