// Package fieldschema resolves consumer field selections into schemas and
// captures, encodes, decodes, restores, and copies the selected field values.
package fieldschema

import (
	"fmt"
	"reflect"
)

// Role says how a selected field is persisted.
type Role uint8

const (
	RoleValue Role = iota + 1
	RoleNullable
	RoleIdentity
)

// Selection is one selected direct struct field, before it is bound to a
// struct layout. The zero Selection is invalid.
type Selection struct {
	role   Role
	owner  reflect.Type
	target reflect.Type
	locate func(instance reflect.Value) (reflect.Value, error)
}

// Select builds a Selection from a typed selector returning a pointer to a
// field of T. P is the selector's result type (*V, **V, or *U).
func Select[T any, P any](role Role, selector func(*T) P) Selection {
	owner := reflect.TypeFor[T]()
	target := reflect.TypeFor[P]().Elem()
	selection := Selection{role: role, owner: owner, target: target}
	if selector == nil {
		return selection
	}
	selection.locate = func(instance reflect.Value) (located reflect.Value, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("field selector panicked: %v", recovered)
			}
		}()
		typed, ok := instance.Interface().(*T)
		if !ok {
			return reflect.Value{}, fmt.Errorf("field selector for %s used with %s", owner, instance.Type())
		}
		return reflect.ValueOf(selector(typed)), nil
	}
	return selection
}
