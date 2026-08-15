package helpertools

import (
	"container/heap"
	"fmt"
	"sync"
)

// AllowNone disables this feature
const AllowNone PacketOrdererAllowMissingPackets = 0

// AllowInDump allows this feature in Dump function only
const AllowInDump PacketOrdererAllowMissingPackets = 1

// AllowInDumpAndPush allows this feature in Dump and Push functions.
//
// Warning: It completly disables heap, because it is not needed
const AllowInDumpAndPush PacketOrdererAllowMissingPackets = 2

// PacketOrdererAllowMissingPackets is helper type for missing packet level setting
type PacketOrdererAllowMissingPackets uint8

// packetOrdererHolder is helper struct for heap
type packetOrdererHolder[orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, dataType any] []KeyValuePair[orderNumberType, dataType]

// Len is the number of elements in the collection.
func (holder packetOrdererHolder[orderNumberType, dataType]) Len() int {
	return len(holder)
}

// Less reports whether the element with index i must sort before the element with index j.
func (holder packetOrdererHolder[orderNumberType, dataType]) Less(i, j int) bool {
	return IsSequenceNumberInFuture(holder[i].Key, holder[j].Key)
}

// Swap swaps the elements with indexes i and j.
func (holder packetOrdererHolder[orderNumberType, dataType]) Swap(i, j int) {
	holder[i], holder[j] = holder[j], holder[i]
}

// Push adds x to heap
func (holder *packetOrdererHolder[orderNumberType, dataType]) Push(x any) {
	*holder = append(*holder, x.(KeyValuePair[orderNumberType, dataType]))
}

// Pop returns last element
func (holder *packetOrdererHolder[orderNumberType, dataType]) Pop() any {
	item := (*holder)[holder.Len()-1]
	*holder = (*holder)[:holder.Len()-1]
	return item
}

// PacketOrderer is struct for packet ordering using heap
type PacketOrderer[orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, dataType any] struct {
	packetsHeap                 packetOrdererHolder[orderNumberType, dataType]
	mutex                       sync.Mutex
	allowMissingPackets         PacketOrdererAllowMissingPackets
	allowOlderPackets           bool
	nextOrderNumberToBeExported orderNumberType
	isFirst                     bool
}

// NewPacketOrderer initializes new packet orderer
func NewPacketOrderer[orderNumberType ~uint8 | ~uint16 | ~uint32 | ~uint64, dataType any](allowMissingPackets PacketOrdererAllowMissingPackets, allowOlderPackets bool, initialHeapSize uint) *PacketOrderer[orderNumberType, dataType] {
	//Setup heap if needed
	var packetsHeap packetOrdererHolder[orderNumberType, dataType] = nil
	if allowMissingPackets != AllowInDumpAndPush {
		packetsHeap = make(packetOrdererHolder[orderNumberType, dataType], 0, initialHeapSize)
	}

	//Setup orderer
	orderer := &PacketOrderer[orderNumberType, dataType]{
		packetsHeap:                 packetsHeap,
		allowMissingPackets:         allowMissingPackets,
		allowOlderPackets:           allowOlderPackets,
		mutex:                       sync.Mutex{},
		nextOrderNumberToBeExported: 0,
		isFirst:                     true,
	}

	//Setup heap if needed
	if allowMissingPackets != AllowInDumpAndPush {
		heap.Init(&orderer.packetsHeap)
	}
	return orderer
}

// Push pushes to queue if waiting for packet.
//
// Returns slice of data if packets get into sequence or nil
func (orderer *PacketOrderer[orderNumberType, dataType]) Push(orderNumber orderNumberType, data dataType) []dataType {
	return orderer.PushWithMissingPacketOption(orderNumber, data, orderer.allowMissingPackets)
}

