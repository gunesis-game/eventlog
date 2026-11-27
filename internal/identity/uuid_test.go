package identity

import "testing"

func TestUUIDTextRoundTrip(t *testing.T) {
	u := UUID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}
	if u.String() != "01234567-89ab-cdef-0123-456789abcdef" {
		t.Fatal(u.String())
	}
	parsed, err := ParseUUID(u.String())
	if err != nil || parsed != u {
		t.Fatalf("parse = %v, %v", parsed, err)
	}
	for _, bad := range []string{"01234567-89AB-cdef-0123-456789abcdef", "0123456789abcdef0123456789abcdef", "01234567-89ab-cdef-0123-456789abcdeg"} {
		if _, err := ParseUUID(bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
	if !(UUID{}).IsZero() || u.IsZero() {
		t.Fatal("IsZero")
	}
}
