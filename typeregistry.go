package eventlog

import (
	"errors"
	"fmt"
	"sync"
)

// ErrSchemaConflict reports that a concrete Event type was prepared again with
// a different exposed field schema. The existing registration is unchanged.
var ErrSchemaConflict = errors.New("eventlog: event type schema conflict")

// typeKey identifies a concrete Go type by reflected package path and name.
type typeKey struct {
	pkgPath string
	name    string
}

func (k typeKey) String() string {
	return k.pkgPath + "." + k.name
}

// typeRegistry holds Event decoder registrations for the runtime lifetime,
// keyed by concrete type. Readers waiting for a missing decoder wait on the
// changed channel, which is closed and replaced on every new registration.
type typeRegistry struct {
	mu      sync.Mutex
	byKey   map[typeKey]*typeSchema
	changed chan struct{}
}

func newTypeRegistry() *typeRegistry {
	return &typeRegistry{byKey: make(map[typeKey]*typeSchema), changed: make(chan struct{})}
}

// register reuses an identical registration, rejects a conflicting schema for
// the same type, or adds a new registration.
func (r *typeRegistry) register(schema *typeSchema) (*typeSchema, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.byKey[schema.key()]; ok {
		if !existing.sameShape(schema) {
			return nil, fmt.Errorf("%w: %s", ErrSchemaConflict, schema.key())
		}
		return existing, nil
	}
	r.byKey[schema.key()] = schema
	close(r.changed)
	r.changed = make(chan struct{})
	return schema, nil
}

// lookup returns the registration for key, or nil plus a channel that is closed
// when any new registration occurs.
func (r *typeRegistry) lookup(key typeKey) (*typeSchema, <-chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if schema, ok := r.byKey[key]; ok {
		return schema, nil
	}
	return nil, r.changed
}
