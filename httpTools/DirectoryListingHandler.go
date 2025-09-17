package httpTools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Directory listing side
const LISTING_HTML = `
<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Directory listing</title>
</head>

<body>
    <dirlist id="directoryListing"></dirlist>
</body>
<script type="module">
/*
Creates directory listing from JSON without file refresh
*/
export function ListDirectoryJSON(owner, path, data) {
    if (path.endsWith("/") && path.length > 1) {
        path = path.substring(0, path.length - 1);
    }
    //Setup current path title
    owner.children[0].innerHTML = "Current directory: " + path;
    //Clean all children
    const list = owner.children[1];
    for (let i = 0; i < list.children.length; i++) {
        const element = list.children[i];
        element.remove();
    }
    //Calculate parent path
    let parent = null;
    if (path.split("/").length > 1) {
        const split = path.split("/");
        split.pop();
        if (path != "/") {
            parent = split.join("/");
            if (parent == "") {
                parent = "/";
            }
        }
    }
    //Create parent folder link
    if (parent != null) {
        const itemParent = document.createElement("li");
        const linkToParent = document.createElement("a");
        linkToParent.href = parent;
        linkToParent.innerHTML = "Parent folder (..)";
        itemParent.appendChild(linkToParent);
        list.appendChild(itemParent);
    }
    //Create all elements to list
    for (let i = 0; i < data.length; i++) {
        const item = data[i];
        const listItem = document.createElement("li");
        const linkToItem = document.createElement("a");
        linkToItem.href = item.Path;
        linkToItem.innerHTML = item.Name;
        listItem.appendChild(linkToItem);
        list.appendChild(listItem);
    }
}
/*
Setups all directory listings with tag <dirlist>
*/
export function SetupDirectoryListings() {
    const elements = document.getElementsByTagName("dirlist");
    for (let i = 0; i < elements.length; i++) {
        //Setup inside elements
        const element = elements[i];
        //Title
        const currentPathHeader = document.createElement("h1");
        element.appendChild(currentPathHeader);
        //Create list
        const listHolder = document.createElement("ul");
        element.appendChild(listHolder);
        ////Get type of list
        //if (element.hasAttribute("useWebSocket")) {
        //    ListDirectoryWithWebSocket(element, wsc)
        //} else {
        //    currentPathHeader.innerHTML = element.getAttribute("path")
        //    ListDirectoryJSON(element, element.getAttribute("path"), JSON.parse(element.getAttribute("httpItems")))
        //}
    }
}
SetupDirectoryListings()

const params = new URLSearchParams(document.location.search)
let path = "[HTTP_PATH]"
let httpItems = [HTTP_ITEMS]
if (params.get("path")) {
    path = params.get("path")
} 
if (params.get("httpItems")) {
    httpItems = JSON.parse(decodeURI(params.get("httpItems")))
}
ListDirectoryJSON(document.getElementsByTagName("dirlist")[0],path,httpItems )
</script>

</html>
`

/*
Renders directory listing to this request
Returns if it was handeled
*/
func HandleDirectoryListingHTTP(w http.ResponseWriter, realPath string, urlPath string, httpServer *HTTPServer) bool {
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
	httpList, err3 := getItemsInFolderRelative(realPath, httpServer)
	if err3 != nil {
		http.Error(w, "Could not get information about folder! Internal error: "+err3.Error(), http.StatusInternalServerError)
		return true
	}

	//Replace data
	data := LISTING_HTML
	data = strings.Replace(data, "[HTTP_ITEMS]", (httpList), 1)
	data = strings.Replace(data, "[HTTP_PATH]", strings.TrimPrefix(urlPath, "."), 1)

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
		newEntry := directoryListingEntry{Path: path + "/" + entry.Name(), Name: entry.Name()}
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
