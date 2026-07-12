package httptools

import (
	"strconv"
	"strings"

	webtools "github.com/kolojar/Go-Webtools"
	"github.com/kolojar/Go-Webtools/helpertools"
)

/*
IHTMLElement is basic interface for any HTML Element / Object / Struct supported by HTML Creator
Do not forget to render all inner elements
*/
type IHTMLElement interface {
	ExportHTML() string
	GetElementBase() *HTMLElementBase
}

/*
HTMLElementBase is basic element structure that must be present in all elements (it is returned in Interface definition)
*/
type HTMLElementBase struct {
	tagName      string
	Attributes   map[string]string
	HTMLElements []IHTMLElement
	InnerHTML    string
}

/*
SetTagName sets tag name of element
Returns true if set was OK
*/
func (base *HTMLElementBase) SetTagName(newName string) bool {
	if strings.ContainsAny(newName, " \"'=></&@!#~$/^*(){}[]\\|,;:") {
		return false
	}
	if len(newName) == 0 {
		return false
	}
	if !strings.ContainsAny(string(newName[0]), webtools.AlphabetLetters) {
		return false
	}
	base.tagName = newName
	return true
}

/*
GetTagName gets tag name of this element
*/
func (base *HTMLElementBase) GetTagName() string {
	return base.tagName
}

/*
NewHTMLElementBase creates new HTML Element base, needs name of tag - a, h1, div, ...
Returns nil when tagName is invalid
*/
func NewHTMLElementBase(tagName string) *HTMLElementBase {
	element := &HTMLElementBase{tagName: tagName, Attributes: map[string]string{}, HTMLElements: make([]IHTMLElement, 0), InnerHTML: ""}
	if element.SetTagName(tagName) {
		return element
	}
	return nil
}

/*
NewHTMLElementBaseWithData creates new HTML Element base, needs name of tag - a, h1, div, ...
Returns nil when tagName is invalid
Sets innerHTML
*/
func NewHTMLElementBaseWithData(tagName string, innerHTML string) *HTMLElementBase {
	element := NewHTMLElementBase(tagName)
	if element == nil {
		return nil
	}
	element.InnerHTML = innerHTML
	return element
}

/*
ExportHTML exports HTML Base to string format that can be used in Web Browser
*/
func (base *HTMLElementBase) ExportHTML() string {
	//Get Attributes
	attributesString := ""
	for k, v := range base.Attributes {
		attributesString += " " + k + "=\"" + v + "\""
	}

	//Generate innerHTML
	innerHTML := base.InnerHTML
	if innerHTML == "" {
		for i := 0; i < len(base.HTMLElements); i++ {
			innerHTML += base.HTMLElements[i].ExportHTML()
		}
	}
	return "<" + base.tagName + attributesString + ">" + innerHTML + "</" + base.tagName + ">"
}

/*
GetElementBase gets ElementBase of element
*/
func (base *HTMLElementBase) GetElementBase() *HTMLElementBase {
	return base
}

/*
FindElementsByAttributes finds element by specified attributes
*/
func (base *HTMLElementBase) FindElementsByAttributes(attributes map[string]string) []IHTMLElement {
	result := make([]IHTMLElement, 0)
	//Check this
	found := (len(base.Attributes) == 0 && len(attributes) == 0) || len(attributes) > 0
	for k, v := range attributes {
		if base.Attributes[k] != v {
			found = false
			break
		}
	}
	if found {
		result = append(result, base)
	}

	//Look for children
	for _, v := range base.HTMLElements {
		result = append(result, v.GetElementBase().FindElementsByAttributes(attributes)...)
	}
	return result
}

/*
MoveScriptsToEnd finds scripts and moves them at the end of element
*/
func (base *HTMLElementBase) MoveScriptsToEnd() {
	//Get scripts
	scripts := make([]IHTMLElement, 0)
	for _, element := range base.HTMLElements {
		if element.GetElementBase().tagName == "script" {
			scripts = append(scripts, element)
		}
	}

	//Delete scripts
	for _, element := range scripts {
		base.HTMLElements = helpertools.RemoveElement(base.HTMLElements, element)
	}

	//Add scripts
	base.HTMLElements = append(base.HTMLElements, scripts...)
}

