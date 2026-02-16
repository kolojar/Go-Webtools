package webtools

/*
BufferSize is shared buffer size over all tools
*/
const BufferSize = 1024 * 16

type NetworkStatus uint8

/*
ReadDataStatus is status of read
*/
const ReadDataStatus NetworkStatus = 3

/*
ConnectStatus is status of connect
*/
const ConnectStatus NetworkStatus = 0

/*
DisconnectStatus is status of disconnect
*/
const DisconnectStatus NetworkStatus = 1

/*
FinishedReadFuncStatus is status of finished reading of one read function and switching to other
*/
const FinishedReadFuncStatus NetworkStatus = 2
