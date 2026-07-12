package p2p

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	webtools "github.com/kolojar/Go-Webtools"
	"github.com/kolojar/Go-Webtools/helpertools"
)

/*
UPnPXMLRoot - helper struct
*/
type UPnPXMLRoot struct {
	XMLName xml.Name        `xml:"root"`
	Device  []UPnPXMLDevice `xml:"device"`
}

/*
UPnPXMLDevice - helper struct
*/
type UPnPXMLDevice struct {
	XMLName  xml.Name         `xml:"device"`
	Services []UPnPXMLService `xml:"serviceList>service"`
	Device   []UPnPXMLDevice2 `xml:"deviceList>device"`
}

/*
UPnPXMLDevice2 - helper struct
*/
type UPnPXMLDevice2 struct {
	XMLName  xml.Name         `xml:"device"`
	Services []UPnPXMLService `xml:"serviceList>service"`
	Device   []UPnPXMLDevice  `xml:"deviceList>device"`
}

/*
UPnPXMLService - helper struct
*/
type UPnPXMLService struct {
	XMLName     xml.Name `xml:"service"`
	ServiceType string   `xml:"serviceType"`
	ControlURL  string   `xml:"controlURL"`
}

const upnpTargetService = "urn:schemas-upnp-org:service:WANIPConnection:1"

/*
UPnPServiceManager is manager for UPnP and getting public IP from router
*/
type UPnPServiceManager struct {
	controlURLs []string
	Logger      *helpertools.ConsoleLogger
	localIP     string
	mappedUrls  helpertools.SafeMap[int, string] //In format externalPort, protocol
}

/*
NewUPnPServiceManager creates new UPnP service manager
It is recommended to call Shutdown on end
*/
func NewUPnPServiceManager(localIP string) *UPnPServiceManager {
	return &UPnPServiceManager{
		controlURLs: make([]string, 0),
		Logger:      webtools.NewConsoleLogger("UPnP", 0),
		mappedUrls:  helpertools.MakeSafeMap[int, string](),
		localIP:     localIP,
	}
}