// PushWithMissingPacketOption pushes to queue if waiting for packet.
//
// Returns slice of data if packets get into sequence or nil
func (orderer *PacketOrderer[orderNumberType, dataType]) PushWithMissingPacketOption(orderNumber orderNumberType, data dataType, allowMissingPackets PacketOrdererAllowMissingPackets) []dataType {
	//Lock mutex
	orderer.mutex.Lock()
	defer orderer.mutex.Unlock()

	//Handle old packets
	if !orderer.isFirst && !orderer.allowOlderPackets && !IsSequenceNumberInFuture(orderer.nextOrderNumberToBeExported-1, orderNumber) {
		//Drop old packet
		fmt.Println("Dropping:", orderNumber, " because too old:", orderer.nextOrderNumberToBeExported)
		return nil
	}

	wasFirst := orderer.isFirst
	if orderer.isFirst {
		orderer.isFirst = false
	}

	//Handle forced missing packets
	if allowMissingPackets == AllowInDumpAndPush {
		fmt.Println("Exporting:", orderNumber, " because option missing packets is active")
		orderer.nextOrderNumberToBeExported = orderNumber + 1
		return []dataType{data}
	}

	//Handle first packet
	if wasFirst {
		if orderNumber == 0 {
			//First packet
			fmt.Println("Exporting:", orderNumber, " because is first")
			orderer.nextOrderNumberToBeExported = orderNumber + 1
			return []dataType{data}
		}

		//Out of order = store
		heap.Push(&orderer.packetsHeap, KeyValuePair[orderNumberType, dataType]{Key: orderNumber, Value: data})
		return nil
	}

	//Check if packets are 1 order number apart
	if IsSequenceNumberInFuture(orderer.nextOrderNumberToBeExported-1, orderNumber) {
		fmt.Println("Trying exporting:", orderNumber, " because bigger that last exported:", orderer.nextOrderNumberToBeExported)
		//orderer.lastOrderNumberExported = orderNumber
		result := []dataType{}
		wasOk := true
		orderNumberExported := false
		for wasOk {
			wasOk = false

			//Check next sequences
			if len(orderer.packetsHeap) != 0 && orderer.packetsHeap[0].Key == orderer.nextOrderNumberToBeExported {
				pop := heap.Pop(&orderer.packetsHeap).(KeyValuePair[orderNumberType, dataType])
				fmt.Println("Exporting:", pop.Key, " because smaller than:", orderNumber)
				orderer.nextOrderNumberToBeExported = pop.Key + 1
				result = append(result, pop.Value)
				wasOk = true
				continue
			}

			//Export current
			if !orderNumberExported && orderer.nextOrderNumberToBeExported == orderNumber {
				result = append(result, data)
				//if IsSequenceNumberInFuture(orderer.lastOrderNumberExported, orderNumber) {
				fmt.Println("Exporting:", orderNumber, " current sequence number:", orderNumber)
				orderer.nextOrderNumberToBeExported = orderNumber + 1
				//}
				wasOk = true
				orderNumberExported = true
			}
		}

		//Validate result
		if len(result) > 0 {
			return result
		}
	}

	//No valid export option = store
	fmt.Println("Storing:", orderNumber, "because not valid in context of:", orderer.nextOrderNumberToBeExported)
	heap.Push(&orderer.packetsHeap, KeyValuePair[orderNumberType, dataType]{Key: orderNumber, Value: data})
	return nil
}

// Dump returns all valid data until orderNumber is reached (included)
func (orderer *PacketOrderer[orderNumberType, dataType]) Dump(orderNumber orderNumberType) []dataType {
	//Lock mutex
	orderer.mutex.Lock()
	defer orderer.mutex.Unlock()

	//Check if slice has items
	if len(orderer.packetsHeap) == 0 || !IsSequenceNumberInFuture(orderer.nextOrderNumberToBeExported-1, orderNumber) {
		return nil
	}

	//Handle forced missing packets
	if orderer.allowMissingPackets == AllowInDump || orderer.allowMissingPackets == AllowInDumpAndPush {
		result := make([]dataType, 0)
		for len(orderer.packetsHeap) != 0 {
			if IsSequenceNumberInFuture(orderNumber, orderer.packetsHeap[0].Key) {
				break
			}
			pop := heap.Pop(&orderer.packetsHeap).(KeyValuePair[orderNumberType, dataType])
			fmt.Println("Dumping(can skip):", pop.Key, "because in sequence of:", orderNumber)
			result = append(result, pop.Value)
			orderer.nextOrderNumberToBeExported = pop.Key + 1
		}
		return result
	}

	//Check if can export
	if orderer.nextOrderNumberToBeExported == orderer.packetsHeap[0].Key {
		if len(orderer.packetsHeap) == 1 {
			//Export only one
			fmt.Println("Dumping(one):", orderer.packetsHeap[0].Key, "because in sequence of:", orderNumber)
			orderer.nextOrderNumberToBeExported = orderer.packetsHeap[0].Key + 1
			result := []dataType{heap.Pop(&orderer.packetsHeap).(KeyValuePair[orderNumberType, dataType]).Value}
			return result
		}

		//Export more
		result := make([]dataType, 0)
		for len(orderer.packetsHeap) != 0 && orderer.packetsHeap[0].Key == orderer.nextOrderNumberToBeExported {
			if IsSequenceNumberInFuture(orderNumber, orderer.packetsHeap[0].Key) {
				break
			}
			pop := heap.Pop(&orderer.packetsHeap).(KeyValuePair[orderNumberType, dataType])
			fmt.Println("Dumping(cant skip):", pop.Key, "because in sequence of:", orderNumber)
			orderer.nextOrderNumberToBeExported = pop.Key + 1
			result = append(result, pop.Value)
		}
		return result
	}
	return nil
}

