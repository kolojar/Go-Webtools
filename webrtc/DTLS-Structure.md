# DTLS
 - Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.1
 - Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.1
 - 1 byte type
 - 2 bytes version
 - 2 bytes epoch
 - 6 bytes sequence number (count of writes to UDP per epoch)
 - 2 bytes lenght
 - Data
 - Compare with [TLS](#tls)

## DTLS Handshake
 - Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.2
 - 1 byte type (Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.3.2)
 - **3 byte length**
 - **2 byte message sequence** (count of messages in this UDP message)
 - **3 byte fragment offset**
 - 3 byte fragment length
 - Data
 - Compare with [TLS](#tls-handshake)

## DTLS ClientHello structure
 - Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.1
 - 2 byte protocol version
 - 32 byte random
 - 1 byte session id length = x
 - x bytes session id
 - **1 byte cookie length = y**
 - **y bytes cookie**
 - Data
 - Compare with [TLS](#tls-clienthello-structure)

## DTLS ServerHello structure
 - Specification: https://datatracker.ietf.org/doc/html/rfc6347#section-4.2.1
 - 2 byte protocol version
 - 32 byte random
 - 1 byte session id length = x
 - x bytes session id
 - **1 byte cookie length = y**
 - **y bytes cookie**
 - Data
 - Compare with [TLS](#tls-serverhello-structure)
 
## DTLS Certificate structure
 - Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.2
 - Data
 - Compare with [TLS](#tls-certificate-structure)
 
# TLS
 - Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-6.2.1
 - 1 byte type
 - 2 bytes version (3,3)
 - 2 bytes length
 - Data
 
## TLS Handshake
 - Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4
 - 1 byte type
 - 3 bytes length
 - Data
 
## TLS ClientHello structure
 - Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.1.2
 - 2 byte protocol version
 - 32 byte random
 - 1 byte session id length = x
 - x bytes session id
 - Data
 
## TLS ServerHello structure
 - Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.1.3
 - 2 byte protocol version
 - 32 byte random
 - 1 byte session id length = x
 - x bytes session id
 - Data
 
 
## TLS Certificate structure
 - Specification: https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.2
 - Data