/*
SetupUPnP setups UPnP manager for usage
*/
func (upnp *UPnPServiceManager) SetupUPnP() error {
	upnp.Logger.Log(2, "Getting UPnP device...")
	//New client for SSDP service that works in UPnP server. It is located on that specific IP and port (it is local, not some internet service)
	addrSSDP, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return err
	}
	connSSDP, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return err
	}
	defer connSSDP.Close()

	//Create search request
	requestMSearch := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 3\r\n" +
		"ST: urn:schemas-upnp-org:device:InternetGatewayDevice:1\r\n" +
		"\r\n"

	//Send request
	_, err = connSSDP.WriteTo([]byte(requestMSearch), addrSSDP)
	if err != nil {
		return err
	}

	//Set deadline
	connSSDP.SetReadDeadline(time.Now().Add(3 * time.Second))

	//Read responce
	locationsOfSSDP := make(map[string]struct{})
	buffer := make([]byte, 1024)

	for {
		//Read
		n, source, err := connSSDP.ReadFrom(buffer)
		if err != nil {
			upnp.Logger.Log(3, "Error reading from SSDP: "+err.Error())
			upnp.Logger.Log(2, "Found "+strconv.Itoa(len(upnp.controlURLs))+" UPnP devices")
			return err
		}

		//Process responce
		responce := buffer[:n]
		upnp.Logger.Log(0, "Got SSDP responce from: "+source.String()+" with data: "+string(responce))
		var responceLocationURL = ""
		readerResponce := bufio.NewReader(strings.NewReader(string(responce)))
		for {
			//Get location
			line, err := readerResponce.ReadString('\n')
			if err != nil {
				upnp.Logger.Log(3, "Error reading responce for location: "+err.Error())
				break
			}
			if strings.HasPrefix(strings.ToUpper(line), "LOCATION:") {
				responceLocationURL = strings.TrimSpace(line[9:]) //Ignore location string
				break
			}
		}

		//Check valid location
		if responceLocationURL == "" {
			//Skip invalid location
			upnp.Logger.Log(1, "Invalid UPnP location URL: "+responceLocationURL+", skiping...")
			continue
		}

		//Check if already viewed
		_, exists := locationsOfSSDP[responceLocationURL]
		if exists {
			//Ignore already viewed request
			upnp.Logger.Log(1, "UPnP location already seen: "+responceLocationURL+", skiping...")
			continue
		}

		//Not viewed request
		locationsOfSSDP[responceLocationURL] = struct{}{}
		/*if !strings.Contains(strings.ToLower(responceLocationURL), "wanip") && !strings.Contains(strings.ToLower(responceLocationURL), "internetgatewaydevice") {
			//Not valid request
			upnp.Logger.Log(1, "Invalid UPnP request: "+responceLocationURL+", skiping...")
			continue
		}*/

		//Get base URL
		splitLocationsOfSSDP := strings.Split(responceLocationURL, "/")
		baseLocationURL := ""
		if len(splitLocationsOfSSDP) >= 3 {
			baseLocationURL = strings.Join(splitLocationsOfSSDP[:3], "/")
		} else {
			baseLocationURL = responceLocationURL
		}

		//Get XML
		upnp.Logger.Log(0, "Getting XML data for settings from: "+responceLocationURL)
		responceBaseLocationXML, err := http.Get(responceLocationURL)
		if err != nil {
			upnp.Logger.Log(3, "Error getting XML responce: "+err.Error())
			continue
		}
		var baseLocationXMLData []byte
		if responceBaseLocationXML.StatusCode == http.StatusOK {
			baseLocationXMLData, err = io.ReadAll(responceBaseLocationXML.Body)
			if err != nil {
				upnp.Logger.Log(3, "Error getting XML data: "+err.Error())
				continue
			}
		}
		upnp.Logger.Log(0, "Got XML data for controling: "+string(baseLocationXMLData))

		//Get controlURL
		var xmlRoot UPnPXMLRoot
		err = xml.Unmarshal(baseLocationXMLData, &xmlRoot)
		if err != nil {
			//Ignore invalid
			upnp.Logger.Log(3, "Error unmarshalling XML data: "+err.Error())
			continue
		}
		//fmt.Println(xmlRoot)
		var controlURL = recurseDevices(xmlRoot.Device, baseLocationURL)
		if controlURL == "" {
			//No valid service was found
			upnp.Logger.Log(3, "No valid controlUrl was found.")
			continue
		}
		upnp.Logger.Log(1, "Found controlURL: "+controlURL)
		upnp.controlURLs = append(upnp.controlURLs, controlURL)
	}
}

func recurseDevices(devices []UPnPXMLDevice, baseLocationURL string) string {
	for _, device := range devices {
		for _, service := range device.Services {
			if service.ServiceType == upnpTargetService {
				//Service is valid type, check if is relative or absolute URL
				if strings.HasPrefix(service.ControlURL, "http") {
					return service.ControlURL
				}
				return strings.TrimSuffix(baseLocationURL, "/") + service.ControlURL
			}
		}
		for _, device := range device.Device {
			for _, service := range device.Services {
				if service.ServiceType == upnpTargetService {
					//Service is valid type, check if is relative or absolute URL
					if strings.HasPrefix(service.ControlURL, "http") {
						return service.ControlURL
					}
					return strings.TrimSuffix(baseLocationURL, "/") + service.ControlURL
				}
			}
			url := recurseDevices(device.Device, baseLocationURL)
			if url != "" {
				return url
			}
		}
	}
	return ""
}

