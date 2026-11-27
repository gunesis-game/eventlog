package identity

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"strings"
	"testing"
)

func filled(n byte) UUID {
	var u UUID
	for i := range u {
		u[i] = n
	}
	return u
}

func shippedDescriptor() Descriptor {
	return Descriptor{PackagePath: "github.com/gunesis-game/eventlog", TypeName: "OrderShipped", FieldNames: []string{"Carrier", "ID"}, IdentityField: "ID"}
}

func appendFramed(out []byte, s string) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(s)))
	return append(out, s...)
}

// The canonical identity input matches the approved byte layout exactly.
func TestCanonicalInputLayout(t *testing.T) {
	value := filled(9)
	got, err := CanonicalInput(EventDomain, shippedDescriptor(), value)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("eventlog\x00event\x00v1\x00")
	want = append(want, 0x01)
	want = appendFramed(want, "github.com/gunesis-game/eventlog")
	want = appendFramed(want, "OrderShipped")
	want = binary.BigEndian.AppendUint32(want, 2)
	want = appendFramed(want, "Carrier")
	want = appendFramed(want, "ID")
	want = appendFramed(want, "ID")
	want = append(want, 0x03)
	want = append(want, value[:]...)
	if !bytes.Equal(got, want) {
		t.Fatalf("canonical input mismatch:\n got %x\nwant %x", got, want)
	}

	id, err := Derive(EventDomain, shippedDescriptor(), value)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha1.Sum(append(append([]byte(nil), namespace[:]...), want...))
	var expected UUID
	copy(expected[:], sum[:16])
	expected[6] = expected[6]&0x0f | 0x50
	expected[8] = expected[8]&0x3f | 0x80
	if id != expected {
		t.Fatalf("UUID v5 = %s, want %s", id, expected)
	}
}

// L-05: identity depends on type, schema, domain, and identity value only.
func TestDeriveRules(t *testing.T) {
	d := shippedDescriptor()
	a, _ := Derive(EventDomain, d, filled(1))
	again, _ := Derive(EventDomain, d, filled(1))
	other, _ := Derive(EventDomain, d, filled(2))
	otherType := d
	otherType.TypeName = "OrderPlaced"
	typed, _ := Derive(EventDomain, otherType, filled(1))
	otherSchema := d
	otherSchema.FieldNames = []string{"Amount", "Carrier", "ID"}
	schema, _ := Derive(EventDomain, otherSchema, filled(1))
	asChannel, _ := Derive(ChannelDomain, d, filled(1))
	if a != again {
		t.Fatal("identity must be deterministic")
	}
	for _, different := range []UUID{other, typed, schema, asChannel} {
		if a == different {
			t.Fatal("identity must differ by value, type, schema, and domain")
		}
	}
	if _, err := Derive(EventDomain, d, UUID{}); err == nil {
		t.Fatal("zero UUID identity must be rejected")
	}
}

func TestCanonicalInputLimits(t *testing.T) {
	cases := map[string]func(*Descriptor){
		"over-long package path": func(d *Descriptor) { d.PackagePath = strings.Repeat("p", MaxComponentBytes+1) },
		"invalid UTF-8":          func(d *Descriptor) { d.TypeName = "\xff" },
		"too many fields":        func(d *Descriptor) { d.FieldNames = make([]string, MaxFieldCount+1) },
		"aggregate over 1 MiB": func(d *Descriptor) {
			d.FieldNames = make([]string, 300)
			for i := range d.FieldNames {
				d.FieldNames[i] = strings.Repeat("n", MaxComponentBytes)
			}
		},
	}
	for name, mutate := range cases {
		d := shippedDescriptor()
		mutate(&d)
		if _, err := CanonicalInput(EventDomain, d, filled(1)); err == nil {
			t.Fatalf("%s: expected rejection", name)
		}
	}
}
