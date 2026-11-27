package eventlog

// Channel is implemented by consumer-defined Channel pointer types. The factory
// passed to PrepareLog receives the notification and error channels that
// eventlog sends on; the Channel stores them and returns them from
// Notifications and Errors.
type Channel interface {
	Fields() Fields
	Notifications() <-chan Event
	Errors() <-chan error
}
