package fieldschema

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"unicode/utf8"
	"unsafe"

	"github.com/gunesis-game/eventlog/internal/identity"
)

// Schema is the resolved field selection of one concrete struct type: its
// reflected identity, the selected direct fields sorted bytewise by name, and
// the designated identity field.
type Schema struct {
	Type        reflect.Type
	PackagePath string
	Name        string
	Fields      []Binding
	Names       []string
	identity    int
}

// Binding binds one selected direct struct field.
type Binding struct {
	Name  string
	Index int
	Role  Role
	Type  reflect.Type
	// Kind is the persisted primitive kind; reflect.Array for the identity.
	Kind reflect.Kind
}

// Key returns the schema's concrete type key.
func (s *Schema) Key() TypeKey {
	return TypeKey{PackagePath: s.PackagePath, Name: s.Name}
}

// IdentityField returns the designated identity field binding.
func (s *Schema) IdentityField() Binding {
	return s.Fields[s.identity]
}

// Descriptor returns the information identity derivation uses.
func (s *Schema) Descriptor() identity.Descriptor {
	return identity.Descriptor{PackagePath: s.PackagePath, TypeName: s.Name, FieldNames: s.Names, IdentityField: s.IdentityField().Name}
}

// SameShape reports whether two schemas select the same fields with the same
// roles and Go types.
func (s *Schema) SameShape(other *Schema) bool {
	if s.Type != other.Type || len(s.Fields) != len(other.Fields) {
		return false
	}
	for i := range s.Fields {
		a, b := s.Fields[i], other.Fields[i]
		if a.Name != b.Name || a.Index != b.Index || a.Role != b.Role || a.Type != b.Type {
			return false
		}
	}
	return true
}

// FieldValue returns a settable reflect.Value for a bound field, including
// unexported fields, on a pointer-to-struct instance.
func (s *Schema) FieldValue(instance reflect.Value, binding Binding) reflect.Value {
	field := instance.Elem().Field(binding.Index)
	return reflect.NewAt(binding.Type, unsafe.Pointer(field.UnsafeAddr())).Elem()
}

// Resolve validates a pointer-to-struct instance's selections and binds every
// selector to exactly one direct struct field.
func Resolve(instance any, selections []Selection) (*Schema, error) {
	value := reflect.ValueOf(instance)
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() || value.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("%T must be a non-nil pointer to a struct", instance)
	}
	structType := value.Type().Elem()
	if structType.Name() == "" {
		return nil, fmt.Errorf("%s must be a named struct type", structType)
	}
	schema := &Schema{Type: structType, PackagePath: structType.PkgPath(), Name: structType.Name(), identity: -1}
	if schema.PackagePath == "" {
		return nil, fmt.Errorf("%s has no package path", structType)
	}
	probe := reflect.New(structType)
	seen := make(map[string]bool, len(selections))
	for i, selection := range selections {
		binding, err := bind(probe, structType, selection)
		if err != nil {
			return nil, fmt.Errorf("%s field %d: %w", structType, i, err)
		}
		if seen[binding.Name] {
			return nil, fmt.Errorf("%s: field %q selected more than once", structType, binding.Name)
		}
		seen[binding.Name] = true
		schema.Fields = append(schema.Fields, binding)
	}
	sort.Slice(schema.Fields, func(i, j int) bool {
		return schema.Fields[i].Name < schema.Fields[j].Name
	})
	for i, binding := range schema.Fields {
		schema.Names = append(schema.Names, binding.Name)
		if binding.Role != RoleIdentity {
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
	if len(schema.Fields) > identity.MaxFieldCount {
		return nil, fmt.Errorf("%s: %d fields exceed the limit of %d", structType, len(schema.Fields), identity.MaxFieldCount)
	}
	return schema, nil
}

func bind(probe reflect.Value, structType reflect.Type, selection Selection) (Binding, error) {
	if selection.role == 0 {
		return Binding{}, errors.New("zero Field; use FieldOf, FieldOfNullable, or IdentityOf")
	}
	if selection.locate == nil {
		return Binding{}, errors.New("field selector is nil")
	}
	if selection.owner != structType {
		return Binding{}, fmt.Errorf("selector is for %s, not %s", selection.owner, structType)
	}
	located, err := selection.locate(probe)
	if err != nil {
		return Binding{}, err
	}
	if !located.IsValid() || located.IsNil() {
		return Binding{}, errors.New("selector returned a nil pointer")
	}
	address := located.Pointer()
	match := -1
	for i := 0; i < structType.NumField(); i++ {
		if structType.Field(i).Type != selection.target || probe.Elem().Field(i).UnsafeAddr() != address {
			continue
		}
		if match >= 0 {
			return Binding{}, errors.New("selector matches more than one direct field")
		}
		match = i
	}
	if match < 0 {
		return Binding{}, errors.New("selector must return the address of a direct field of the struct")
	}
	field := structType.Field(match)
	if field.Name == "" || !utf8.ValidString(field.Name) {
		return Binding{}, fmt.Errorf("invalid field name %q", field.Name)
	}
	if len(field.Name) > identity.MaxComponentBytes {
		return Binding{}, fmt.Errorf("field name exceeds %d bytes", identity.MaxComponentBytes)
	}
	binding := Binding{Name: field.Name, Index: match, Role: selection.role, Type: field.Type}
	switch selection.role {
	case RoleNullable:
		binding.Kind = field.Type.Elem().Kind()
	case RoleIdentity:
		binding.Kind = reflect.Array
	default:
		binding.Kind = field.Type.Kind()
	}
	return binding, nil
}
