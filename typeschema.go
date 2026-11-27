package eventlog

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"unicode/utf8"
	"unsafe"
)

// typeSchema is the resolved field selection of one concrete Event or Channel
// struct type: its reflected identity, the selected direct fields sorted
// bytewise by name, and the designated identity field.
type typeSchema struct {
	typ      reflect.Type
	pkgPath  string
	name     string
	fields   []fieldBinding
	names    []string
	identity int
}

// fieldBinding binds one selected direct struct field.
type fieldBinding struct {
	name  string
	index int
	role  fieldRole
	typ   reflect.Type
	kind  reflect.Kind
}

type fieldsDeclarer interface {
	Fields() Fields
}

func (s *typeSchema) key() typeKey {
	return typeKey{pkgPath: s.pkgPath, name: s.name}
}

func (s *typeSchema) identityField() fieldBinding {
	return s.fields[s.identity]
}

// sameShape reports whether two schemas select the same fields with the same
// roles and Go types.
func (s *typeSchema) sameShape(other *typeSchema) bool {
	if s.typ != other.typ || len(s.fields) != len(other.fields) {
		return false
	}
	for i := range s.fields {
		a, b := s.fields[i], other.fields[i]
		if a.name != b.name || a.index != b.index || a.role != b.role || a.typ != b.typ {
			return false
		}
	}
	return true
}

// fieldValueOf returns a settable reflect.Value for a bound field, including
// unexported fields, on a pointer-to-struct instance.
func (s *typeSchema) fieldValueOf(instance reflect.Value, binding fieldBinding) reflect.Value {
	field := instance.Elem().Field(binding.index)
	return reflect.NewAt(binding.typ, unsafe.Pointer(field.UnsafeAddr())).Elem()
}

// resolveTypeSchema validates a pointer-to-struct instance's Fields declaration
// and resolves every selector to exactly one direct struct field.
func resolveTypeSchema(instance any) (*typeSchema, error) {
	value := reflect.ValueOf(instance)
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() || value.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("%T must be a non-nil pointer to a struct", instance)
	}
	structType := value.Type().Elem()
	if structType.Name() == "" {
		return nil, fmt.Errorf("%s must be a named struct type", structType)
	}
	declarer, ok := instance.(fieldsDeclarer)
	if !ok {
		return nil, fmt.Errorf("%T does not implement Fields() Fields", instance)
	}
	declared, err := callFields(declarer)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", structType, err)
	}
	schema := &typeSchema{typ: structType, pkgPath: structType.PkgPath(), name: structType.Name(), identity: -1}
	if schema.pkgPath == "" {
		return nil, fmt.Errorf("%s has no package path", structType)
	}
	probe := reflect.New(structType)
	seen := make(map[string]bool, len(declared))
	for i, field := range declared {
		binding, err := bindField(probe, structType, field)
		if err != nil {
			return nil, fmt.Errorf("%s field %d: %w", structType, i, err)
		}
		if seen[binding.name] {
			return nil, fmt.Errorf("%s: field %q selected more than once", structType, binding.name)
		}
		seen[binding.name] = true
		schema.fields = append(schema.fields, binding)
	}
	sort.Slice(schema.fields, func(i, j int) bool {
		return schema.fields[i].name < schema.fields[j].name
	})
	for i, binding := range schema.fields {
		schema.names = append(schema.names, binding.name)
		if binding.role != fieldRoleIdentity {
			continue
		}
		if schema.identity >= 0 {
			return nil, fmt.Errorf("%s: more than one IdentityOf field", structType)
		}
		schema.identity = i
	}
	if schema.identity < 0 {
		return nil, fmt.Errorf("%s: Fields() must include exactly one IdentityOf field", structType)
	}
	if len(schema.fields) > identityMaxFieldCount {
		return nil, fmt.Errorf("%s: %d fields exceed the limit of %d", structType, len(schema.fields), identityMaxFieldCount)
	}
	return schema, nil
}

func callFields(declarer fieldsDeclarer) (fields Fields, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("Fields() panicked: %v", recovered)
		}
	}()
	return declarer.Fields(), nil
}

var errNilSelector = errors.New("field selector is nil")

func bindField(probe reflect.Value, structType reflect.Type, field Field) (fieldBinding, error) {
	if field.role == 0 {
		return fieldBinding{}, errors.New("zero Field; use FieldOf, FieldOfNullable, or IdentityOf")
	}
	if field.locate == nil {
		return fieldBinding{}, errNilSelector
	}
	if field.owner != structType {
		return fieldBinding{}, fmt.Errorf("selector is for %s, not %s", field.owner, structType)
	}
	located, err := field.locate(probe)
	if err != nil {
		return fieldBinding{}, err
	}
	if !located.IsValid() || located.IsNil() {
		return fieldBinding{}, errors.New("selector returned a nil pointer")
	}
	address := located.Pointer()
	match := -1
	for i := 0; i < structType.NumField(); i++ {
		candidate := structType.Field(i)
		if candidate.Type != field.target {
			continue
		}
		if probe.Elem().Field(i).UnsafeAddr() != address {
			continue
		}
		if match >= 0 {
			return fieldBinding{}, errors.New("selector matches more than one direct field")
		}
		match = i
	}
	if match < 0 {
		return fieldBinding{}, errors.New("selector must return the address of a direct field of the struct")
	}
	structField := structType.Field(match)
	if structField.Name == "" || !utf8.ValidString(structField.Name) {
		return fieldBinding{}, fmt.Errorf("invalid field name %q", structField.Name)
	}
	if len(structField.Name) > identityMaxComponent {
		return fieldBinding{}, fmt.Errorf("field name exceeds %d bytes", identityMaxComponent)
	}
	binding := fieldBinding{name: structField.Name, index: match, role: field.role, typ: structField.Type}
	switch field.role {
	case fieldRoleNullable:
		binding.kind = structField.Type.Elem().Kind()
	case fieldRoleIdentity:
		binding.kind = reflect.Array
	default:
		binding.kind = structField.Type.Kind()
	}
	return binding, nil
}
