package fieldschema

import (
	"errors"
	"fmt"
	"github.com/gunesis-game/eventlog/internal/identity"
	"math"
	"reflect"
	"strconv"
	"unicode/utf8"
)

const (
	fieldTagNull = "null"
	fieldTagUUID = "uuid"
)

// Value is the strict type-tagged persisted form of one field value.
// Value holds a string for string, integer, float, and uuid tags, a bool for the
// bool tag, and nil for null.
type Value struct {
	Type  string `json:"type"`
	Value any    `json:"value,omitempty"`
}

// Snapshot is an immutable, ordered capture of a schema's selected fields.
type Snapshot []Value

func (s Snapshot) Equal(other Snapshot) bool {
	if len(s) != len(other) {
		return false
	}
	for i := range s {
		if s[i] != other[i] {
			return false
		}
	}
	return true
}

func (s Snapshot) RecordFields(schema *Schema) map[string]interface{} {
	out := make(map[string]interface{}, len(s))
	for i, value := range s {
		out[schema.Fields[i].Name] = value
	}
	return out
}

// Capture reads and validates every selected field of instance. It also
// returns the raw identity value.
func Capture(schema *Schema, instance reflect.Value) (Snapshot, identity.UUID, error) {
	snapshot := make(Snapshot, len(schema.Fields))
	var raw identity.UUID
	for i, binding := range schema.Fields {
		field := schema.FieldValue(instance, binding)
		if binding.Role == RoleIdentity {
			reflect.Copy(reflect.ValueOf(raw[:]), field)
			snapshot[i] = Value{Type: fieldTagUUID, Value: raw.String()}
			continue
		}
		if binding.Role == RoleNullable {
			if field.IsNil() {
				snapshot[i] = Value{Type: fieldTagNull}
				continue
			}
			field = field.Elem()
		}
		value, err := encodePrimitive(field)
		if err != nil {
			return nil, identity.UUID{}, fmt.Errorf("field %q: %w", binding.Name, err)
		}
		snapshot[i] = value
	}
	return snapshot, raw, nil
}

func encodePrimitive(field reflect.Value) (Value, error) {
	kind := field.Kind()
	switch kind {
	case reflect.String:
		text := field.String()
		if !utf8.ValidString(text) {
			return Value{}, errors.New("string value is not valid UTF-8")
		}
		return Value{Type: kind.String(), Value: text}, nil
	case reflect.Bool:
		return Value{Type: kind.String(), Value: field.Bool()}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return Value{Type: kind.String(), Value: strconv.FormatInt(field.Int(), 10)}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return Value{Type: kind.String(), Value: strconv.FormatUint(field.Uint(), 10)}, nil
	case reflect.Float32:
		number := float32(field.Float())
		if math.IsNaN(float64(number)) || math.IsInf(float64(number), 0) {
			return Value{}, errors.New("float32 value is not finite")
		}
		return Value{Type: kind.String(), Value: fmt.Sprintf("%08x", math.Float32bits(number))}, nil
	case reflect.Float64:
		number := field.Float()
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return Value{}, errors.New("float64 value is not finite")
		}
		return Value{Type: kind.String(), Value: fmt.Sprintf("%016x", math.Float64bits(number))}, nil
	default:
		return Value{}, fmt.Errorf("unsupported field kind %s", kind)
	}
}

// ValueFromRecord converts a decoded JSON field object into Value.
func ValueFromRecord(raw interface{}) (Value, error) {
	object, ok := raw.(map[string]interface{})
	if !ok {
		return Value{}, fmt.Errorf("field value is %T, not an object", raw)
	}
	tag, ok := object["type"].(string)
	if !ok {
		return Value{}, errors.New("field value has no type tag")
	}
	return Value{Type: tag, Value: object["value"]}, nil
}

// ValidatePersisted checks value-level rules the JSON Schema cannot
// express: exact integer ranges, finite floats, and canonical UUID text.
func ValidatePersisted(value Value) error {
	switch value.Type {
	case fieldTagNull, "string", "bool":
		return nil
	case fieldTagUUID:
		text, _ := value.Value.(string)
		_, err := identity.ParseUUID(text)
		return err
	}
	kind, ok := persistedKinds[value.Type]
	if !ok {
		return fmt.Errorf("unknown field type tag %q", value.Type)
	}
	text, ok := value.Value.(string)
	if !ok {
		return fmt.Errorf("%s value is not a string", value.Type)
	}
	_, err := parsePrimitive(kind, text)
	return err
}

