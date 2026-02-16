package webtools

/*
BufferSize is shared buffer size over all tools
*/
const BufferSize = 1024 * 16

type NetworkStatus uint8

/*
ConnectStatus is status of connect
*/
const NoneNetworkStatus NetworkStatus = 0

/*
ReadDataStatus is status of read
*/
const ReadDataStatus NetworkStatus = 4

/*
ConnectStatus is status of connect
*/
const ConnectStatus NetworkStatus = 1

/*
DisconnectStatus is status of disconnect
*/
const DisconnectStatus NetworkStatus = 2

/*
FinishedReadFuncStatus is status of finished reading of one read function and switching to other
*/
const FinishedReadFuncStatus NetworkStatus = 3
