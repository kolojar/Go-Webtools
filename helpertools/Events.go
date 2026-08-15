package helpertools

// Event is base generic Event struct, uses Lazy construction
type Event struct {
	callers SafeSlice[ThreeValuePair[string, bool, func(args ...any)]]
}

// AddEventListerner adds event listerner for this event, returns id of listener
func (event *Event) AddEventListerner(callInGorountine bool, f func(args ...any)) string {
	if event.callers.IsNil() {
		event.callers = MakeSafeSlice[ThreeValuePair[string, bool, func(args ...any)]]()
	}
	id := GenerateRandomID()
	event.callers.Append(ThreeValuePair[string, bool, func(args ...any)]{
		A: id,
		B: callInGorountine,
		C: f,
	})
	return id
}

// RemoveEventListener removes event listerer for this event identified by id
func (event *Event) RemoveEventListener(id string) {
	if event.callers.IsNil() {
		return
	}
	event.callers.Remove(func(v ThreeValuePair[string, bool, func(args ...any)]) bool {
		return v.A == id
	})
}

// CallEvent calls all event listeners
func (event *Event) CallEvent(args ...any) {
	if event.callers.IsNil() {
		return
	}
	for _, v := range event.callers.GetValues() {
		if v.B {
			go v.C(args...)
		} else {
			v.C(args...)
		}
	}
}

// Event0 is struct build on Event with 0 arguments in called function
type Event0 struct {
	event Event
}

// AddEventListerner adds event listerner for this event, returns id of listener
func (event *Event0) AddEventListerner(callInGorountine bool, f func()) string {
	return event.event.AddEventListerner(callInGorountine, func(args ...any) {
		f()
	})
}

// RemoveEventListener removes event listerer for this event identified by id
func (event *Event0) RemoveEventListener(id string) {
	event.event.RemoveEventListener(id)
}

// CallEvent calls all event listeners
func (event *Event0) CallEvent() {
	event.event.CallEvent()
}

// Event1 is struct build on Event with 1 argument in called function
type Event1[T1 any] struct {
	event Event
}

// AddEventListerner adds event listerner for this event, returns id of listener
func (event *Event1[T1]) AddEventListerner(callInGorountine bool, f func(arg1 T1)) string {
	return event.event.AddEventListerner(callInGorountine, func(args ...any) {
		f(args[0].(T1))
	})
}

// RemoveEventListener removes event listerer for this event identified by id
func (event *Event1[T1]) RemoveEventListener(id string) {
	event.event.RemoveEventListener(id)
}

// CallEvent calls all event listeners
func (event *Event1[T1]) CallEvent(arg1 T1) {
	event.event.CallEvent(arg1)
}
