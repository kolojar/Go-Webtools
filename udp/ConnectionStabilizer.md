# Structures of packets
## Replay window (helper structure)
- 4 byte ACK right edge as uint32 in LittleEndian
- 8 byte ACK window (status of last 64 packets - if were recieved or not) as uint64 in LittleEndian

## Ping (1) / Pong (2)
- 1 byte Control sequence
- 12 byte Replay window
- 8 byte Timestamp in UnixMicro in LittleEndian

## Data recieved (ACK) frame (3)
- 1 byte Control sequence
- 4 byte ACK right edge as uint32 in LittleEndian
- 1 byte ACK window size (up to 32)
- up to 256 byte ACK window (status of last 2048 packets - if were recieved or not) as 32x uint64 in LittleEndian

## Data frame (4)
- 1 byte Control sequence
- x byte Data

## Data frame with resend (5)
- 1 byte Control sequence
- 12 byte Replay window
- 1-8 byte Sequence number in LittleEndian
- x byte Data

## Data frame with order instant (6)
- 1 byte Control sequence
- 1-8 byte Order number simple in LittleEndian
- x byte Data

## Data frame with order with timeout (7)
- 1 byte Control sequence
- 1-8 byte Order number simple in LittleEndian
- x byte Data

TODO:

## Data frame with order and resend (8)
- 1 byte Control sequence
- 12 byte Replay window
- 1-8 byte Sequence number in LittleEndian
- 1-8 byte Order number precise in LittleEndian
- x byte Data
