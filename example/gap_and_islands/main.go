package main

import (
	"fmt"
	"github.com/hallgren/eventsourcing"
	"github.com/hallgren/eventsourcing/eventstore/memory"
	"github.com/hallgren/eventsourcing/serializer/json"
	"github.com/hallgren/eventsourcing/snapshotstore"
	"time"
)

// DeviceSequence is the Aggregate
type DeviceSequence struct {
	eventsourcing.AggregateRoot
	DeviceID string
	Islands []Island
}

// Island is a type used inside the aggregate to hold the current state of the islands
// The internal representation of the aggregate could change based on your needs
type Island struct {
	Start time.Time
	Stop time.Time
}

// SequenceInitiated Event
type SequenceInitiated struct {
	DeviceID string
}

// Observation Event
type Observation struct {
	Timestamp time.Time
	Duration time.Duration
}

// Transition func that builds the aggregate from its events
func (ds *DeviceSequence) Transition(event eventsourcing.Event) {
	switch e := event.Data.(type) {
	case *SequenceInitiated:
		ds.DeviceID = e.DeviceID
	case *Observation:
		ds.Islands = ds.CalcIslands(ds.Islands, *e)
	}
}

// CalcIslands re-calculate the islands
func (ds *DeviceSequence) CalcIslands(islands []Island, o Observation) []Island {
	// TODO: build up the state of the islands for the device based on the current islands and the new observation
	// currently we just add a new island to the array
	stop := o.Timestamp.Add(o.Duration)
	return append(islands, Island{Start:o.Timestamp, Stop: stop})
}

// New is the constructor that binds the sequence to the device
func New(deviceID string) *DeviceSequence {
	ds := DeviceSequence{}
	_ = ds.TrackChange(&ds, &SequenceInitiated{DeviceID:deviceID})
	return &ds
}

// The Observe command that triggers an Observation event and adds it to the DeviceSequence aggreagate
func (ds *DeviceSequence) Observe(t time.Time, d time.Duration) error {
	// Could hold some validation of the input before the event is created
	return ds.TrackChange(ds, &Observation{Timestamp: t, Duration: d})
}


func main() {

	// serializer
	serializer := json.New()
	serializer.Register(&DeviceSequence{}, &SequenceInitiated{}, &Observation{})

	// Setup a memory based event and snapshot store with json serializer
	repo := eventsourcing.NewRepository(memory.Create(serializer), snapshotstore.New(serializer))

	// Creates the aggregate and adds a second event
	aggregate := New("device1")
	d := time.Second * 10

	// generate some event on the device 1 aggregate
	aggregate.Observe(time.Now(), d)
	aggregate.Observe(time.Now().Add(d), d)
	aggregate.Observe(time.Now().Add(d*d), d)
	aggregate.Observe(time.Now().Add(-d), d)

	// saves the events to the memory backed event store
	err := repo.Save(aggregate)
	if err != nil {
		panic("Could not save the aggregate")
	}

	// Save Snapshot of the current state of the device sequence aggregate
	repo.SaveSnapshot(aggregate)
	
	// extra events are generated
	aggregate.Observe(time.Now().Add(d), d)
	aggregate.Observe(time.Now().Add(d*d), d)
	aggregate.Observe(time.Now().Add(-d), d)
	fmt.Println(len(aggregate.Islands))
	// Load the saved aggregate from the snapshot but without the generated events that are not saved yet
	copy := DeviceSequence{}
	err = repo.Get(string(aggregate.AggregateID), &copy)
	if err != nil {
		panic("Could not get aggregate")
	}
	// has only 4 of the islands
	fmt.Println(len(copy.Islands))

	// Saves the events
	repo.Save(aggregate)

	// Load the saved aggregate from the snapshot plus the extra events that was created after the snapshot was saved
	copy2 := DeviceSequence{}
	err = repo.Get(string(aggregate.AggregateID), &copy2)
	if err != nil {
		panic("Could not get aggregate")
	}

	// copy 2 has all the 7 islands in its array
	fmt.Println(len(copy2.Islands))

}

