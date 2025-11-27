# Go Web Tools
- This package provides tools for UDP, TCP, HTTP, WebSockets, Peer to Peer (P2P) and some proxies (translating one type of traffic to another - one protocol to another)
- Go Web servers and clients
- Run this to update submodules: ```git submodule update --recursive --remote``

# TODO
- [X] TCP Server
- [X] TCP Client
- [X] UDP Server
- [X] UDP Client
- [X] HTTP Server
- [X] HTTP Client
- [X] HTTP WebSocket Server
- [X] HTTP WebSocket Client
- [ ] TCP Bridge
- [ ] UDP Bridge
- [X] TCP HTTP Proxy Server
- [X] TCP HTTP Proxy Client
- [X] UDP HTTP Proxy Server
- [X] UDP HTTP Proxy Client
- [ ] Encryption
    - [X] TCP Server
    - [X] TCP Client
    - [ ] UDP Server
    - [ ] UDP Client
    - [ ] HTTP Server
    - [ ] HTTP Client
    - [ ] HTTP WebSocket Server
    - [ ] HTTP WebSocket Client
    - [ ] TCP Bridge
    - [ ] UDP Bridge
    - [ ] TCP HTTP Proxy Server
    - [ ] TCP HTTP Proxy Client
    - [ ] UDP HTTP Proxy Server
    - [ ] UDP HTTP Proxy Client
- [X] Add some kind of connection merger and splitter
- [ ] Add encryption to all servers and clients
- [ ] P2P

# Fixes
- [X] Fix map sync error
- [X] Add address fix to TCP server. Instead whole address object use just string of address, apply this fix to all UDP components.
- [ ] Add dynamic cleanups to PROXIES!!!
- [ ] Fix P2P Punching
- [ ] Fix P2P Punching at line 258