// Package identity defines eventlog's UUID representation and the canonical v1
// identity encoding from which Event and Channel UUID v5 identities derive.
package identity

import (
	"encoding/hex"
	"fmt"
)

// UUID is the 16-byte identity value. Its canonical text form is lowercase,
// hyphenated 8-4-4-4-12 hex.
type UUID [16]byte

// IsZero reports whether u is the all-zero UUID, which is never a valid identity.
func (u UUID) IsZero() bool {
	return u == UUID{}
}

func (u UUID) String() string {
	var buf [36]byte
	hex.Encode(buf[0:8], u[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], u[10:16])
	return string(buf[:])
}

// ParseUUID accepts only the canonical lowercase hyphenated text form.
func ParseUUID(text string) (UUID, error) {
	var u UUID
	if len(text) != 36 || text[8] != '-' || text[13] != '-' || text[18] != '-' || text[23] != '-' {
		return u, fmt.Errorf("invalid UUID %q", text)
	}
	compact := text[0:8] + text[9:13] + text[14:18] + text[19:23] + text[24:36]
	for i := 0; i < len(compact); i++ {
		c := compact[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return u, fmt.Errorf("invalid UUID %q", text)
		}
	}
	if _, err := hex.Decode(u[:], []byte(compact)); err != nil {
		return u, fmt.Errorf("invalid UUID %q: %w", text, err)
	}
	return u, nil
}
