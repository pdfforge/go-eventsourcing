package unsafe

import (
	"github.com/hallgren/eventsourcing/eventstore"
	"unsafe"
)

type Handler struct{}

// New returns a json Handle
func New() *Handler {
	return &Handler{}
}

func (h *Handler) SerializeEvent(event eventstore.Event) ([]byte, error) {
	value := make([]byte, unsafe.Sizeof(eventstore.Event{}))
	t := (*eventstore.Event)(unsafe.Pointer(&value[0]))

	// Sets the properties on the event
	t.AggregateRootID = event.AggregateRootID
	t.AggregateType = event.AggregateType
	t.Data = event.Data
	t.MetaData = event.MetaData
	t.Reason = event.Reason
	t.Version = event.Version

	return value, nil
}

func (h *Handler) DeserializeEvent(obj []byte) (eventstore.Event, error) {
	var event = &eventstore.Event{}
	event = (*eventstore.Event)(unsafe.Pointer(&obj[0]))
	return *event, nil
}
