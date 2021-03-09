package es

import (
	"github.com/EventStore/EventStore-Client-Go/client"
	"github.com/hallgren/eventsourcing"
)

type ES struct {
	client *client.Client
}

func Open(client *client.Client) *ES {
	return &ES{
		client: client,
	}
}

// Close the connection
func (s *ES) Close() {
	s.client.Close()
}

func (s *ES) Save(events []eventsourcing.Event) error {
	return nil
}

func (s *ES) Get(id string, aggregateType string, afterVersion eventsourcing.Version) (events []eventsourcing.Event, err error) {
	return nil, nil
}
