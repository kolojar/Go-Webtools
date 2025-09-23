# P2P Connect chart and verification with Examples
0. Command structure
- CONNECT_PEER;SOURCE_ID;TARGET_ID;SERVER_ID;ENCRYPTION_TEST
- CONNECT_WAITING;CONN_ID
- CONNECT_REQUEST;CONN_ID;SOURCE_ID;TARGET_ID;SERVER_ID;ENCRYPTION_TEST
- CONNECT_RESULT;ok/err;SOURCE_ID;TARGET_ID;SERVER_ID;ENCRYPTION_TEST_DECRYPTED
- CONNECT_BEGIN_WAITING;CONN_ID
- CONNECT_REQUEST_VALIDATION;ok/err;CONN_ID;SOURCE_ID;TARGET_ID;SERVER_ID;ENCRYPTION_TEST_DECRYPTED
- CONNECT_VALIDATION;ok/err;CONN_ID;SOURCE_ID;TARGET_ID;SERVER_ID
1. Connect to coordination server from clients 
```
Client A -> Server + Client B -> Server
```
2. Server stores IP addresses from clients
```
Name     -> Id -> IP address
------------------------------
Server   -> 0  -> 127.0.0.1
Client A -> 1  -> 192.168.0.6
Client B -> 2  -> 192.168.0.25
```
3. Client A wants to connect to Client B
4. Client A sends request (no encryption in this example)
```
Client A -> Server = CONNECT_PEER;1;2;0;abcd
```
5. Server verifies all IDs and creates new connection with phase 1
```
Id  -> Source   -> Target
---------------------------
100 -> Client A -> Client B
```
6. Server sends:
```
Server -> Client A = CONNECT_WAITING;100
Server -> Client B = CONNECT_REQUEST;100;1;2;0;abcd
```
7. Client A now waits and Client B comes to play
	- Checks ID of server and it self
	- Tries to decrypt test for encryption verification 
	- Sends result back to server
```
Client B -> Server = CONNECT_RESULT;ok;1;2;0;abcd
Server -> Client B = CONNECT_BEGIN_WAITING;100
```
8. Server verifies all IDs and sets connection phase to 2 and sends result to SOURCE
```
Server -> Client A = CONNECT_REQUEST_VALIDATION;ok;100;1;2;0;abcd
```
9. Client A processes validates result
	- Checks ID of server and it self
	- Checks decrypted message
	- Sends ready result back to server
```
Client A -> Server = CONNECT_VALIDATION;ok;100;1;2;0
Server -> Client A = CONNECT_BEGIN_WAITING;100
```
10. Server check data for one last time and then setups connection
```
Server -> Client A = CONNECT_START;time
Server -> Client B = CONNECT_START;time
```
- Server sends time T+5 seconds
- Server checks ServerID, Source and Target IDs
11. Clients starts connecting to each other
12. A) Retry connection
```
Client A/B -> Server = CONNECT_RETRY
Server -> Client A = CONNECT_RETRY
Server -> Client B = CONNECT_RETRY
Server -> Client A = CONNECT_START;time
Server -> Client B = CONNECT_START;time
```
12. B) Established connection - Start exchanging data trought peer
13. A) Disconnect
```
Client A/B -> Server = CLOSE_PEER
Server -> Client A = CLOSE_PEER
Server -> Client B = CLOSE_PEER
```
13. B) Connection crashed, try retry
13. C) Connection crashed, try RELAY server
```
Client B -> Server = CONNECT_RELAY
Server -> Client A = REQUEST_RELAY
```
14. If RELAY available, switch to Binary frames = Just move frame data from A to B
15. If RELAY unavalible (Server or Client A) = Close connection and cancel request
```
Server -> Client B = ERROR_RELAY
```
16. To close relay use:
```
Client A -> Server = CLOSE_RELAY
```
17. If no other clients are connected to same server as RELAY, request close of RELAY server
```
Server -> Client B = FINISH_RELAY;connID
```