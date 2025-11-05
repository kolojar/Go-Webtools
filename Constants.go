package webtools

/*
BufferSize is shared buffer size over all tools
*/
const BufferSize = 1024 * 16

/*
ReadDataStatus is status of read
*/
const ReadDataStatus = uint8(3)

/*
ConnectStatus is status of connect
*/
const ConnectStatus = uint8(0)

/*
DisconnectStatus is status of disconnect
*/
const DisconnectStatus = uint8(1)

/*
FinishedReadFuncStatus is status of finished reading of one read function and switching to other
*/
const FinishedReadFuncStatus = uint8(2)
