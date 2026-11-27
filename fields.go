package eventlog

import "github.com/gunesis-game/eventlog/internal/fieldschema"

// Fields is the set of direct struct fields an Event or Channel exposes for
// persistence. Build it from FieldOf, FieldOfNullable, and exactly one IdentityOf.
type Fields []Field

// Field is an opaque selection of one direct struct field. Create it with
// FieldOf, FieldOfNullable, or IdentityOf.
type Field struct {
	selection fieldschema.Selection
}

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
	return Field{selection: fieldschema.Select[T, *V](fieldschema.RoleValue, selector)}
}

// FieldOfNullable selects a nullable primitive field (a pointer field), for
// example FieldOfNullable(func(o *Order) **string { return &o.Note }).
func FieldOfNullable[T any, V primitive](selector func(*T) **V) Field {
	return Field{selection: fieldschema.Select[T, **V](fieldschema.RoleNullable, selector)}
}

// IdentityOf selects the UUID identity field, whose type must have underlying
// type [16]byte, for example IdentityOf(func(o *Order) *OrderID { return &o.ID }).
func IdentityOf[T any, U ~[16]byte](selector func(*T) *U) Field {
	return Field{selection: fieldschema.Select[T, *U](fieldschema.RoleIdentity, selector)}
}
