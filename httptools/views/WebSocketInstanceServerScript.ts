console.log("Sending request...");

//Create request
const xhr = new XMLHttpRequest();
xhr.open("POST", "/instanceServerWebsocketNewInstance");
xhr.setRequestHeader("Content-Type", "application/json; charset=UTF-8")

//Get responce (lots of time none)
xhr.addEventListener("load", () => {
    if (xhr.readyState == 4 && xhr.status == 201) {
        const responce = xhr.responseText
        console.log(responce);
        window.location.replace("{HREF}");
    } else {
        console.log(`Error: ${xhr.status}`);
    }
});

//Send data
xhr.send(encodeURI(window.location.toString()));