package eventlog

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// PrepareLog returns the live Event and Channel instances for a concrete Event
// type on the logical Channel built by newChannel. Both type arguments must be
// pointer-to-struct types. newChannel is a constructor only: it receives the
// notification and error channels eventlog sends on, and its candidate is
// discarded when the logical Channel already has a runtime.
//
// Repeated compatible preparation returns the same live pointers. eventlog
// persists a new Event occurrence whenever it observes a change to the Event's
// selected fields, and delivers every committed occurrence of the Channel, in
// order, on the Channel's notification stream.
func PrepareLog[TEvent Event, TChannel Channel](ctx context.Context, newChannel func(chan Event, chan error) TChannel) (TEvent, TChannel, error) {
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
	return prepareInRuntime[TEvent, TChannel](ctx, rt, newChannel)
}

func prepareInRuntime[TEvent Event, TChannel Channel](ctx context.Context, rt *logRuntime, newChannel func(chan Event, chan error) TChannel) (TEvent, TChannel, error) {
	var zeroEvent TEvent
	var zeroChannel TChannel
	eventType, err := requireStructPointer(reflect.TypeFor[TEvent](), "TEvent")
	if err != nil {
		return zeroEvent, zeroChannel, err
	}
	if _, err := requireStructPointer(reflect.TypeFor[TChannel](), "TChannel"); err != nil {
		return zeroEvent, zeroChannel, err
	}
	if newChannel == nil {
		return zeroEvent, zeroChannel, errors.New("eventlog: Channel factory is nil")
	}

	notifications := make(chan Event, rt.policy.notificationBuffer)
	errs := make(chan error, 1)
	candidate, err := constructChannel(newChannel, notifications, errs)
	if err != nil {
		return zeroEvent, zeroChannel, err
	}
	channelSchema, err := resolveTypeSchema(candidate)
	if err != nil {
		return zeroEvent, zeroChannel, fmt.Errorf("eventlog: Channel: %w", err)
	}
	channelInstance := reflect.ValueOf(candidate)
	channelSnapshot, channelRaw, err := captureSnapshot(channelSchema, channelInstance)
	if err != nil {
		return zeroEvent, zeroChannel, fmt.Errorf("eventlog: Channel: %w", err)
	}
	channelID, err := deriveIdentity(channelIdentityDomain, channelSchema, channelRaw)
	if err != nil {
		return zeroEvent, zeroChannel, fmt.Errorf("eventlog: Channel: %w", err)
	}

	eventSchema, err := resolveTypeSchema(reflect.New(eventType.Elem()).Interface())
	if err != nil {
		return zeroEvent, zeroChannel, fmt.Errorf("eventlog: Event: %w", err)
	}
	if eventSchema, err = rt.registry.register(eventSchema); err != nil {
		return zeroEvent, zeroChannel, err
	}
	if err := ctx.Err(); err != nil {
		return zeroEvent, zeroChannel, err
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()
	runtime := rt.channels[channelID]
	if runtime == nil {
		runtime = &channelRuntime{
			rt:            rt,
			id:            channelID,
			identity:      channelRaw,
			schema:        channelSchema,
			live:          candidate,
			instance:      channelInstance,
			notifications: notifications,
			errs:          errs,
			events:        make(map[typeKey]*eventTracker),
			latest:        channelSnapshot,
			observed:      channelSnapshot,
		}
		rt.channels[channelID] = runtime
		rt.changes.trackChannel(runtime)
		rt.startWorker(runtime.runReader)
	}
	liveChannel, ok := runtime.live.(TChannel)
	if !ok {
		return zeroEvent, zeroChannel, fmt.Errorf("eventlog: Channel %s is prepared as %T, not %s", channelID, runtime.live, reflect.TypeFor[TChannel]())
	}

	runtime.mu.Lock()
	tracker := runtime.events[eventSchema.key()]
	if tracker == nil {
		instance := reflect.New(eventType.Elem())
		initial, _, captureErr := captureSnapshot(eventSchema, instance)
		if captureErr != nil {
			runtime.mu.Unlock()
			return zeroEvent, zeroChannel, fmt.Errorf("eventlog: Event: %w", captureErr)
		}
		tracker = &eventTracker{channel: runtime, schema: eventSchema, instance: instance, last: initial}
		runtime.events[eventSchema.key()] = tracker
		runtime.mu.Unlock()
		rt.changes.trackEvent(tracker)
	} else {
		runtime.mu.Unlock()
	}
	liveEvent, ok := tracker.instance.Interface().(TEvent)
	if !ok {
		return zeroEvent, zeroChannel, fmt.Errorf("eventlog: Event %s is prepared as %s", eventSchema.key(), tracker.instance.Type())
	}
	return liveEvent, liveChannel, nil
}

func requireStructPointer(t reflect.Type, label string) (reflect.Type, error) {
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("eventlog: %s must be a pointer to a struct type, got %s", label, t)
	}
	return t, nil
}

func constructChannel[TChannel Channel](newChannel func(chan Event, chan error) TChannel, notifications chan Event, errs chan error) (candidate TChannel, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("eventlog: Channel factory panicked: %v", recovered)
		}
	}()
	candidate = newChannel(notifications, errs)
	value := reflect.ValueOf(candidate)
	if !value.IsValid() || value.IsNil() {
		return candidate, errors.New("eventlog: Channel factory returned nil")
	}
	return candidate, nil
}
