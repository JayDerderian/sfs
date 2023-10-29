package monitor

import (
	"log"
	"time"
)

type EventType string

const (
	FileCreate EventType = "create"
	FileDelete EventType = "delete"
	FileChange EventType = "change"

	DirCreate EventType = "create"
	DirDelete EventType = "delete"
	DirChange EventType = "change"
)

type Event struct {
	// UUID of the event
	ID string
	// type of file event, i.e. create, edit, or delete
	Type EventType
	// location of the file event (path to the file itself)
	Path string
	// time of the event
	Time time.Time
}

// Elist is a buffer for file events in order to maximize
// synchronization operations between client and server
type EList []Event

// arbitrary threhold limit for Elists
const THRESHOLD = 10

type Events struct {
	// buffer limit
	threshold int
	// current total events
	Total int
	// flag to indicate whether a sync operation should start
	StartSync bool
	// event object list
	Events EList
}

// new Events tracker. if buffered sync
// events will be delayed after THRESHOLD amount of events
// have been added to the EList buffer
func NewEvents(buffered bool) *Events {
	var threshold int
	if buffered {
		threshold = THRESHOLD
	} else {
		threshold = 1
	}
	return &Events{
		threshold: threshold,
		Events:    make(EList, 0),
	}
}

func (e *Events) Reset() {
	e.Events = make(EList, 0)
	e.StartSync = false
	e.Total = 0
}

func (e *Events) HasEvent(evt Event) bool {
	for _, e := range e.Events {
		if evt.ID == e.ID {
			return true
		}
	}
	return false
}

// add events until threshold is met. any subsequent
// events won't be added and will be ignored.
// sets StartSync to true when threshold is met.
func (e *Events) AddEvent(evt Event) {
	if !e.HasEvent(evt) && e.Total+1 <= e.threshold {
		e.Events = append(e.Events, evt)
		e.Total += 1
		if e.Total == e.threshold {
			e.StartSync = true
		}
	} else {
		log.Printf("[WARNING] event list threshold met")
	}
}