/*
AddUPnPPort adds UPnP mapping to all avaliable control URLs
*/
func (upnp *UPnPServiceManager) AddUPnPPort(externalPort int, internalPort int, protocol string, description string) error {
	if len(upnp.controlURLs) == 0 {
		upnp.Logger.Log(3, "No UPnP control URLs found!")
		return errors.New("no control urls found")
	}
	if !upnp.mappedUrls.Has(externalPort) {
		upnp.Logger.Log(1, "This external port is already registered, error may occur.")
	}

	//Create SOAP body
	upnp.Logger.Log(1, "Adding UPnP port for external port: "+strconv.Itoa(externalPort)+" to internal port: "+strconv.Itoa(internalPort)+" to IP: "+upnp.localIP+" with protocol: "+protocol+" with description: "+description+"...")
	soapAddPortBody := `<?xml version="1.0"?>
	<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
    <s:Body>
        <u:AddPortMapping xmlns:u="` + upnpTargetService + `">
            <NewRemoteHost></NewRemoteHost>
            <NewExternalPort>` + strconv.Itoa(externalPort) + `</NewExternalPort>
            <NewProtocol>` + protocol + `</NewProtocol>
            <NewInternalPort>` + strconv.Itoa(internalPort) + `</NewInternalPort>
            <NewInternalClient>` + upnp.localIP + `</NewInternalClient>
            <NewEnabled>1</NewEnabled>
            <NewPortMappingDescription>` + description + `</NewPortMappingDescription>
            <NewLeaseDuration>0</NewLeaseDuration>
        </u:AddPortMapping>
    </s:Body>
	</s:Envelope>`

	//Create POST request for SOAP
	for _, controlURL := range upnp.controlURLs {
		soapRequest, err := http.NewRequest("POST", controlURL, bytes.NewBufferString(soapAddPortBody))
		if err != nil {
			upnp.Logger.Log(3, "Error creating SOAP request for: "+controlURL+" | Error:"+err.Error())
			continue
		}

		//Set headers
		soapRequest.Header.Set("Content-Type", "text/xml")
		soapRequest.Header.Set("SOAPAction", "\""+upnpTargetService+"#AddPortMapping\"")

		//Send request
		soapClient := &http.Client{Timeout: 5 * time.Second}
		soapResponce, err := soapClient.Do(soapRequest)
		if err != nil {
			upnp.Logger.Log(3, "Error sending SOAP request for: "+controlURL+" | Error:"+err.Error())
			continue
		}
		defer soapResponce.Body.Close()

		//Check if created successfully
		if soapResponce.StatusCode == http.StatusOK {
			upnp.mappedUrls.Set(externalPort, protocol)
			upnp.Logger.Log(2, "Successfully created UPnP at: "+controlURL)
		} else {
			soapBodyError, _ := io.ReadAll(soapResponce.Body)
			upnp.Logger.Log(3, "Error creating UPnP for: "+controlURL+" | Error code:"+strconv.Itoa(soapResponce.StatusCode)+" | Error message: "+string(soapBodyError))
		}
	}
	return nil
}

/*
RemoveUPnPPort removes UPnP mapping to all avaliable control URLs
*/
func (upnp *UPnPServiceManager) RemoveUPnPPort(externalPort int, protocol string) error {
	if len(upnp.controlURLs) == 0 {
		upnp.Logger.Log(3, "No UPnP control URLs found!")
		return errors.New("no control urls found")
	}
	if !upnp.mappedUrls.Has(externalPort) {
		upnp.Logger.Log(1, "This external port is not registered, error may occur.")
	}

	//Create SOAP body
	upnp.Logger.Log(1, "Removing UPnP port for external port: "+strconv.Itoa(externalPort)+"  with protocol: "+protocol+"...")
	soapRemovePortBody := `<?xml version="1.0"?>
    <s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"
      s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
      <s:Body>
        <u:DeletePortMapping xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1">
          <NewRemoteHost></NewRemoteHost>
          <NewExternalPort>` + strconv.Itoa(externalPort) + `</NewExternalPort>
          <NewProtocol>` + protocol + `</NewProtocol>
        </u:DeletePortMapping>
      </s:Body>
    </s:Envelope>`

	//Create POST request for SOAP
	for _, controlURL := range upnp.controlURLs {
		soapRequest, err := http.NewRequest("POST", controlURL, bytes.NewBufferString(soapRemovePortBody))
		if err != nil {
			upnp.Logger.Log(3, "Error creating SOAP request for: "+controlURL+" | Error:"+err.Error())
			continue
		}

		//Set headers
		soapRequest.Header.Set("Content-Type", "text/xml")
		soapRequest.Header.Set("SOAPAction", "\""+upnpTargetService+"#DeletePortMapping\"")

		//Send request
		soapClient := &http.Client{Timeout: 5 * time.Second}
		soapResponce, err := soapClient.Do(soapRequest)
		if err != nil {
			upnp.Logger.Log(3, "Error sending SOAP request for: "+controlURL+" | Error:"+err.Error())
			continue
		}
		defer soapResponce.Body.Close()

		//Check if removed successfully
		if soapResponce.StatusCode == http.StatusOK {
			upnp.mappedUrls.Delete(externalPort)
			upnp.Logger.Log(2, "Successfully removed UPnP at: "+controlURL)
		} else {
			soapBodyError, _ := io.ReadAll(soapResponce.Body)
			upnp.Logger.Log(3, "Error removing UPnP for: "+controlURL+" | Error code:"+strconv.Itoa(soapResponce.StatusCode)+" | Error message: "+string(soapBodyError))
		}
	}
	return nil
}