/*
NewHTMLJSLinkElement creates new JS Link
*/
func NewHTMLJSLinkElement(src string, typeOfImport string) *HTMLElementBase {
	base := NewHTMLElementBase("script")
	if src != "" {
		base.Attributes["src"] = src
	}
	if typeOfImport != "" {
		base.Attributes["type"] = typeOfImport
	}
	return base
}

/*
HTMLListElement is list element for HTML
*/
type HTMLListElement struct {
	base *HTMLElementBase
}

/*
ExportHTML exports list to HTML format
*/
func (list *HTMLListElement) ExportHTML() string {
	return list.base.ExportHTML()
}

/*
GetElementBase gets list element base
*/
func (list *HTMLListElement) GetElementBase() *HTMLElementBase {
	return list.base
}

/*
SetIsOrdered sets if list is ordered or not
*/
func (list *HTMLListElement) SetIsOrdered(isOrdered bool) {
	if isOrdered {
		list.base.tagName = "ol"
	} else {
		list.base.tagName = "ul"
	}
}

/*
AddItem adds element in list
*/
func (list *HTMLListElement) AddItem(element IHTMLElement) {
	item := NewHTMLElementBase("li")
	item.HTMLElements = append(item.HTMLElements, element)
	list.base.HTMLElements = append(list.base.HTMLElements, item)
}

/*
NewHTMLListElement creates new List element
*/
func NewHTMLListElement() *HTMLListElement {
	l := &HTMLListElement{}
	l.base = NewHTMLElementBase("ul")
	return l
}

/*
NewHTMLHxElement generates new hx element
*/
func NewHTMLHxElement(level uint8, text string) *HTMLElementBase {
	if level == 0 || level > 6 {
		return nil
	}

	//Generate hx element
	base := NewHTMLElementBase("h" + strconv.FormatUint(uint64(level), 10))
	base.InnerHTML = text
	return base
}

/*
NewHTMLAElement creates new link (a) element
*/
func NewHTMLAElement(href string, target string) *HTMLElementBase {
	base := NewHTMLElementBase("a")
	if target != "" {
		base.Attributes["target"] = target
	}
	base.Attributes["href"] = href
	return base
}

/*
HTMLCreator is basic HTML structure generator
*/
type HTMLCreator struct {
	HeadElements      []IHTMLElement
	BodyElement       *HTMLElementBase
	GenerateStructure bool
	Lang              string
	Title             string
	MoveScriptsToEnd  bool
}

/*
NewHTMLCreator creates new instance of HTML Creator
*/
func NewHTMLCreator(generateStructure bool, lang string, title string, moveScriptsToEnd bool) *HTMLCreator {
	return &HTMLCreator{GenerateStructure: generateStructure, Lang: lang, BodyElement: NewHTMLElementBase(helpertools.FormatByBool(generateStructure, "body", "div")), HeadElements: make([]IHTMLElement, 0), Title: title, MoveScriptsToEnd: moveScriptsToEnd}
}

/*
ExportHTML exports whole HTML file
*/
func (creator *HTMLCreator) ExportHTML() string {
	if creator.MoveScriptsToEnd {
		creator.BodyElement.MoveScriptsToEnd()
	}
	if !creator.GenerateStructure {
		return creator.BodyElement.ExportHTML()
	}

	//Generate structure
	result := "<!DOCTYPE html>"
	result += "<html lang=\"" + creator.Lang + "\">"

	//Make head
	result += "<head>"
	result += "<meta charset=\"UTF-8\">"
	result += "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">"
	result += "<title>" + creator.Title + "</title>"
	for i := 0; i < len(creator.HeadElements); i++ {
		result += creator.HeadElements[i].ExportHTML()
	}
	result += "</head>"

	//Generate body
	//result += "<body>"
	result += creator.BodyElement.ExportHTML()
	//result += "</body>"
	result += "</html>"
	return result
}

/*
GetElementBase gets base, in this case body, element
*/
func (creator *HTMLCreator) GetElementBase() *HTMLElementBase {
	return creator.BodyElement
}

/*
AddBodyElement adds element to body
*/
func (creator *HTMLCreator) AddBodyElement(element IHTMLElement) {
	creator.BodyElement.HTMLElements = append(creator.BodyElement.HTMLElements, element)
}

/*
RemoveBodyElement removes element from body
*/
func (creator *HTMLCreator) RemoveBodyElement(element IHTMLElement) {
	creator.BodyElement.HTMLElements = helpertools.RemoveElement(creator.BodyElement.HTMLElements, element)
}
