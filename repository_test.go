package eventsourcing_test

import (
	"github.com/hallgren/eventsourcing"
	"github.com/hallgren/eventsourcing/eventstore/memory"
	"testing"
)

type Device struct {
	name string
}

type DeviceAggregate struct {
	eventsourcing.AggregateRoot
	Device
}

type NameSet struct {
	Name string
}

func (d *DeviceAggregate) SetName(name string) {
	d.TrackChange(NameSet{Name: name})
}

// Transition the person state dependent on the events
func (d *DeviceAggregate) Transition(event eventsourcing.Event) {
	switch e := event.Data.(type) {
	case NameSet:
		d.name = e.Name
	}
}

func TestCreatePerson(t *testing.T) {
	repo := eventsourcing.NewRepository(memory.Create())
	person := Person{}
	repo.New(&person)

	err := person.Create("kalle")
	if err != nil {
		t.Fatalf("could not create person %v", err)
	}
}

func TestGrowPersonBeforeBorn(t *testing.T) {
	repo := eventsourcing.NewRepository(memory.Create())
	person := Person{}
	repo.New(&person)

	err := person.GrowOlder()
	if err == nil {
		t.Fatalf("person has to be born before ageing")
	}
}

func TestSaveAndGetAggregate(t *testing.T) {
	repo := eventsourcing.NewRepository(memory.Create())

	person := Person{}
	repo.New(&person)
	person.Create("kalle")
	err := repo.Save(&person)
	if err != nil {
		t.Fatal("could not save person")
	}
	twin := Person{}
	err = repo.Get(person.ID(), &twin)
	if err != nil {
		t.Fatalf("could not get person %v", err)
	}

	if person.Version() != twin.Version() {
		t.Fatalf("Wrong version org %q copy %q", person.Version(), twin.Version())
	}
}

func TestSaveBeforeNew(t *testing.T) {
	repo := eventsourcing.NewRepository(memory.Create())

	person := Person{}
	person.GrowOlder()
	err := repo.Save(&person)
	if err != nil {
		t.Fatal("could not save person")
	}

}