package bbolt

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/imkira/go-observer"
	"gitlab.se.axis.com/morganh/eventsourcing"
	"gitlab.se.axis.com/morganh/eventsourcing/eventstore"
	"time"
	"unsafe"

	"github.com/etcd-io/bbolt"
)

const (
	globalEventOrderBucketName = "global_event_order"
)

// ErrorNotFound is returned when a given entity cannot be found in the event stream
var ErrorNotFound = errors.New("NotFoundError")

// itob returns an 8-byte big endian representation of v.
func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

// BBolt is a handler for event streaming
type BBolt struct {
	db 				*bbolt.DB 			// The bbolt db where we store everything
	eventsProperty  observer.Property   // A property to which all event changes for all event types are published
}

// MustOpenBBolt opens the event stream found in the given file. If the file is not found it will be created and
// initialized. Will panic if it has problems persisting the changes to the filesystem.
func MustOpenBBolt(dbFile string) *BBolt {
	db, err := bbolt.Open(dbFile, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		panic(err)
	}

	// Ensure that we have a bucket to store the global event ordering
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(globalEventOrderBucketName)); err != nil {
			return fmt.Errorf("could not create global event order bucket")
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return &BBolt{
		db: db,
		eventsProperty: observer.NewProperty(nil),
	}
}

// Save an aggregate (its events)
func (e *BBolt) Save(events []eventsourcing.Event) error {
	// Return if there is no events to save
	if len(events) == 0 {
		return nil
	}

	// get bucket name from first event
	aggregateType := events[0].AggregateType
	aggregateID := events[0].AggregateRootID
	bucketName := aggregateKey(aggregateType, string(aggregateID))

	tx, err := e.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	evBucket := tx.Bucket([]byte(bucketName))
	if evBucket == nil {
		// Ensure that we have a bucket named events_aggregateType_aggregateID for the given aggregate
		err = e.createBucket([]byte(bucketName), tx)
		if err != nil {
			return fmt.Errorf("could not create aggregate events bucket")
		}
		evBucket = tx.Bucket([]byte(bucketName))
	}

	currentVersion := eventsourcing.Version(0)
	cursor := evBucket.Cursor()
	k, obj := cursor.Last()
	if k != nil {
		event := (*eventsourcing.Event)(unsafe.Pointer(&obj[0]))
		currentVersion = event.Version
	}

	//Validate events
	err = eventstore.ValidateEvents(aggregateID, currentVersion, events)
	if err != nil {
		return err
	}

	globalBucket := tx.Bucket([]byte(globalEventOrderBucketName))
	if globalBucket == nil {
		return fmt.Errorf("global bucket not found")
	}

	for _, event := range events {

		sequence, err := evBucket.NextSequence()
		if err != nil {
			return fmt.Errorf("could not get sequence for %#v", bucketName)
		}

		value := make([]byte, unsafe.Sizeof(eventsourcing.Event{}))
		t := (*eventsourcing.Event)(unsafe.Pointer(&value[0]))

		// Sets the properties on the event
		t.AggregateRootID = event.AggregateRootID
		t.AggregateType = event.AggregateType
		t.Data = event.Data
		t.MetaData = event.MetaData
		t.Reason = event.Reason
		t.Version = event.Version

		err = evBucket.Put(itob(int(sequence)), value)
		if err != nil {
			return fmt.Errorf("could not save event %#v in bucket", event)
		}
		// We need to establish a global event order that spans over all buckets. This is so that we can be
		// able to play the event (or send) them in the order that they was entered into this database.
		// The global sequence bucket contains an ordered line of pointer to all events on the form bucket_name:seq_num
		globalSequence, err := globalBucket.NextSequence()
		if err != nil {
			return fmt.Errorf("could not get next sequence for global bucket")
		}
		//globalSequenceValue := bucketName + ":" + strconv.FormatUint(sequence, 10)
		err = globalBucket.Put(itob(int(globalSequence)), value)
		if err != nil {
			return fmt.Errorf("could not save global sequence pointer for %#v", bucketName)
		}

		e.eventsProperty.Update(event)

	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

// Get aggregate events
func (e *BBolt) Get(id string, aggregateType string) ([]eventsourcing.Event, error) {
	bucketName := aggregateKey(aggregateType, id)

	tx, err := e.db.Begin(false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	evBucket := tx.Bucket([]byte(bucketName))

	cursor := evBucket.Cursor()
	events := make([]eventsourcing.Event, 0)
	event := &eventsourcing.Event{}

	for k, obj := cursor.First(); k != nil; k, obj = cursor.Next() {
		event = (*eventsourcing.Event)(unsafe.Pointer(&obj[0]))
		events = append(events, *event)
	}
	return events, nil
}

// GlobalGet returns events from the global order
func (e *BBolt) GlobalGet(start int, count int) []eventsourcing.Event {
	tx, err := e.db.Begin(false)
	if err != nil {
		return nil
	}
	defer tx.Rollback()

	evBucket := tx.Bucket([]byte(globalEventOrderBucketName))
	cursor := evBucket.Cursor()
	events := make([]eventsourcing.Event, 0)
	event := &eventsourcing.Event{}
	counter := 0

	for k, obj := cursor.Seek([]byte(itob(int(start)))); k != nil; k, obj = cursor.Next() {
		event = (*eventsourcing.Event)(unsafe.Pointer(&obj[0]))
		events = append(events, *event)
		counter++

		if counter >= count {
			break
		}
	}

	return events
}

// EventStream returns a stream with all saved events
func (e *BBolt) EventStream() observer.Stream {
	return e.eventsProperty.Observe()
}

// Close closes the event stream and the underlying database
func (e *BBolt) Close() error {
	return e.db.Close()
}

// CreateBucket creates a bucket
func (e *BBolt) createBucket(bucketName []byte, tx *bbolt.Tx) error {
	// Ensure that we have a bucket named event_type for the given type
	if _, err := tx.CreateBucketIfNotExists([]byte(bucketName)); err != nil {
		return fmt.Errorf("could not create bucket for %s: %s", bucketName, err)
	}
	return nil

}

// aggregateKey generate a aggregate key to store events against from aggregateType and aggregateID
func aggregateKey(aggregateType, aggregateID string) string {
	return aggregateType + "_" + aggregateID
}
