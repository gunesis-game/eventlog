package eventlog

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// PrepareLog returns the live Event and Channel for the logical identities
// that newEvent and newChannel construct. Both type arguments must be
// pointer-to-struct types. Both factories are constructors only: they receive
// the channels eventlog uses, must set the identity field, and their result is
// discarded when that logical Event or Channel is already live.
//
// newEvent receives the Channel's shared transaction channel, which the Event
// returns from Transaction. Each function sent on it is applied atomically, in
// send order with every other transaction of the Channel, and each changed
// Event becomes a committed occurrence delivered in order on the Channel's
// notification stream. Repeated preparation of the same logical identities
// returns the same live pointers.
func PrepareLog[TEvent Event, TChannel Channel](ctx context.Context, newEvent func(chan<- func() error) TEvent, newChannel func(chan Event, chan *ChannelError) TChannel) (TEvent, TChannel, error) {
	var event TEvent
	var channel TChannel
	if ctx == nil {
		return event, channel, errors.New("eventlog: nil Context")
	}
	if err := ctx.Err(); err != nil {
		return event, channel, err
	}
	settings, err := currentConfig()
	if err != nil {
		return event, channel, err
	}
	rt, err := sharedLogRuntime(ctx, settings)
	if err != nil {
		return event, channel, err
	}
	return prepareInRuntime[TEvent, TChannel](ctx, rt, newEvent, newChannel)
}

func prepareInRuntime[TEvent Event, TChannel Channel](ctx context.Context, rt *logRuntime, newEvent func(chan<- func() error) TEvent, newChannel func(chan Event, chan *ChannelError) TChannel) (TEvent, TChannel, error) {
	var zeroEvent TEvent
	var zeroChannel TChannel
	eventType, err := requireStructPointer(reflect.TypeFor[TEvent](), "TEvent")
	if err != nil {
		return zeroEvent, zeroChannel, err
	}
	if _, err := requireStructPointer(reflect.TypeFor[TChannel](), "TChannel"); err != nil {
		return zeroEvent, zeroChannel, err
	}
	if newEvent == nil || newChannel == nil {
		return zeroEvent, zeroChannel, errors.New("eventlog: Event and Channel factories must not be nil")
	}

	eventSchema, err := resolveTypeSchema(reflect.New(eventType.Elem()).Interface())
	if err != nil {
		return zeroEvent, zeroChannel, fmt.Errorf("eventlog: Event: %w", err)
	}
	if eventSchema, err = rt.registry.register(eventSchema); err != nil {
		return zeroEvent, zeroChannel, err
	}
	runtime, err := resolveChannel(rt, newChannel)
	if err != nil {
		return zeroEvent, zeroChannel, err
	}
	liveChannel, ok := runtime.live.(TChannel)
	if !ok {
		return zeroEvent, zeroChannel, fmt.Errorf("eventlog: Channel %s is prepared as %T, not %s", runtime.id, runtime.live, reflect.TypeFor[TChannel]())
	}
	candidate, err := constructEvent(newEvent, runtime, eventSchema)
	if err != nil {
		return zeroEvent, zeroChannel, err
	}
	if err := ctx.Err(); err != nil {
		return zeroEvent, zeroChannel, err
	}
	liveEvent, ok := runtime.adoptEvent(candidate).event.(TEvent)
	if !ok {
		return zeroEvent, zeroChannel, fmt.Errorf("eventlog: Event %s is prepared as another type", candidate.id)
	}
	return liveEvent, liveChannel, nil
}

func requireStructPointer(t reflect.Type, label string) (reflect.Type, error) {
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("eventlog: %s must be a pointer to a struct type, got %s", label, t)
	}
	return t, nil
}

// resolveChannel constructs a Channel candidate and returns the runtime for its
// logical identity, adopting the candidate and starting its Reader and
// transaction processor when the identity is new.
func resolveChannel[TChannel Channel](rt *logRuntime, newChannel func(chan Event, chan *ChannelError) TChannel) (*channelRuntime, error) {
	notifications := make(chan Event, rt.policy.notificationBuffer)
	errs := make(chan *ChannelError)
	candidate, err := callFactory(func() TChannel {
		return newChannel(notifications, errs)
	})
	if err != nil {
		return nil, fmt.Errorf("eventlog: Channel factory: %w", err)
	}
	schema, err := resolveTypeSchema(candidate)
	if err != nil {
		return nil, fmt.Errorf("eventlog: Channel: %w", err)
	}
	_, identity, err := captureSnapshot(schema, reflect.ValueOf(candidate))
	if err != nil {
		return nil, fmt.Errorf("eventlog: Channel: %w", err)
	}
	id, err := deriveIdentity(channelIdentityDomain, schema, identity)
	if err != nil {
		return nil, fmt.Errorf("eventlog: Channel: %w", err)
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if runtime := rt.channels[id]; runtime != nil {
		return runtime, nil
	}
	runtime := newChannelRuntime(rt, id, identity, schema, candidate, notifications, errs)
	rt.channels[id] = runtime
	rt.startWorker(runtime.runReader)
	runtime.processor = newTransactionProcessor(runtime)
	rt.startWorker(runtime.processor.runIntake)
	rt.startWorker(runtime.processor.run)
	return runtime, nil
}

// constructEvent builds an Event candidate wired to the Channel's transaction
// channel and validates its schema, fields, and identity.
func constructEvent[TEvent Event](newEvent func(chan<- func() error) TEvent, runtime *channelRuntime, schema *typeSchema) (*liveEvent, error) {
	candidate, err := callFactory(func() TEvent {
		return newEvent(runtime.transactions)
	})
	if err != nil {
		return nil, fmt.Errorf("eventlog: Event factory: %w", err)
	}
	if candidate.Transaction() != (chan<- func() error)(runtime.transactions) {
		return nil, errors.New("eventlog: Event.Transaction() must return the channel its factory received")
	}
	own, err := resolveTypeSchema(candidate)
	if err != nil {
		return nil, fmt.Errorf("eventlog: Event: %w", err)
	}
	if !own.sameShape(schema) {
		return nil, fmt.Errorf("%w: %s", ErrSchemaConflict, schema.key())
	}
	instance := reflect.ValueOf(candidate)
	_, identity, err := captureSnapshot(schema, instance)
	if err != nil {
		return nil, fmt.Errorf("eventlog: Event: %w", err)
	}
	id, err := deriveIdentity(eventIdentityDomain, schema, identity)
	if err != nil {
		return nil, fmt.Errorf("eventlog: Event: %w", err)
	}
	return &liveEvent{id: id, identity: identity, schema: schema, instance: instance, event: candidate}, nil
}

// callFactory runs a consumer factory, rejecting a panic or a nil result.
func callFactory[T any](construct func() T) (result T, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panicked: %v", recovered)
		}
	}()
	result = construct()
	value := reflect.ValueOf(result)
	if !value.IsValid() || value.IsNil() {
		return result, errors.New("returned nil")
	}
	return result, nil
}
