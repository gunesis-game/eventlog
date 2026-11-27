package fieldschema

import (
	"math"
	"reflect"
	"testing"
)

type Celsius float32
type Flag bool

type allKinds struct {
	ID   ID
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

func (a *allKinds) selections() []Selection {
	return []Selection{
		identityOf(func(a *allKinds) *ID { return &a.ID }),
		fieldOf(func(a *allKinds) *string { return &a.S }),
		fieldOf(func(a *allKinds) *Flag { return &a.B }),
		fieldOf(func(a *allKinds) *int { return &a.I }),
		fieldOf(func(a *allKinds) *int8 { return &a.I8 }),
		fieldOf(func(a *allKinds) *int16 { return &a.I16 }),
		fieldOf(func(a *allKinds) *int32 { return &a.I32 }),
		fieldOf(func(a *allKinds) *int64 { return &a.I64 }),
		fieldOf(func(a *allKinds) *uint { return &a.U }),
		fieldOf(func(a *allKinds) *uint8 { return &a.U8 }),
		fieldOf(func(a *allKinds) *uint16 { return &a.U16 }),
		fieldOf(func(a *allKinds) *uint32 { return &a.U32 }),
		fieldOf(func(a *allKinds) *uint64 { return &a.U64 }),
		fieldOf(func(a *allKinds) *Celsius { return &a.F32 }),
		fieldOf(func(a *allKinds) *float64 { return &a.F64 }),
		nullableOf(func(a *allKinds) **string { return &a.NS }),
		nullableOf(func(a *allKinds) **int8 { return &a.NI }),
		nullableOf(func(a *allKinds) **float64 { return &a.NF }),
		nullableOf(func(a *allKinds) **uint32 { return &a.NNil }),
	}
}

func roundTrip(t *testing.T, source *allKinds) *allKinds {
	t.Helper()
	schema, err := Resolve(source, source.selections())
	if err != nil {
		t.Fatal(err)
	}
	snapshot, _, err := Capture(schema, reflect.ValueOf(source))
	if err != nil {
		t.Fatal(err)
	}
	target := reflect.New(schema.Type)
	for i, binding := range schema.Fields {
		if err := Decode(binding, schema.FieldValue(target, binding), snapshot[i]); err != nil {
			t.Fatalf("decode %s: %v", binding.Name, err)
		}
	}
	return target.Interface().(*allKinds)
}

// L-09: every supported type, width, named type, and nullability round-trips.
func TestFieldValuesRoundTripPreservingTypesAndNullability(t *testing.T) {
	zeroInt8 := int8(0)
	negativeZero := math.Copysign(0, -1)
	source := &allKinds{
		ID: filled(7), S: "héllo", B: true,
		I: math.MinInt64, I8: math.MinInt8, I16: math.MaxInt16, I32: math.MinInt32, I64: math.MaxInt64,
		U: math.MaxUint64, U8: math.MaxUint8, U16: math.MaxUint16, U32: math.MaxUint32, U64: math.MaxUint64,
		F32: Celsius(math.SmallestNonzeroFloat32), F64: negativeZero,
		NS: pointerTo(""), NI: &zeroInt8, NF: &negativeZero,
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
		schema, _ := Resolve(source, source.selections())
		if _, _, err := Capture(schema, reflect.ValueOf(source)); err == nil {
			t.Fatalf("%s: expected capture error", name)
		}
	}
}

func TestPersistedValueValidationEnforcesRanges(t *testing.T) {
	invalid := []Value{
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
		if err := ValidatePersisted(value); err == nil {
			t.Fatalf("expected %v to be rejected", value)
		}
	}
	valid := []Value{{Type: "int8", Value: "-128"}, {Type: "uint64", Value: "18446744073709551615"}, {Type: "null"}, {Type: "bool", Value: false}}
	for _, value := range valid {
		if err := ValidatePersisted(value); err != nil {
			t.Fatalf("%v: %v", value, err)
		}
	}
}

func TestDecodeRejectsTypeAndNullabilityMismatch(t *testing.T) {
	schema, _ := Resolve(&allKinds{}, (&allKinds{}).selections())
	target := reflect.New(schema.Type)
	byName := map[string]Binding{}
	for _, binding := range schema.Fields {
		byName[binding.Name] = binding
	}
	cases := []struct {
		field string
		value Value
	}{
		{"I8", Value{Type: "int16", Value: "1"}},
		{"I8", Value{Type: "null"}},
		{"ID", Value{Type: "string", Value: "x"}},
		{"NS", Value{Type: "int", Value: "1"}},
	}
	for _, c := range cases {
		binding := byName[c.field]
		if err := Decode(binding, schema.FieldValue(target, binding), c.value); err == nil {
			t.Fatalf("%s: expected rejection of %v", c.field, c.value)
		}
	}
}

type ID [16]byte

func filled(n byte) ID {
	var u ID
	for i := range u {
		u[i] = n
	}
	return u
}

func pointerTo(s string) *string {
	return &s
}

// Test helpers mirroring the root package's public selectors.
func fieldOf[T any, V any](selector func(*T) *V) Selection {
	return Select[T, *V](RoleValue, selector)
}

func nullableOf[T any, V any](selector func(*T) **V) Selection {
	return Select[T, **V](RoleNullable, selector)
}

func identityOf[T any, U any](selector func(*T) *U) Selection {
	return Select[T, *U](RoleIdentity, selector)
}
