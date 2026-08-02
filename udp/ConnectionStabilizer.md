# Structures of packets
## Replay window (helper structure)
- 8 byte ACK window (status of last 64 packets - if were recieved or not) as uint64 in LittleEndian
- 4 byte ACK right edge as uint32 in LittleEndian

## Ping (1) / Pong (2)
- 1 byte Control sequence
- 12 byte Replay window
- 8 byte Timestamp in UnixMicro in LittleEndian

## Data recieved (ACK) frame (3)
- 1 byte Control sequence
- 12 byte Replay window

## Data frame (4)
- 1 byte Control sequence
- x byte Data

TODO: Need to fix ACK frame

## Data frame with resend (5)
- 1 byte Control sequence
- 12 byte Replay window
- 4 byte Sequence number in LittleEndian
- x byte Data

TODO:

## Data frame with order instant (6)
- 1 byte Control sequence
- 4 byte Order number simple in LittleEndian
- x byte Data

## Data frame with order with timeout (7)
- 1 byte Control sequence
- 4 byte Order number simple in LittleEndian
- x byte Data

## Data frame with order and resend (8)
- 1 byte Control sequence
- 12 byte Replay window
- 4 byte Sequence number in LittleEndian
- 4 byte Order number precise in LittleEndian
- x byte Data
