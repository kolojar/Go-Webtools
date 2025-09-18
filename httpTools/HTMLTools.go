package httpTools

import (
	"strconv"
)

// Do not forget to render all inner elements
type IHTMLElement interface {
	ExportHTML() string
	GetElementBase() *HTMLElementBase
}

/*
Element base is basic element structure
*/
type HTMLElementBase struct {
	TagName      string
	Attributes   map[string]string
	HTMLElements []IHTMLElement
	InnerHTML    string
}

func NewHTMLElementBase(tagName string) *HTMLElementBase {
	return &HTMLElementBase{TagName: tagName, Attributes: map[string]string{}, HTMLElements: make([]IHTMLElement, 0), InnerHTML: ""}
}
func (base *HTMLElementBase) ExportHTML() string {
	//Get Attributes
	attributesString := ""
	for k, v := range base.Attributes {
		attributesString += " " + k + "=\"" + v + "\""
	}

	//Generate InnerHTML
	innerHtml := base.InnerHTML
	if innerHtml == "" {
		for i := 0; i < len(base.HTMLElements); i++ {
			innerHtml += base.HTMLElements[i].ExportHTML()
		}
	}
	return "<" + base.TagName + attributesString + ">" + innerHtml + "</" + base.TagName + ">"
}
func (base *HTMLElementBase) GetElementBase() *HTMLElementBase {
	return base
}

/*
Creates new JS Link
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
List element for HTML
*/
type HTMLListElement struct {
	base *HTMLElementBase
}

func (list *HTMLListElement) ExportHTML() string {
	return list.base.ExportHTML()
}
func (list *HTMLListElement) GetElementBase() *HTMLElementBase {
	return list.base
}
func (list *HTMLListElement) SetIsOrdered(isOrdered bool) {
	if isOrdered {
		list.base.TagName = "ol"
	} else {
		list.base.TagName = "ul"
	}
}
func (list *HTMLListElement) AddItem(element IHTMLElement) {
	item := NewHTMLElementBase("li")
	item.HTMLElements = append(item.HTMLElements, element)
	list.base.HTMLElements = append(list.base.HTMLElements, item)
}
func NewHTMLListElement() *HTMLListElement {
	l := &HTMLListElement{}
	l.base = NewHTMLElementBase("ul")
	return l
}

/*
Hx generator
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
A (link) generator
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
Basic HTML structure generator
*/
type HTMLCreator struct {
	HeadElements      []IHTMLElement
	BodyElement       *HTMLElementBase
	GenerateStructure bool
	Lang              string
	Title             string
}

func NewHTMLCreator(GenerateStructure bool, lang string, titile string) *HTMLCreator {
	return &HTMLCreator{GenerateStructure: GenerateStructure, Lang: lang, BodyElement: NewHTMLElementBase("body"), HeadElements: make([]IHTMLElement, 0), Title: titile}
}
func (creator *HTMLCreator) ExportHTML() string {
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
	result += "<body>"
	result += creator.BodyElement.ExportHTML()
	result += "</body>"
	return result
}
func (creator *HTMLCreator) GetElementBase() *HTMLElementBase {
	return creator.BodyElement
}
func (creator *HTMLCreator) AddBodyElement(element IHTMLElement) {
	creator.BodyElement.HTMLElements = append(creator.BodyElement.HTMLElements, element)
}
