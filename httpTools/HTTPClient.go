package httptools

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

// Primitive HTTPClient
type HTTPClient struct {
	client        *http.Client
	address       string
	Logger        *webtools.ConsoleLogger
	reportTraffic bool
}

func (cl *HTTPClient) GetReportTraffic() bool {
	return cl.reportTraffic
}

/*
Creates new HTTP Client and makes it ready for usage. Timeout is in seconds
*/
func NewHTTPClient(address string, timeout int64, reportTraffic bool) (*HTTPClient, error) {
	level := uint8(0)
	if !reportTraffic {
		level = 2
	}
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		return nil, errors.New("invalid url format, must start with http:// or https://")
	}
	return &HTTPClient{client: &http.Client{Timeout: time.Duration(timeout) * time.Second}, address: address, Logger: webtools.NewConsoleLogger("HTTPClient", level), reportTraffic: reportTraffic}, nil
}

/*
Creates HTTP request coresponding to HTTP Client
*/
func (cl *HTTPClient) NewRequest(method string, contentType string, data []byte) (*http.Request, error) {
	//Make reader
	var reader io.Reader = nil
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
Sends raw request
*/
func (cl *HTTPClient) SendRequestRaw(request *http.Request) *http.Response {
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
Sends request and returns responce. Do not forget to close bodu on responce
*/
func (cl *HTTPClient) SendRequest(method string, contentType string, data []byte) *http.Response {
	request, err := cl.NewRequest(method, contentType, data)
	if err != nil {
		cl.Logger.Log(3, "Error creating request: "+err.Error())
		return nil
	}

	//Send request
	return cl.SendRequestRaw(request)
}

/*
Sends request and returns responce data
*/
func (cl *HTTPClient) SendRequestData(method string, contentType string, data []byte) []byte {
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
Get Hijack address for TCP client
*/
func (cl *HTTPClient) GetHijackAddress() string {
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