var persistedKinds = map[string]reflect.Kind{
	"string": reflect.String, "bool": reflect.Bool,
	"int": reflect.Int, "int8": reflect.Int8, "int16": reflect.Int16, "int32": reflect.Int32, "int64": reflect.Int64,
	"uint": reflect.Uint, "uint8": reflect.Uint8, "uint16": reflect.Uint16, "uint32": reflect.Uint32, "uint64": reflect.Uint64,
	"float32": reflect.Float32, "float64": reflect.Float64,
}

// kindBits returns the persisted bit width for a kind. Go int and uint are
// always persisted as 64-bit values regardless of architecture.
func kindBits(kind reflect.Kind) int {
	switch kind {
	case reflect.Int8, reflect.Uint8:
		return 8
	case reflect.Int16, reflect.Uint16:
		return 16
	case reflect.Int32, reflect.Uint32, reflect.Float32:
		return 32
	default:
		return 64
	}
}

// parsePrimitive parses a persisted numeric text value into int64, uint64, or
// float64 bits depending on kind, enforcing exact range and finiteness.
func parsePrimitive(kind reflect.Kind, text string) (interface{}, error) {
	bits := kindBits(kind)
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.ParseInt(text, 10, bits)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.ParseUint(text, 10, bits)
	case reflect.Float32:
		raw, err := strconv.ParseUint(text, 16, 32)
		if err != nil {
			return nil, err
		}
		number := float64(math.Float32frombits(uint32(raw)))
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, errors.New("float32 value is not finite")
		}
		return number, nil
	case reflect.Float64:
		raw, err := strconv.ParseUint(text, 16, 64)
		if err != nil {
			return nil, err
		}
		number := math.Float64frombits(raw)
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, errors.New("float64 value is not finite")
		}
		return number, nil
	default:
		return nil, fmt.Errorf("unsupported numeric kind %s", kind)
	}
}

// Decode sets a bound field from its persisted value, preserving the
// field's concrete type, width, and nullability.
func Decode(binding Binding, target reflect.Value, value Value) error {
	if err := ValidatePersisted(value); err != nil {
		return fmt.Errorf("field %q: %w", binding.Name, err)
	}
	switch binding.Role {
	case RoleIdentity:
		if value.Type != fieldTagUUID {
			return fmt.Errorf("identity field %q has type %q, want uuid", binding.Name, value.Type)
		}
		parsed, _ := identity.ParseUUID(value.Value.(string))
		reflect.Copy(target, reflect.ValueOf(parsed[:]))
		return nil
	case RoleNullable:
		if value.Type == fieldTagNull {
			target.SetZero()
			return nil
		}
		element := reflect.New(binding.Type.Elem())
		if err := decodePrimitive(binding, element.Elem(), value); err != nil {
			return err
		}
		target.Set(element)
		return nil
	default:
		return decodePrimitive(binding, target, value)
	}
}

func decodePrimitive(binding Binding, target reflect.Value, value Value) error {
	if value.Type != binding.Kind.String() {
		return fmt.Errorf("field %q has persisted type %q, want %q", binding.Name, value.Type, binding.Kind.String())
	}
	switch binding.Kind {
	case reflect.String:
		target.SetString(value.Value.(string))
		return nil
	case reflect.Bool:
		flag, ok := value.Value.(bool)
		if !ok {
			return fmt.Errorf("field %q bool value is %T", binding.Name, value.Value)
		}
		target.SetBool(flag)
		return nil
	}
	parsed, err := parsePrimitive(binding.Kind, value.Value.(string))
	if err != nil {
		return fmt.Errorf("field %q: %w", binding.Name, err)
	}
	switch number := parsed.(type) {
	case int64:
		target.SetInt(number)
	case uint64:
		target.SetUint(number)
	case float64:
		target.SetFloat(number)
	}
	return nil
}

// Restore writes a captured snapshot back into instance's selected
// fields. It is used to roll back a rejected transaction.
func Restore(schema *Schema, instance reflect.Value, snapshot Snapshot) error {
	for i, binding := range schema.Fields {
		if err := Decode(binding, schema.FieldValue(instance, binding), snapshot[i]); err != nil {
			return err
		}
	}
	return nil
}

// CopySelected returns a fresh instance holding isolated copies of
// instance's selected field values, whether or not they are valid.
func CopySelected(schema *Schema, instance reflect.Value) reflect.Value {
	duplicate := reflect.New(schema.Type)
	for _, binding := range schema.Fields {
		source := schema.FieldValue(instance, binding)
		target := schema.FieldValue(duplicate, binding)
		if binding.Role == RoleNullable && !source.IsNil() {
			element := reflect.New(binding.Type.Elem())
			element.Elem().Set(source.Elem())
			target.Set(element)
			continue
		}
		target.Set(source)
	}
	return duplicate
}
