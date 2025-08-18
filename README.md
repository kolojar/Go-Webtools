# Go Web Tools
- Go Web servers and clients
- Needs complete rewrite

# TODO
- [X] TCP Server
- [X] TCP Client
- [X] UDP Server
- [X] UDP Client
- [X] HTTP Server
- [ ] HTTP Client
- [X] HTTP Internal WebSocket Server (no frames, encoding, ...)
- [X] HTTP Internal WebSocket Client
- [X] HTTP WebSocket Server
- [X] HTTP WebSocket Client
- [ ] TCP Bridge
- [ ] UDP Bridge
- [ ] TCP HTTP Proxy Server
- [ ] TCP HTTP Proxy Client
- [ ] UDP HTTP Proxy Server
- [ ] UDP HTTP Proxy Client
- [ ] Encryption
    - [X] TCP Server
    - [X] TCP Client
    - [ ] UDP Server
    - [ ] UDP Client
    - [ ] HTTP Server
    - [ ] HTTP Client
    - [ ] HTTP Internal WebSocket Server (no frames, encoding, ...)
    - [ ] HTTP Internal WebSocket Client
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

# Fixes
- [X] Fix map sync error
- [ ] Add address fix to TCP server. Instead whole address object use just string of address, apply this fix to all UDP components.