package eventsourcing_test

import (
	"fmt"
	"github.com/hallgren/eventsourcing"
	"testing"
)

// Person aggregate
type Person struct {
	eventsourcing.AggregateRoot
	name          string
	age           int
	dead          int
}

// Born event
type Born struct {
	name string
}

// AgedOneYear event
type AgedOneYear struct {
}

// Create the first person events
func (p *Person) Create(name string) error {
	if p.ID() != "" {
		return fmt.Errorf("the person is already initialized")
	}
	if name == "" {
		return fmt.Errorf("name can't be blank")
	}

	p.TrackChange(Born{name: name})
	return nil
}

// CreateWithID the first person event that sets the aggregate id from the outside
func (p *Person) CreateWithID(id, name string) error {
	if p.ID() != "" {
		return fmt.Errorf("the person is already initialized")
	}
	if name == "" {
		return fmt.Errorf("name can't be blank")
	}

	err := p.SetID(id)
	if err == eventsourcing.ErrAggregateAlreadyExists {
		return err
	} else if err != nil {
		panic(err)
	}
	p.TrackChange(Born{name: name})
	return nil
}

// GrowOlder command
func (p *Person) GrowOlder() error {
	if p.ID() == "" {
		return fmt.Errorf("person not born")
	}
	p.TrackChange(AgedOneYear{})
	return nil
}

// Transition the person state dependent on the events
func (p *Person) Transition(event eventsourcing.Event) {
	switch e := event.Data.(type) {

	case Born:
		p.age = 0
		p.name = e.name

	case AgedOneYear:
		p.age += 1
	}
}

func TestCreateNewPerson(t *testing.T) {
	person := Person{}
	eventsourcing.CreateAggregate(&person)
	err := person.Create("kalle")
	if err != nil {
		t.Fatal("Error when creating person", err.Error())
	}

	if person.name != "kalle" {
		t.Fatal("Wrong person name")
	}

	if person.age != 0 {
		t.Fatal("Wrong person age")
	}

	if len(person.Changes()) != 1 {
		t.Fatal("There should be one event on the person aggregateRoot")
	}

	if person.Version() != 1 {
		t.Fatal("Wrong version on the person aggregateRoot", person.Version())
	}
}

func TestCreateNewPersonWithIDFromOutside(t *testing.T) {
	id := "123"
	person := Person{}
	eventsourcing.CreateAggregate(&person)
	err := person.CreateWithID(id, "kalle")
	if err != nil {
		t.Fatal("Error when creating person", err.Error())
	}

	if person.ID() != id {
		t.Fatal("Wrong aggregate id on the person aggregateRoot", person.ID())
	}
}


func TestBlankName(t *testing.T) {
	person := Person{}
	eventsourcing.CreateAggregate(&person)
	err := person.Create("")
	if err == nil {
		t.Fatal("The constructor should return error on blank name")
	}

}

func TestSetIDOnExistingPerson(t *testing.T) {
	person := Person{}
	eventsourcing.CreateAggregate(&person)
	err := person.Create("Kalle")
	if err != nil {
		t.Fatal("The constructor returned error")
	}

	err = person.SetID("new_id")
	if err == nil {
		t.Fatal("Should not be possible to set id on already existing person")
	}

}

func TestPersonAgedOneYear(t *testing.T) {
	person := Person{}
	eventsourcing.CreateAggregate(&person)
	_ = person.Create("kalle")
	person.GrowOlder()

	if len(person.Changes()) != 2 {
		t.Fatal("There should be two event on the person aggregateRoot", person.Changes())
	}

	if person.Changes()[len(person.Changes())-1].Reason != "AgedOneYear" {
		t.Fatal("The last event reason should be AgedOneYear", person.Changes()[len(person.Changes())-1].Reason)
	}
}


func TestPersonGrewTenYears(t *testing.T) {
	person := Person{}
	eventsourcing.CreateAggregate(&person)
	person.Create("kalle")
	for i := 1; i <= 10; i++ {
		person.GrowOlder()
	}

	if person.age != 10 {
		t.Fatal("person has the wrong age")
	}
}

func TestCreatePersonTwice(t *testing.T) {
	person := Person{}
	eventsourcing.CreateAggregate(&person)
	person.Create("kalle")
	err := person.Create("anka")
	if err == nil {
		t.Fatalf("Could not be able to create twice on same person")
	}
}
