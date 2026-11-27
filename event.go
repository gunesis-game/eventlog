package eventlog

// Event is implemented by consumer-defined Event pointer types. Fields declares
// the direct fields eventlog persists, including exactly one IdentityOf field.
// Concrete Event values arrive on a Channel's Notifications stream through this
// common interface.
type Event interface {
	Fields() Fields
}
