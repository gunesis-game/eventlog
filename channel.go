package eventlog

// Channel is implemented by consumer-defined Channel pointer types. The factory
// passed to PrepareLog receives the notification and error channels eventlog
// sends on; the Channel stores them and returns them from Notifications and
// Errors. Errors must be received: a rejected transaction pauses the Channel
// until its error is received.
//
// Events delivered on Notifications, and the Events in a ChannelError, are
// isolated snapshots. Their Transaction channel is nil; change the live Event
// returned by PrepareLog instead.
type Channel interface {
	Fields() Fields
	Notifications() <-chan Event
	Errors() <-chan *ChannelError
}
