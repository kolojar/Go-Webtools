package p2pTools

import (
	"bufio"
	"encoding/xml"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"webtools"
)

type UPnPXMLRoot struct {
	XMLName xml.Name      `xml:"root"`
	Device  UPnPXMLDevice `xml:"device"`
}
type UPnPXMLDevice struct {
	XMLName  xml.Name         `xml:"device"`
	Services []UPnPXMLService `xml:"serviceList>service"`
}
type UPnPXMLService struct {
	XMLName     xml.Name `xml:"service"`
	ServiceType string   `xml:"serviceType"`
	ControlURL  string   `xml:"controlURL"` // Sem se posílají SOAP požadavky
}

const UPnPTargetService = "urn:schemas-upnp-org:service:WANIPConnection:1"

type UPnPServiceManager struct {
	controlUrls []string
	logger      *webtools.ConsoleLogger
}

func (upnp *UPnPServiceManager) SetupUPnP() error {
	//New client for SSDP service that works in UPnP server. It is located on that specific IP and port (it is local, not some internet service)
	addrSSDP, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return err
	}
	connSSDP, err := net.DialUDP("udp4", nil, addrSSDP)
	if err != nil {
		return err
	}
	defer connSSDP.Close()

	//Create search request
	requestMSearch := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 3\r\n" + // Max. doba čekání na odpověď v sekundách
		"ST: ssdp:all\r\n" + // Cíl hledání
		"\r\n"

	//Send request
	_, err = connSSDP.Write([]byte(requestMSearch))
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
		n, _, err := connSSDP.ReadFrom(buffer)
		if err != nil {
			return err
		}

		//Process responce
		responce := buffer[:n]
		var responceLocationURL string = ""
		readerResponce := bufio.NewReader(strings.NewReader(string(responce)))
		for {
			//Get location
			line, err := readerResponce.ReadString('\n')
			if err != nil {
				upnp.logger.Log(3, "Error reading responce for location: "+err.Error())
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
			continue
		}

		//Check if already viewed
		_, exists := locationsOfSSDP[responceLocationURL]
		if exists {
			//Ignore already viewed request
			continue
		}

		//Not viewed request
		locationsOfSSDP[responceLocationURL] = struct{}{}
		if !strings.Contains(strings.ToLower(responceLocationURL), "wanip") && !strings.Contains(strings.ToLower(responceLocationURL), "internetgatewaydevice") {
			//Not valid request
			continue
		}

		//Get base URL
		splitLocationsOfSSDP := strings.Split(responceLocationURL, "/")
		baseLocationUrl := ""
		if len(splitLocationsOfSSDP) >= 3 {
			baseLocationUrl = strings.Join(splitLocationsOfSSDP[:3], "/")
		} else {
			baseLocationUrl = responceLocationURL
		}

		//Get XML
		responceBaseLocationXML, err := http.Get(baseLocationUrl)
		if err != nil {
			upnp.logger.Log(3, "Error getting XML responce: "+err.Error())
			continue
		}
		var baseLocationXMLData []byte
		if responceBaseLocationXML.StatusCode == http.StatusOK {
			baseLocationXMLData, err = io.ReadAll(responceBaseLocationXML.Body)
			if err != nil {
				upnp.logger.Log(3, "Error getting XML data: "+err.Error())
				continue
			}
		}

		//Get controlURL
		var xmlRoot UPnPXMLRoot
		var controlURL string = ""
		err = xml.Unmarshal(baseLocationXMLData, &xmlRoot)
		if err != nil {
			//Ignore invalid
			upnp.logger.Log(3, "Error unmarshalling XML data: "+err.Error())
			continue
		}
		for _, service := range xmlRoot.Device.Services {
			if service.ServiceType == UPnPTargetService {
				//Service is valid type, check if is relative or absolute URL
				if strings.HasPrefix(service.ControlURL, "http") {
					controlURL = service.ControlURL
				} else {
					controlURL = strings.TrimSuffix(baseLocationUrl, "/") + service.ControlURL
				}
			}
		}
		if controlURL == "" {
			//No valid service was found
			upnp.logger.Log(3, "No valid controlUrl was found.")
			continue
		}
		upnp.controlUrls = append(upnp.controlUrls, controlURL)
	}
}
