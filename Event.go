package webtools

/*
EventListener is function declaration for Event Listener
*/
type EventListener[EventDataType any] func(EventData EventDataType)

/*
Event is basis scruct for event. It can be fired. Fired event will trigger all listeners.
*/
type Event[EventDataType any] struct {
	Listeners SafeMap[string, KeyValuePair[EventListener[EventDataType], bool]]
}

/*
AddListener add listener for event. Retuns listenerID
*/
func (event *Event[EventDataType]) AddListener(listener EventListener[EventDataType], callUsingGoRoutine bool) string {
	if listener == nil {
		return ""
	}
	id := GenerateRandomID()
	event.Listeners.Set(id, KeyValuePair[EventListener[EventDataType], bool]{Key: listener, Value: callUsingGoRoutine})
	return id
}

/*
RemoveListener removes listener for event. Needs listenerID
*/
func (event *Event[EventDataType]) RemoveListener(listenerID string) {
	event.Listeners.Delete(listenerID)
}

/*
Fire fires (runs) the event and calls all
*/
func (event *Event[EventDataType]) Fire(data EventDataType, callGoRoutinesFirst bool) {
	if callGoRoutinesFirst {
		normalFunctions := make([]EventListener[EventDataType], 0)
		for _, v := range event.Listeners.GetValues() {
			if v.Value {
				go v.Key(data)
			} else {
				normalFunctions = append(normalFunctions, v.Key)
			}
		}
		for _, v := range normalFunctions {
			v(data)
		}
	} else {
		for _, v := range event.Listeners.GetValues() {
			if v.Value {
				go v.Key(data)
			} else {
				v.Key(data)
			}
		}
	}
}
