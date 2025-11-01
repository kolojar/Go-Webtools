import { WebSocketConnection } from "./serverComunication";
import { SetupWebsocketWithToasts } from "./webSocketForms";

/*
Lists directory with update support from server using WebSocket
*/
export function ListDirectoryWithWebSocket(owner: HTMLElement, wsc: WebSocketConnection) {
    console.error("TODO");
    alert("todo")
}

/*
Creates directory listing from JSON without file refresh
*/
export function ListDirectoryJSON(owner: HTMLElement, path: string, data: any) {
    if (path.endsWith("/") && path.length > 1) {
        path = path.substring(0, path.length - 1)
    }

    //Setup current path title
    owner.children[0].innerHTML = "Current directory: " + path

    //Clean all children
    const list = owner.children[1]
    for (let i = 0; i < list.children.length; i++) {
        const element = list.children[i];
        element.remove()
    }

    //Calculate parent path
    let parent = null
    if (path.split("/").length > 1) {
        const split = path.split("/")
        split.pop()
        if (path != "/") {
            parent = split.join("/")
            if (parent == "") {
                parent = "/"
            }
        }
    }

    //Create parent folder link
    if (parent != null) {
        const itemParent = document.createElement("li")
        const linkToParent = document.createElement("a")
        linkToParent.href = parent
        linkToParent.innerHTML = "Parent folder (..)"
        itemParent.appendChild(linkToParent)
        list.appendChild(itemParent)
    }

    //Create all elements to list
    for (let i = 0; i < data.length; i++) {
        const item = data[i];
        const listItem = document.createElement("li")
        const linkToItem = document.createElement("a")
        linkToItem.href = item.Path
        linkToItem.innerHTML = item.Name
        listItem.appendChild(linkToItem)
        list.appendChild(listItem)
    }
}

/*
Setups all directory listings with tag <dirlist>
*/
export function SetupDirectoryListings() {
    const elements = document.getElementsByTagName("dirlist")
    for (let i = 0; i < elements.length; i++) {
        //Setup inside elements
        const element = elements[i] as HTMLElement;

        //Title
        const currentPathHeader = document.createElement("h1")
        element.appendChild(currentPathHeader)

        //Create list
        const listHolder = document.createElement("ul")
        element.appendChild(listHolder)

        ////Get type of list
        //if (element.hasAttribute("useWebSocket")) {
        //    ListDirectoryWithWebSocket(element, wsc)
        //} else {
        //    currentPathHeader.innerHTML = element.getAttribute("path")
        //    ListDirectoryJSON(element, element.getAttribute("path"), JSON.parse(element.getAttribute("httpItems")))
        //}
    }
}
