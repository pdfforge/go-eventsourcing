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
	Duration time.Duration
}

// SequenceCreated Event
type SequenceCreated struct {
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
	case *Observation:
		ds.Islands = ds.CalcIslands(ds.Islands, *e)
	}
}

// CalcIslands re-calculate the islands
func (ds *DeviceSequence) CalcIslands(islands []Island, o Observation) []Island {
	// TODO: build up the state of the islands for the device based on the current islands and the new observation
	return append(islands, Island{Duration:o.Duration, Start:o.Timestamp})
}

// New is the constructor that binds the sequence to the device
func New(deviceID string) *DeviceSequence {
	ds := DeviceSequence{}
	_ = ds.TrackChange(&ds, &SequenceCreated{DeviceID:deviceID})
	return &ds
}

// The Observe command that triggers an Observation event and adds it to the DeviceSequence aggreagate
func (ds *DeviceSequence) Observe(t time.Time, d time.Duration) error {
	return ds.TrackChange(ds, &Observation{Timestamp: t, Duration: d})
}


func main() {

	// serializer
	serializer := json.New()
	serializer.Register(&DeviceSequence{}, &SequenceCreated{}, &Observation{})

	// Setup a memory based event and snapshot store with json serializer
	repo := eventsourcing.NewRepository(memory.Create(serializer), snapshotstore.New(serializer))
	stream := repo.EventStream()

	// Read the event stream async
	go func() {
		for {
			<-stream.Changes()
			// advance to next value
			stream.Next()
			event := stream.Value().(eventsourcing.Event)
			fmt.Println("STREAM EVENT")
			fmt.Println(event)
		}
	}()

	// Creates the aggregate and adds a second event
	aggregate := New("device1")
	d := time.Second * 10

	// generate some event on the device 1 aggregate
	aggregate.Observe(time.Now(), d)
	aggregate.Observe(time.Now().Add(d), d)
	aggregate.Observe(time.Now().Add(d*d), d)
	aggregate.Observe(time.Now().Add(-d), d)

	// saves the events to the memory backed eventstore
	err := repo.Save(aggregate)
	if err != nil {
		panic("Could not save the aggregate")
	}

	// Save Snapshot
	repo.SaveSnapshot(aggregate)

	// Load the saved aggregate from the snapshot
	copy := DeviceSequence{}
	err = repo.Get(string(aggregate.AggregateID), &copy)
	if err != nil {
		panic("Could not get aggregate")
	}

	// Sleep to make sure the events are delivered from the stream
	time.Sleep(time.Millisecond * 100)
	fmt.Println("AGGREGATE")
	fmt.Println(copy)

}