// DumpAll tries do Dump all packets as possible (works same as Dump but without orderNumber limit)
func (orderer *PacketOrderer[orderNumberType, dataType]) DumpAll() []dataType {
	//Lock mutex
	orderer.mutex.Lock()
	defer orderer.mutex.Unlock()

	//Check if slice has items
	if len(orderer.packetsHeap) == 0 {
		return nil
	}

	//Handle forced missing packets
	if orderer.allowMissingPackets == AllowInDump || orderer.allowMissingPackets == AllowInDumpAndPush {
		result := make([]dataType, 0)
		for len(orderer.packetsHeap) != 0 {
			pop := heap.Pop(&orderer.packetsHeap).(KeyValuePair[orderNumberType, dataType])
			fmt.Println("Dumping all(can skip):", pop.Key)
			result = append(result, pop.Value)
			orderer.nextOrderNumberToBeExported = pop.Key + 1
		}
		return result
	}

	//Check if can export
	if orderer.nextOrderNumberToBeExported == orderer.packetsHeap[0].Key {
		if len(orderer.packetsHeap) == 1 {
			//Export only one
			fmt.Println("Dumping all(one):", orderer.packetsHeap[0].Key)
			orderer.nextOrderNumberToBeExported = orderer.packetsHeap[0].Key + 1
			result := []dataType{heap.Pop(&orderer.packetsHeap).(KeyValuePair[orderNumberType, dataType]).Value}
			return result
		}

		//Export more
		result := make([]dataType, 0)
		for len(orderer.packetsHeap) != 0 && orderer.packetsHeap[0].Key == orderer.nextOrderNumberToBeExported {
			pop := heap.Pop(&orderer.packetsHeap).(KeyValuePair[orderNumberType, dataType])
			fmt.Println("Dumping all(cant skip):", pop.Key)
			orderer.nextOrderNumberToBeExported = pop.Key + 1
			result = append(result, pop.Value)
		}
		return result
	}
	return nil
}

// GetMissingOrderNumbers returns all missing order number between last lastOrderNumberExported (excluded) and orderNumber (included).
// When nothing is found, nil is returned.
func (orderer *PacketOrderer[orderNumberType, dataType]) GetMissingOrderNumbers(orderNumber orderNumberType) []orderNumberType {
	//Lock mutex
	orderer.mutex.Lock()
	defer orderer.mutex.Unlock()

	//Check if orderNumber is in future of lastOrderNumberExported
	if !IsSequenceNumberInFuture(orderer.nextOrderNumberToBeExported, orderNumber) {
		return nil
	}

	//Check if empty
	if len(orderer.packetsHeap) == 0 {
		distance := orderNumber - orderer.nextOrderNumberToBeExported
		if distance == 0 {
			return nil
		}

		//Process missing
		result := make([]orderNumberType, 0)
		for i := orderNumberType(1); i <= distance; i++ {
			result = append(result, orderer.nextOrderNumberToBeExported+i)
		}
		return result
	}

	//Get present packets
	var present map[orderNumberType]struct{} = nil
	if len(orderer.packetsHeap) > 32 {
		present = make(map[orderNumberType]struct{}, len(orderer.packetsHeap))
		for _, v := range orderer.packetsHeap {
			present[v.Key] = struct{}{}
		}
	}

	//Get missing packets
	var result []orderNumberType = nil
	for order := orderer.nextOrderNumberToBeExported + 1; order != orderNumber+1; order++ {
		//Check if exists
		exists := false
		if present != nil {
			_, exists = present[order]
		} else {
			for _, v := range orderer.packetsHeap {
				if v.Key == order {
					exists = true
					break
				}
			}
		}

		if !exists {
			//Packet not found
			if result == nil {
				result = make([]orderNumberType, 0)
			}
			result = append(result, order)
		}
	}
	return result
}

// IsSequenceNumberInFuture checks if sequenceNumber is in future of rightEdge
func IsSequenceNumberInFuture[T ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint](rightEdge T, sequenceNumber T) bool {
	jump := sequenceNumber - rightEdge
	return jump > 0 && jump <= ((T(0)-1)>>1)
}
