package eventlog

// Event is implemented by consumer-defined Event pointer types. Fields declares
// the direct fields eventlog persists, including exactly one IdentityOf field.
// Transaction returns the channel the Event's factory received: send a
// function on it to change the Event, and optionally its Channel, atomically.
// Concrete Event values arrive on a Channel's Notifications stream through this
// common interface.
type Event interface {
	Fields() Fields
	Transaction() chan<- func() error
}
