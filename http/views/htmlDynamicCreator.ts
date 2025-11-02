let sendState = 0
const websocket = new WebSocket("/dynamicHTML")
websocket.addEventListener("open",function() {
    sendState = 1
    console.log("DYNAMIC-HTML: Internal WebSocket opened.");
})
websocket.addEventListener("close",function() {
    sendState = 2
    console.log("DYNAMIC-HTML: Internal WebSocket closed.");
})
websocket.addEventListener("message",function(message) {
    //Message format: ID, operation, data
    const messageSplit = String(message.data).split("&",3)
    const targetElements =document.querySelectorAll('[dynamic-html-id="' + messageSplit[0] + '"]');
    targetElements.forEach(element => {
        //Work with every element
        switch (String(messageSplit[1])) {
            case "setInnerHTML":
                element.innerHTML = messageSplit[2]
                break;
            default:
                console.warn("DYNAMIC-HTML: Operation: " + String(messageSplit[1]) + " not found."); 
                break;
        }
    });
})