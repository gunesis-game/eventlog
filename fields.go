package eventlog

import (
	"fmt"
	"reflect"
)

// Fields is the set of direct struct fields an Event or Channel exposes for
// persistence. Build it from FieldOf, FieldOfNullable, and exactly one IdentityOf.
type Fields []Field

// Field is an opaque selection of one direct struct field. Create it with
// FieldOf, FieldOfNullable, or IdentityOf.
type Field struct {
	role   fieldRole
	owner  reflect.Type
	target reflect.Type
	locate func(instance reflect.Value) (reflect.Value, error)
}

type fieldRole uint8

const (
	fieldRoleValue fieldRole = iota + 1
	fieldRoleNullable
	fieldRoleIdentity
)

// primitive lists the supported persisted field value types, including named
// types over them. uintptr and complex values are intentionally excluded.
type primitive interface {
	~string | ~bool |
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// FieldOf selects a non-nullable primitive field, for example
// FieldOf(func(o *Order) *int64 { return &o.Amount }).
func FieldOf[T any, V primitive](selector func(*T) *V) Field {
	return newField[T, *V](fieldRoleValue, selector)
}

// FieldOfNullable selects a nullable primitive field (a pointer field), for
// example FieldOfNullable(func(o *Order) **string { return &o.Note }).
func FieldOfNullable[T any, V primitive](selector func(*T) **V) Field {
	return newField[T, **V](fieldRoleNullable, selector)
}

// IdentityOf selects the UUID identity field, whose type must have underlying
// type [16]byte, for example IdentityOf(func(o *Order) *OrderID { return &o.ID }).
func IdentityOf[T any, U ~[16]byte](selector func(*T) *U) Field {
	return newField[T, *U](fieldRoleIdentity, selector)
}

func newField[T any, P any](role fieldRole, selector func(*T) P) Field {
	owner := reflect.TypeFor[T]()
	target := reflect.TypeFor[P]().Elem()
	field := Field{role: role, owner: owner, target: target}
	if selector == nil {
		return field
	}
	field.locate = func(instance reflect.Value) (located reflect.Value, err error) {
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
	return field
}
