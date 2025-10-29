
import socket
import re

def discover_upnp_gateway():
    msg = (
        'M-SEARCH * HTTP/1.1\r\n'
        'HOST:239.255.255.250:1900\r\n'
        'MAN:"ssdp:discover"\r\n'
        'MX:1\r\n'
        'ST:urn:schemas-upnp-org:device:InternetGatewayDevice:1\r\n'
        '\r\n'
    )

    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_UDP)
    sock.settimeout(2)
    sock.sendto(msg.encode('utf-8'), ('239.255.255.250', 1900))

    try:
        while True:
            data, _ = sock.recvfrom(65507)
            resp = data.decode('utf-8', errors='ignore')
            if "LOCATION:" in resp:
                loc = re.search(r'LOCATION:\s*(.*)\r\n', resp, re.IGNORECASE)
                if loc:
                    return loc.group(1).strip()
    except socket.timeout:
        return None


import urllib.request
import xml.etree.ElementTree as ET

def get_control_url(description_url):
    with urllib.request.urlopen(description_url) as r:
        xml_data = r.read()

    root = ET.fromstring(xml_data)
    for service in root.iter('service'):
        stype = service.find('serviceType')
        if stype is not None and 'WANIPConnection' in stype.text:
            ctrl = service.find('controlURL').text
            base = description_url.rsplit('/', 1)[0]
            return base + ctrl
    return None

import http.client

def add_port_mapping(control_url, external_port, internal_port, internal_client, protocol='TCP'):
    soap_body = f'''<?xml version="1.0"?>
    <s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"
      s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
      <s:Body>
        <u:AddPortMapping xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1">
          <NewRemoteHost></NewRemoteHost>
          <NewExternalPort>{external_port}</NewExternalPort>
          <NewProtocol>{protocol}</NewProtocol>
          <NewInternalPort>{internal_port}</NewInternalPort>
          <NewInternalClient>{internal_client}</NewInternalClient>
          <NewEnabled>1</NewEnabled>
          <NewPortMappingDescription>PythonTest</NewPortMappingDescription>
          <NewLeaseDuration>0</NewLeaseDuration>
        </u:AddPortMapping>
      </s:Body>
    </s:Envelope>'''

    parsed = urllib.parse.urlparse(control_url)
    conn = http.client.HTTPConnection(parsed.hostname, parsed.port or 80)
    conn.request(
        "POST", parsed.path,
        body=soap_body,
        headers={
            "Content-Type": "text/xml",
            "SOAPAction": '"urn:schemas-upnp-org:service:WANIPConnection:1#AddPortMapping"'
        }
    )
    resp = conn.getresponse()
    return resp.status, resp.read().decode()


def delete_port_mapping(control_url, external_port, protocol='TCP'):
    soap_body = f'''<?xml version="1.0"?>
    <s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"
      s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
      <s:Body>
        <u:DeletePortMapping xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1">
          <NewRemoteHost></NewRemoteHost>
          <NewExternalPort>{external_port}</NewExternalPort>
          <NewProtocol>{protocol}</NewProtocol>
        </u:DeletePortMapping>
      </s:Body>
    </s:Envelope>'''

    parsed = urllib.parse.urlparse(control_url)
    conn = http.client.HTTPConnection(parsed.hostname, parsed.port or 80)
    conn.request(
        "POST", parsed.path,
        body=soap_body,
        headers={
            "Content-Type": "text/xml",
            "SOAPAction": '"urn:schemas-upnp-org:service:WANIPConnection:1#DeletePortMapping"'
        }
    )
    resp = conn.getresponse()
    return resp.status, resp.read().decode()

import time
if __name__ == "__main__":
    loc = discover_upnp_gateway()
    if not loc:
        print("No UPnP gateway found.")
    else:
        print("Found gateway:", loc)
        ctrl = "http://192.168.0.1:1900/vdavf/ctl/IPConn"
        print("Control URL:", ctrl)

        local_ip = socket.gethostbyname(socket.gethostname())

        # Add port mapping
        status, body = add_port_mapping(ctrl, 5555, 5555, local_ip)
        print("Add mapping:", status, body)
        time.sleep(10)

        # Delete it
        status, body = delete_port_mapping(ctrl, 5555)
        print("Delete mapping:", status, body)
