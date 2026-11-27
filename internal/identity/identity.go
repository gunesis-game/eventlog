package identity

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

// namespace is the fixed UUID v5 namespace for eventlog identities.
var namespace = UUID{0x26, 0x61, 0x2f, 0x1d, 0xf6, 0x48, 0x48, 0x3a, 0x9c, 0xfe, 0x59, 0xc1, 0x8f, 0x11, 0x38, 0x91}

const (
	// EventDomain and ChannelDomain prefix the canonical input so Event and
	// Channel identities never collide.
	EventDomain   = "eventlog\x00event\x00v1\x00"
	ChannelDomain = "eventlog\x00channel\x00v1\x00"

	// MaxComponentBytes bounds the package path, type name, and each field name.
	MaxComponentBytes = 4 << 10
	// MaxFieldCount bounds the exposed schema field count.
	MaxFieldCount = 1024
	// MaxCanonicalInputBytes bounds the complete canonical identity input.
	MaxCanonicalInputBytes = 1 << 20

	formatVersion = 0x01
	uuidTag       = 0x03
)

// Descriptor is the type and schema information that identity derives from.
// FieldNames must be sorted bytewise and include IdentityField.
type Descriptor struct {
	PackagePath   string
	TypeName      string
	FieldNames    []string
	IdentityField string
}

// CanonicalInput builds the approved v1 canonical identity encoding: domain
// prefix, format byte, package path, type name, sorted schema field names,
// identity field name, and the UUID-tagged identity value.
func CanonicalInput(domain string, descriptor Descriptor, value UUID) ([]byte, error) {
	if len(descriptor.FieldNames) > MaxFieldCount {
		return nil, fmt.Errorf("identity schema has %d fields, limit is %d", len(descriptor.FieldNames), MaxFieldCount)
	}
	out := make([]byte, 0, 256)
	out = append(out, domain...)
	out = append(out, formatVersion)
	var err error
	if out, err = appendComponent(out, "package path", descriptor.PackagePath); err != nil {
		return nil, err
	}
	if out, err = appendComponent(out, "type name", descriptor.TypeName); err != nil {
		return nil, err
	}
	out = binary.BigEndian.AppendUint32(out, uint32(len(descriptor.FieldNames)))
	for _, name := range descriptor.FieldNames {
		if out, err = appendComponent(out, "field name", name); err != nil {
			return nil, err
		}
	}
	if out, err = appendComponent(out, "identity field name", descriptor.IdentityField); err != nil {
		return nil, err
	}
	out = append(out, uuidTag)
	out = append(out, value[:]...)
	if len(out) > MaxCanonicalInputBytes {
		return nil, fmt.Errorf("canonical identity input is %d bytes, limit is %d", len(out), MaxCanonicalInputBytes)
	}
	return out, nil
}

func appendComponent(out []byte, label, value string) ([]byte, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("identity %s is not valid UTF-8", label)
	}
	if len(value) > MaxComponentBytes {
		return nil, fmt.Errorf("identity %s is %d bytes, limit is %d", label, len(value), MaxComponentBytes)
	}
	out = binary.BigEndian.AppendUint32(out, uint32(len(value)))
	return append(out, value...), nil
}

// Derive returns the deterministic UUID v5 logical identity.
func Derive(domain string, descriptor Descriptor, value UUID) (UUID, error) {
	if value.IsZero() {
		return UUID{}, fmt.Errorf("identity field %q of %s.%s is the zero UUID", descriptor.IdentityField, descriptor.PackagePath, descriptor.TypeName)
	}
	input, err := CanonicalInput(domain, descriptor, value)
	if err != nil {
		return UUID{}, err
	}
	hash := sha1.New()
	_, _ = hash.Write(namespace[:])
	_, _ = hash.Write(input)
	var id UUID
	copy(id[:], hash.Sum(nil))
	id[6] = (id[6] & 0x0f) | 0x50
	id[8] = (id[8] & 0x3f) | 0x80
	return id, nil
}
