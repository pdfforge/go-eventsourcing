package eventsourcing_test

import (
	"encoding/json"
	"testing"

	"github.com/hallgren/eventsourcing"
	"github.com/hallgren/eventsourcing/core"
	"github.com/hallgren/eventsourcing/eventstore/memory"
	"github.com/hallgren/eventsourcing/internal"
)

func createBornEvent(es *memory.Memory, name string) error {
	person, err := CreatePerson(name)
	if err != nil {
		return err
	}

	events := make([]core.Event, 0)
	for _, e := range person.Events() {
		data, err := json.Marshal(e.Data())
		if err != nil {
			return err
		}

		events = append(events, core.Event{
			AggregateID:   e.AggregateID(),
			Reason:        e.Reason(),
			AggregateType: e.AggregateType(),
			Version:       core.Version(e.Version()),
			GlobalVersion: core.Version(e.GlobalVersion()),
			Timestamp:     e.Timestamp(),
			Data:          data,
		})
	}
	return es.Save(events)
}

func TestRunOnce(t *testing.T) {
	// setup
	es := memory.Create()
	register := internal.NewRegister()
	register.Register(&Person{})

	projectedName := ""
	sourceName := "kalle"

	err := createBornEvent(es, sourceName)
	if err != nil {
		t.Fatal(err)
	}

	// run projection
	p := eventsourcing.NewProjections(register, json.Unmarshal)
	proj := p.Add(es.All(0, 1), func(event eventsourcing.Event) {
		switch e := event.Data().(type) {
		case *Born:
			projectedName = e.Name
		}
	})

	err = proj.RunOnce()
	if err != nil {
		t.Fatal(err)
	}

	if projectedName != sourceName {
		t.Fatalf("expected %q was %q", sourceName, projectedName)
	}
}