/*
Shutdown shuts down all open UPnP ports
*/
func (upnp *UPnPServiceManager) Shutdown() {
	upnp.Logger.Log(2, "Shutting down UPnP Service manager...")
	for _, val := range upnp.mappedUrls.GetData() {
		upnp.RemoveUPnPPort(val.Key, val.Value)
	}
	upnp.Logger.Log(2, "Shutting down UPnP Service manager complete.")
}

/*
GetThisComputerLocalIP gets this PC local IP
*/
func GetThisComputerLocalIP() (string, error) {
	//Get all adresses
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	//Sort addresses
	for _, address := range addresses {
		//Check if address is not loopback
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no local IP found")
}

/*
GetRouterPublicIP gets router public IP using SOAP
*/
func (upnp *UPnPServiceManager) GetRouterPublicIP() ([]string, error) {
	if len(upnp.controlURLs) == 0 {
		upnp.Logger.Log(3, "No UPnP control URLs found!")
		return nil, errors.New("no control urls found")
	}

	//Create SOAP body
	upnp.Logger.Log(1, "Getting router public IP via SOAP...")
	soapAddPortBody := `<?xml version="1.0"?>
	<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
		<s:Body>
			<u:GetExternalIPAddress xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1"/>
		</s:Body>
	</s:Envelope>`

	//Create POST request for SOAP
	result := make([]string, 0)
	for _, controlURL := range upnp.controlURLs {
		soapRequest, err := http.NewRequest("POST", controlURL, bytes.NewBufferString(soapAddPortBody))
		if err != nil {
			upnp.Logger.Log(3, "Error creating SOAP request for: "+controlURL+" | Error:"+err.Error())
			continue
		}

		//Set headers
		soapRequest.Header.Set("Content-Type", "text/xml")
		soapRequest.Header.Set("SOAPAction", "\""+upnpTargetService+"#GetExternalIPAddress\"")

		//Send request
		soapClient := &http.Client{Timeout: 5 * time.Second}
		soapResponce, err := soapClient.Do(soapRequest)
		if err != nil {
			upnp.Logger.Log(3, "Error sending SOAP request for: "+controlURL+" | Error:"+err.Error())
			continue
		}
		defer soapResponce.Body.Close()

		//Check if created successfully
		if soapResponce.StatusCode == http.StatusOK {
			soapBody, _ := io.ReadAll(soapResponce.Body)
			soapBodyString := string(soapBody)
			beginIndex := strings.Index(soapBodyString, "<NewExternalIPAddress>")
			if beginIndex == -1 {
				//No valid IP
				upnp.Logger.Log(3, "Error finding IP in SOAP responce")
				continue
			}
			endIndex := strings.Index(soapBodyString, "</NewExternalIPAddress>")

			//Get IP
			beginIndex += 22 // Offset of begin
			ip := soapBodyString[beginIndex:endIndex]
			result = append(result, ip)
			upnp.Logger.Log(2, "Successfully got router public IP: "+ip+" at: "+controlURL)
		} else {
			soapBodyError, _ := io.ReadAll(soapResponce.Body)
			upnp.Logger.Log(3, "Error creating SOAP request for: "+controlURL+" | Error code:"+strconv.Itoa(soapResponce.StatusCode)+" | Error message: "+string(soapBodyError))
		}
	}
	return result, nil
}
