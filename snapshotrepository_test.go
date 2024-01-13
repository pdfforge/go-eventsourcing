package eventsourcing_test

import (
	"context"
	"testing"

	"github.com/hallgren/eventsourcing"
	"github.com/hallgren/eventsourcing/eventstore/memory"
	snap "github.com/hallgren/eventsourcing/snapshotstore/memory"
)

func TestSaveAndGetSnapshot(t *testing.T) {
	eventrepo := eventsourcing.NewEventRepository(memory.Create())
	eventrepo.Register(&Person{})

	snapshotrepo := eventsourcing.NewSnapshotRepository(snap.Create(), eventrepo)

	person, err := CreatePerson("kalle")
	if err != nil {
		t.Fatal(err)
	}
	err = snapshotrepo.Save(person)
	if err != nil {
		t.Fatalf("could not save aggregate, err: %v", err)
	}

	twin := Person{}
	err = snapshotrepo.GetWithContext(context.Background(), person.ID(), &twin)
	if err != nil {
		t.Fatal("could not get aggregate")
	}

	// Check internal aggregate version
	if person.Version() != twin.Version() {
		t.Fatalf("Wrong version org %q copy %q", person.Version(), twin.Version())
	}

	if person.ID() != twin.ID() {
		t.Fatalf("Wrong id org %q copy %q", person.ID(), twin.ID())
	}
}

func TestSaveSnapshotWithUnsavedEvents(t *testing.T) {
	eventrepo := eventsourcing.NewEventRepository(memory.Create())
	eventrepo.Register(&Person{})

	snapshotrepo := eventsourcing.NewSnapshotRepository(snap.Create(), eventrepo)

	person, err := CreatePerson("kalle")
	if err != nil {
		t.Fatal(err)
	}
	err = snapshotrepo.SaveSnapshot(person)
	if err == nil {
		t.Fatalf("should not be able to save snapshot with unsaved events")
	}
}