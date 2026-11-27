package eventlog

import (
	"fmt"
	"reflect"
	"strings"
)

// ErrorKind classifies a ChannelError.
type ErrorKind int

const (
	// TransactionRefused means the transaction function returned an error.
	TransactionRefused ErrorKind = iota + 1
	// TransactionPanicked means the transaction function panicked.
	TransactionPanicked
	// InvalidState means the transaction left a changed identity, an invalid
	// field value, or a Channel change without an Event change.
	InvalidState
	// CommitFailed means saving the batch failed; every transaction in it was
	// rolled back.
	CommitFailed
	// ReaderFailed means committed history could not be read or decoded. The
	// Channel's streams close after this error.
	ReaderFailed
	// DeliveryFailed means notification delivery retries were exhausted. The
	// Channel's streams close after this error.
	DeliveryFailed
)

func (k ErrorKind) String() string {
	switch k {
	case TransactionRefused:
		return "transaction refused"
	case TransactionPanicked:
		return "transaction panicked"
	case InvalidState:
		return "invalid state"
	case CommitFailed:
		return "commit failed"
	case ReaderFailed:
		return "reader failed"
	case DeliveryFailed:
		return "delivery failed"
	default:
		return fmt.Sprintf("ErrorKind(%d)", int(k))
	}
}

// ChannelError is delivered on a Channel's Errors stream. For a rejected
// transaction, its changes were rolled back and every transaction sent before
// the error was received is rejected too.
type ChannelError struct {
	Kind ErrorKind
	// Cause is the transaction function's error, the panic value, the field
	// problem, or the I/O error.
	Cause error
	// Events holds isolated copies of the attempted values of each Event the
	// failed transaction changed, or of every Event in the batch for
	// CommitFailed.
	Events []Event

	channelID uuid
}

func (e *ChannelError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "eventlog: channel %s: %s", e.channelID, e.Kind)
	if len(e.Events) > 0 {
		names := make([]string, 0, len(e.Events))
		for _, event := range e.Events {
			names = append(names, describeEvent(event))
		}
		fmt.Fprintf(&b, ": %s", strings.Join(names, ", "))
	}
	if e.Cause != nil {
		fmt.Fprintf(&b, ": %v", e.Cause)
	}
	if e.Kind <= CommitFailed {
		b.WriteString("; transactions queued behind it are rejected")
	}
	return b.String()
}

// Unwrap returns Cause.
func (e *ChannelError) Unwrap() error {
	return e.Cause
}

// describeEvent names an Event by its type and logical identity.
func describeEvent(event Event) string {
	value := reflect.ValueOf(event)
	name := value.Type().Elem().Name()
	schema, err := resolveTypeSchema(event)
	if err != nil {
		return name
	}
	_, raw, err := captureSnapshot(schema, value)
	if err != nil || raw.isZero() {
		return name
	}
	id, err := deriveIdentity(eventIdentityDomain, schema, raw)
	if err != nil {
		return name
	}
	return name + " " + id.String()
}
