package httptools

import "webtools"

type HTMLDynamicActiveElement struct {
	element IHTMLElement
	value   string
}

/*
Creator with ability of creating dynamic sites based on server events
*/
type HTMLCreatorDynamic struct {
	creator         *HTMLCreator
	websocketServer *WebSocketServer
	activeObjects   webtools.SafeMap[string, *HTMLDynamicActiveElement]
}

/*
Creates new instance of HTML creator dynamic
*/
func NewHTMLCreatorDynamic() {

}
