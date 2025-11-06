package httpTools

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"webtools"
)

/*
Client is primitive HTTP client
*/
type Client struct {
	client  *http.Client
	address string
	Logger  *webtools.ConsoleLogger
}

/*
NewClient creates new HTTP Client and makes it ready for usage. Timeout is in seconds
*/
func NewClient(address string, timeout int64, reportTraffic bool) (*Client, error) {
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		return nil, errors.New("invalid url format, must start with http:// or https://")
	}
	return &Client{client: &http.Client{Timeout: time.Duration(timeout) * time.Second}, address: address, Logger: webtools.NewConsoleLoggerForTraffic("HTTPClient", reportTraffic)}, nil
}

/*
NewRequest creates HTTP request coresponding to HTTP Client
*/
func (cl *Client) NewRequest(method string, contentType string, data []byte) (*http.Request, error) {
	//Make reader
	var reader io.Reader
	if data != nil {
		reader = bytes.NewReader(data)
	}

	//Make reader
	request, err := http.NewRequest(method, cl.address, reader)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return request, nil
}

/*
SendRequestRaw sends raw request
*/
func (cl *Client) SendRequestRaw(request *http.Request) *http.Response {
	cl.Logger.Log(1, "Sending - "+request.Method+" - "+request.URL.String())
	responce, err := cl.client.Do(request)
	if err != nil {
		cl.Logger.Log(3, "Error sending request: "+err.Error())
		return nil
	}
	cl.Logger.Log(1, "Reading - "+responce.Request.Method+" - "+strconv.Itoa(responce.StatusCode)+" - "+responce.Request.URL.String())
	if responce.StatusCode != 200 {
		cl.Logger.Log(2, "Reading "+responce.Request.URL.String()+" returned non OK responce with code: "+responce.Status)
	}
	return responce
}

/*
SendRequest sends request and returns responce. Do not forget to close body on responce
*/
func (cl *Client) SendRequest(method string, contentType string, data []byte) *http.Response {
	request, err := cl.NewRequest(method, contentType, data)
	if err != nil {
		cl.Logger.Log(3, "Error creating request: "+err.Error())
		return nil
	}

	//Send request
	return cl.SendRequestRaw(request)
}

/*
SendRequestData sends request and returns responce data
*/
func (cl *Client) SendRequestData(method string, contentType string, data []byte) []byte {
	//Get responce
	responce := cl.SendRequest(method, contentType, data)
	defer responce.Body.Close()

	//Get data
	body, err2 := io.ReadAll(responce.Body)
	if err2 != nil {
		cl.Logger.Log(3, "Error reading responce data: "+err2.Error())
		return nil
	}
	return body
}

/*
GetHijackAddress gets address for TCP client
*/
func (cl *Client) GetHijackAddress() string {
	cl.Logger.Log(2, "Getting host address...")
	resp := cl.SendRequest("GET", "", nil)
	host := resp.Request.URL.Hostname()
	port := resp.Request.URL.Port()
	if port == "" {
		cl.Logger.Log(2, "Error getting host address, port not specified and could not be get from host, using default by protocol.")
		switch resp.Request.URL.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			cl.Logger.Log(3, "Error getting host address, port not specified and could not be get from host, protocol is invalid.")
			return ""
		}
	}
	cl.Logger.Log(2, "Got host address")
	return net.JoinHostPort(host, port)
}
