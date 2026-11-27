package eventlog

import (
	"math"
	"reflect"
	"testing"
)

type Celsius float32
type Flag bool

type allKinds struct {
	ID   OrderID
	S    string
	B    Flag
	I    int
	I8   int8
	I16  int16
	I32  int32
	I64  int64
	U    uint
	U8   uint8
	U16  uint16
	U32  uint32
	U64  uint64
	F32  Celsius
	F64  float64
	NS   *string
	NI   *int8
	NF   *float64
	NNil *uint32
}

func (a *allKinds) Fields() Fields {
	return Fields{
		IdentityOf(func(a *allKinds) *OrderID { return &a.ID }),
		FieldOf(func(a *allKinds) *string { return &a.S }),
		FieldOf(func(a *allKinds) *Flag { return &a.B }),
		FieldOf(func(a *allKinds) *int { return &a.I }),
		FieldOf(func(a *allKinds) *int8 { return &a.I8 }),
		FieldOf(func(a *allKinds) *int16 { return &a.I16 }),
		FieldOf(func(a *allKinds) *int32 { return &a.I32 }),
		FieldOf(func(a *allKinds) *int64 { return &a.I64 }),
		FieldOf(func(a *allKinds) *uint { return &a.U }),
		FieldOf(func(a *allKinds) *uint8 { return &a.U8 }),
		FieldOf(func(a *allKinds) *uint16 { return &a.U16 }),
		FieldOf(func(a *allKinds) *uint32 { return &a.U32 }),
		FieldOf(func(a *allKinds) *uint64 { return &a.U64 }),
		FieldOf(func(a *allKinds) *Celsius { return &a.F32 }),
		FieldOf(func(a *allKinds) *float64 { return &a.F64 }),
		FieldOfNullable(func(a *allKinds) **string { return &a.NS }),
		FieldOfNullable(func(a *allKinds) **int8 { return &a.NI }),
		FieldOfNullable(func(a *allKinds) **float64 { return &a.NF }),
		FieldOfNullable(func(a *allKinds) **uint32 { return &a.NNil }),
	}
}

func roundTrip(t *testing.T, source *allKinds) *allKinds {
	t.Helper()
	schema, err := resolveTypeSchema(source)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, _, err := captureSnapshot(schema, reflect.ValueOf(source))
	if err != nil {
		t.Fatal(err)
	}
	target := reflect.New(schema.typ)
	for i, binding := range schema.fields {
		if err := decodeFieldInto(binding, schema.fieldValueOf(target, binding), snapshot[i]); err != nil {
			t.Fatalf("decode %s: %v", binding.name, err)
		}
	}
	return target.Interface().(*allKinds)
}

// L-09: every supported type, width, named type, and nullability round-trips.
func TestFieldValuesRoundTripPreservingTypesAndNullability(t *testing.T) {
	zeroInt8 := int8(0)
	negativeZero := math.Copysign(0, -1)
	source := &allKinds{
		ID: testUUID(7), S: "héllo", B: true,
		I: math.MinInt64, I8: math.MinInt8, I16: math.MaxInt16, I32: math.MinInt32, I64: math.MaxInt64,
		U: math.MaxUint64, U8: math.MaxUint8, U16: math.MaxUint16, U32: math.MaxUint32, U64: math.MaxUint64,
		F32: Celsius(math.SmallestNonzeroFloat32), F64: negativeZero,
		NS: stringPointer(""), NI: &zeroInt8, NF: &negativeZero,
	}
	got := roundTrip(t, source)
	if !reflect.DeepEqual(got, source) {
		t.Fatalf("round trip mismatch:\n got %#v\nwant %#v", got, source)
	}
	if got.NNil != nil {
		t.Fatal("nil nullable must stay nil")
	}
	if got.NI == nil || *got.NI != 0 {
		t.Fatal("pointer to zero must stay distinct from nil")
	}
	if !math.Signbit(got.F64) {
		t.Fatal("negative zero must be preserved")
	}
}

func TestCaptureRejectsNonFiniteFloatsAndInvalidUTF8(t *testing.T) {
	for name, source := range map[string]*allKinds{
		"NaN":       {F64: math.NaN()},
		"Inf":       {F32: Celsius(math.Inf(1))},
		"bad UTF-8": {S: "\xff"},
	} {
		schema, _ := resolveTypeSchema(source)
		if _, _, err := captureSnapshot(schema, reflect.ValueOf(source)); err == nil {
			t.Fatalf("%s: expected capture error", name)
		}
	}
}

func TestPersistedValueValidationEnforcesRanges(t *testing.T) {
	invalid := []fieldValue{
		{Type: "int8", Value: "128"},
		{Type: "uint16", Value: "65536"},
		{Type: "int32", Value: "-2147483649"},
		{Type: "float64", Value: "7ff0000000000000"},
		{Type: "float32", Value: "7fc00000"},
		{Type: "uuid", Value: "not-a-uuid"},
		{Type: "complex64", Value: "1"},
		{Type: "int", Value: 5.0},
	}
	for _, value := range invalid {
		if err := validatePersistedValue(value); err == nil {
			t.Fatalf("expected %v to be rejected", value)
		}
	}
	valid := []fieldValue{{Type: "int8", Value: "-128"}, {Type: "uint64", Value: "18446744073709551615"}, {Type: "null"}, {Type: "bool", Value: false}}
	for _, value := range valid {
		if err := validatePersistedValue(value); err != nil {
			t.Fatalf("%v: %v", value, err)
		}
	}
}

func TestDecodeRejectsTypeAndNullabilityMismatch(t *testing.T) {
	schema, _ := resolveTypeSchema(&allKinds{})
	target := reflect.New(schema.typ)
	byName := map[string]fieldBinding{}
	for _, binding := range schema.fields {
		byName[binding.name] = binding
	}
	cases := []struct {
		field string
		value fieldValue
	}{
		{"I8", fieldValue{Type: "int16", Value: "1"}},
		{"I8", fieldValue{Type: "null"}},
		{"ID", fieldValue{Type: "string", Value: "x"}},
		{"NS", fieldValue{Type: "int", Value: "1"}},
	}
	for _, c := range cases {
		binding := byName[c.field]
		if err := decodeFieldInto(binding, schema.fieldValueOf(target, binding), c.value); err == nil {
			t.Fatalf("%s: expected rejection of %v", c.field, c.value)
		}
	}
}

func TestUUIDTextRoundTrip(t *testing.T) {
	u := uuid{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}
	if u.String() != "01234567-89ab-cdef-0123-456789abcdef" {
		t.Fatal(u.String())
	}
	parsed, err := parseUUID(u.String())
	if err != nil || parsed != u {
		t.Fatalf("parse = %v, %v", parsed, err)
	}
	for _, bad := range []string{"01234567-89AB-cdef-0123-456789abcdef", "0123456789abcdef0123456789abcdef", "01234567-89ab-cdef-0123-456789abcdeg"} {
		if _, err := parseUUID(bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}
