package eventlog

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"unicode/utf8"
)

const (
	fieldTagNull = "null"
	fieldTagUUID = "uuid"
)

// fieldValue is the strict type-tagged persisted form of one field value.
// Value holds a string for string, integer, float, and uuid tags, a bool for the
// bool tag, and nil for null.
type fieldValue struct {
	Type  string `json:"type"`
	Value any    `json:"value,omitempty"`
}

// fieldSnapshot is an immutable, ordered capture of a schema's selected fields.
type fieldSnapshot []fieldValue

func (s fieldSnapshot) equal(other fieldSnapshot) bool {
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

func (s fieldSnapshot) toRecordFields(schema *typeSchema) map[string]interface{} {
	out := make(map[string]interface{}, len(s))
	for i, value := range s {
		out[schema.fields[i].name] = value
	}
	return out
}

// captureSnapshot reads and validates every selected field of instance. It also
// returns the raw identity value.
func captureSnapshot(schema *typeSchema, instance reflect.Value) (fieldSnapshot, uuid, error) {
	snapshot := make(fieldSnapshot, len(schema.fields))
	var identity uuid
	for i, binding := range schema.fields {
		field := schema.fieldValueOf(instance, binding)
		if binding.role == fieldRoleIdentity {
			reflect.Copy(reflect.ValueOf(identity[:]), field)
			snapshot[i] = fieldValue{Type: fieldTagUUID, Value: identity.String()}
			continue
		}
		if binding.role == fieldRoleNullable {
			if field.IsNil() {
				snapshot[i] = fieldValue{Type: fieldTagNull}
				continue
			}
			field = field.Elem()
		}
		value, err := encodePrimitive(field)
		if err != nil {
			return nil, uuid{}, fmt.Errorf("field %q: %w", binding.name, err)
		}
		snapshot[i] = value
	}
	return snapshot, identity, nil
}

func encodePrimitive(field reflect.Value) (fieldValue, error) {
	kind := field.Kind()
	switch kind {
	case reflect.String:
		text := field.String()
		if !utf8.ValidString(text) {
			return fieldValue{}, errors.New("string value is not valid UTF-8")
		}
		return fieldValue{Type: kind.String(), Value: text}, nil
	case reflect.Bool:
		return fieldValue{Type: kind.String(), Value: field.Bool()}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fieldValue{Type: kind.String(), Value: strconv.FormatInt(field.Int(), 10)}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fieldValue{Type: kind.String(), Value: strconv.FormatUint(field.Uint(), 10)}, nil
	case reflect.Float32:
		number := float32(field.Float())
		if math.IsNaN(float64(number)) || math.IsInf(float64(number), 0) {
			return fieldValue{}, errors.New("float32 value is not finite")
		}
		return fieldValue{Type: kind.String(), Value: fmt.Sprintf("%08x", math.Float32bits(number))}, nil
	case reflect.Float64:
		number := field.Float()
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return fieldValue{}, errors.New("float64 value is not finite")
		}
		return fieldValue{Type: kind.String(), Value: fmt.Sprintf("%016x", math.Float64bits(number))}, nil
	default:
		return fieldValue{}, fmt.Errorf("unsupported field kind %s", kind)
	}
}

// fieldValueFromRecord converts a decoded JSON field object into fieldValue.
func fieldValueFromRecord(raw interface{}) (fieldValue, error) {
	object, ok := raw.(map[string]interface{})
	if !ok {
		return fieldValue{}, fmt.Errorf("field value is %T, not an object", raw)
	}
	tag, ok := object["type"].(string)
	if !ok {
		return fieldValue{}, errors.New("field value has no type tag")
	}
	return fieldValue{Type: tag, Value: object["value"]}, nil
}

// validatePersistedValue checks value-level rules the JSON Schema cannot
// express: exact integer ranges, finite floats, and canonical UUID text.
func validatePersistedValue(value fieldValue) error {
	switch value.Type {
	case fieldTagNull, "string", "bool":
		return nil
	case fieldTagUUID:
		text, _ := value.Value.(string)
		_, err := parseUUID(text)
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

// decodeFieldInto sets a bound field from its persisted value, preserving the
// field's concrete type, width, and nullability.
func decodeFieldInto(binding fieldBinding, target reflect.Value, value fieldValue) error {
	if err := validatePersistedValue(value); err != nil {
		return fmt.Errorf("field %q: %w", binding.name, err)
	}
	switch binding.role {
	case fieldRoleIdentity:
		if value.Type != fieldTagUUID {
			return fmt.Errorf("identity field %q has type %q, want uuid", binding.name, value.Type)
		}
		parsed, _ := parseUUID(value.Value.(string))
		reflect.Copy(target, reflect.ValueOf(parsed[:]))
		return nil
	case fieldRoleNullable:
		if value.Type == fieldTagNull {
			target.SetZero()
			return nil
		}
		element := reflect.New(binding.typ.Elem())
		if err := decodePrimitiveInto(binding, element.Elem(), value); err != nil {
			return err
		}
		target.Set(element)
		return nil
	default:
		return decodePrimitiveInto(binding, target, value)
	}
}

func decodePrimitiveInto(binding fieldBinding, target reflect.Value, value fieldValue) error {
	if value.Type != binding.kind.String() {
		return fmt.Errorf("field %q has persisted type %q, want %q", binding.name, value.Type, binding.kind.String())
	}
	switch binding.kind {
	case reflect.String:
		target.SetString(value.Value.(string))
		return nil
	case reflect.Bool:
		flag, ok := value.Value.(bool)
		if !ok {
			return fmt.Errorf("field %q bool value is %T", binding.name, value.Value)
		}
		target.SetBool(flag)
		return nil
	}
	parsed, err := parsePrimitive(binding.kind, value.Value.(string))
	if err != nil {
		return fmt.Errorf("field %q: %w", binding.name, err)
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
