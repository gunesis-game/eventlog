package eventlog

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
)

func appendFramed(out []byte, s string) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(s)))
	return append(out, s...)
}

// The canonical identity input matches the approved byte layout exactly.
func TestCanonicalIdentityInputLayout(t *testing.T) {
	schema, err := resolveTypeSchema(&OrderShipped{})
	if err != nil {
		t.Fatal(err)
	}
	value := uuid(testUUID(9))
	got, err := canonicalIdentityInput(eventIdentityDomain, schema, value)
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

	id, err := deriveIdentity(eventIdentityDomain, schema, value)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha1.Sum(append(append([]byte(nil), identityNamespace[:]...), want...))
	var expected uuid
	copy(expected[:], sum[:16])
	expected[6] = expected[6]&0x0f | 0x50
	expected[8] = expected[8]&0x3f | 0x80
	if id != expected {
		t.Fatalf("UUID v5 = %s, want %s", id, expected)
	}
}

// L-05: identity depends on type, schema, and identity value only.
func TestDeriveIdentityRules(t *testing.T) {
	placed, _ := resolveTypeSchema(&OrderPlaced{})
	shipped, _ := resolveTypeSchema(&OrderShipped{})
	a, _ := deriveIdentity(eventIdentityDomain, placed, uuid(testUUID(1)))
	again, _ := deriveIdentity(eventIdentityDomain, placed, uuid(testUUID(1)))
	other, _ := deriveIdentity(eventIdentityDomain, placed, uuid(testUUID(2)))
	otherType, _ := deriveIdentity(eventIdentityDomain, shipped, uuid(testUUID(1)))
	asChannel, _ := deriveIdentity(channelIdentityDomain, placed, uuid(testUUID(1)))
	if a != again {
		t.Fatal("identity must be deterministic")
	}
	if a == other || a == otherType || a == asChannel {
		t.Fatal("identity must differ by value, type, and domain")
	}
	if _, err := deriveIdentity(eventIdentityDomain, placed, uuid{}); err == nil {
		t.Fatal("zero UUID identity must be rejected")
	}

	first := &OrderPlaced{ID: testUUID(1), Amount: 1}
	second := &OrderPlaced{ID: testUUID(1), Amount: 99, Note: stringPointer("x")}
	_, rawFirst, _ := captureSnapshot(placed, reflect.ValueOf(first))
	_, rawSecond, _ := captureSnapshot(placed, reflect.ValueOf(second))
	idFirst, _ := deriveIdentity(eventIdentityDomain, placed, rawFirst)
	idSecond, _ := deriveIdentity(eventIdentityDomain, placed, rawSecond)
	if idFirst != idSecond {
		t.Fatal("non-identity field values must not affect identity")
	}
}

func TestCanonicalIdentityLimits(t *testing.T) {
	schema, _ := resolveTypeSchema(&OrderShipped{})
	long := *schema
	long.pkgPath = strings.Repeat("p", identityMaxComponent+1)
	if _, err := canonicalIdentityInput(eventIdentityDomain, &long, uuid(testUUID(1))); err == nil {
		t.Fatal("over-long package path must be rejected")
	}
	invalid := *schema
	invalid.name = "\xff"
	if _, err := canonicalIdentityInput(eventIdentityDomain, &invalid, uuid(testUUID(1))); err == nil {
		t.Fatal("invalid UTF-8 must be rejected")
	}
	wide := *schema
	wide.names = make([]string, identityMaxFieldCount+1)
	if _, err := canonicalIdentityInput(eventIdentityDomain, &wide, uuid(testUUID(1))); err == nil {
		t.Fatal("too many fields must be rejected")
	}
	large := *schema
	large.names = make([]string, 300)
	for i := range large.names {
		large.names[i] = strings.Repeat("n", identityMaxComponent)
	}
	if _, err := canonicalIdentityInput(eventIdentityDomain, &large, uuid(testUUID(1))); err == nil {
		t.Fatal("aggregate input over 1 MiB must be rejected")
	}
}
