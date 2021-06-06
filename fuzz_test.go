// +build gofuzzbeta

package eventsourcing_test

import (
	"testing"

	"github.com/hallgren/eventsourcing"
	"github.com/hallgren/eventsourcing/eventstore/memory"
)

func FuzzName(f *testing.F) {
	f.Add("Kalle")
	f.Fuzz(func(t *testing.T, name string) {
		if name == "" {
			t.Skip()
		}
		p, err := CreatePerson(name)
		if err != nil {
			t.Fatal(err)
		}
		if p.Name != name {
			t.Fatalf("wronge name expected: %s got: %s", name, p.Name)
		}
	})
}

func FuzzAge(f *testing.F) {
	f.Add(1)
	f.Fuzz(func(t *testing.T, age int) {
		if age < 0 {
			// can't age negative years
			t.Skip()
		}
		p, err := CreatePerson("kalle")
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= age; i++ {
			p.GrowOlder()
		}
		if p.Age != age {
			t.Fatalf("wrong age: %d got: %d", age, p.Age)
		}
		// Born + AgedOneYear events
		if len(p.Events()) != age+1 {
			t.Fatalf("expected %d events got %d", age+1, len(p.Events()))
		}
		// version
		if int(p.Version()) != age+1 {
			t.Fatalf("expected %d version got %d", age+1, int(p.Version()))
		}
		// GlobalVersion
		if p.GlobalVersion() != 0 {
			t.Fatalf("global version should be 0 was %d", p.GlobalVersion())
		}

		// save events
		repo := eventsourcing.NewRepository(memory.Create(), nil)
		err = repo.Save(p)
		if err != nil {
			t.Fatal(err)
		}
		// GlobalVersion afer save
		if p.GlobalVersion() != eventsourcing.Version(age+1) {
			t.Fatalf("global version should be %d was %d", age+1, p.GlobalVersion())
		}

		// get person
		twin := Person{}
		err = repo.Get(p.ID(), &twin)
		if err != nil {
			t.Fatal("could not get person")
		}
		// Check internal aggregate version
		if p.Version() != twin.Version() {
			t.Fatalf("Wrong version org %q copy %q", p.Version(), twin.Version())
		}
		if p.Age != twin.Age {
			t.Fatalf("age missmatch %d %d", p.Age, twin.Age)
		}
		if p.Name != twin.Name {
			t.Fatalf("name differs %s %s", p.Name, twin.Name)
		}
	})
}
