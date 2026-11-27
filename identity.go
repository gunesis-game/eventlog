package eventlog

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

// identityNamespace is the fixed UUID v5 namespace for eventlog identities.
var identityNamespace = uuid{0x26, 0x61, 0x2f, 0x1d, 0xf6, 0x48, 0x48, 0x3a, 0x9c, 0xfe, 0x59, 0xc1, 0x8f, 0x11, 0x38, 0x91}

const (
	eventIdentityDomain   = "eventlog\x00event\x00v1\x00"
	channelIdentityDomain = "eventlog\x00channel\x00v1\x00"

	identityFormatVersion  = 0x01
	identityUUIDTag        = 0x03
	identityMaxComponent   = 4 << 10
	identityMaxFieldCount  = 1024
	identityMaxCanonicalIn = 1 << 20
)

// canonicalIdentityInput builds the approved v1 canonical identity encoding:
// domain prefix, format byte, package path, type name, sorted schema field
// names, identity field name, and the UUID-tagged identity value.
func canonicalIdentityInput(domain string, schema *typeSchema, value uuid) ([]byte, error) {
	if len(schema.names) > identityMaxFieldCount {
		return nil, fmt.Errorf("identity schema has %d fields, limit is %d", len(schema.names), identityMaxFieldCount)
	}
	out := make([]byte, 0, 256)
	out = append(out, domain...)
	out = append(out, identityFormatVersion)
	var err error
	if out, err = appendIdentityComponent(out, "package path", schema.pkgPath); err != nil {
		return nil, err
	}
	if out, err = appendIdentityComponent(out, "type name", schema.name); err != nil {
		return nil, err
	}
	out = binary.BigEndian.AppendUint32(out, uint32(len(schema.names)))
	for _, name := range schema.names {
		if out, err = appendIdentityComponent(out, "field name", name); err != nil {
			return nil, err
		}
	}
	if out, err = appendIdentityComponent(out, "identity field name", schema.identityField().name); err != nil {
		return nil, err
	}
	out = append(out, identityUUIDTag)
	out = append(out, value[:]...)
	if len(out) > identityMaxCanonicalIn {
		return nil, fmt.Errorf("canonical identity input is %d bytes, limit is %d", len(out), identityMaxCanonicalIn)
	}
	return out, nil
}

func appendIdentityComponent(out []byte, label, value string) ([]byte, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("identity %s is not valid UTF-8", label)
	}
	if len(value) > identityMaxComponent {
		return nil, fmt.Errorf("identity %s is %d bytes, limit is %d", label, len(value), identityMaxComponent)
	}
	out = binary.BigEndian.AppendUint32(out, uint32(len(value)))
	return append(out, value...), nil
}

// deriveIdentity returns the deterministic UUID v5 logical identity.
func deriveIdentity(domain string, schema *typeSchema, value uuid) (uuid, error) {
	if value.isZero() {
		return uuid{}, fmt.Errorf("identity field %q of %s.%s is the zero UUID", schema.identityField().name, schema.pkgPath, schema.name)
	}
	input, err := canonicalIdentityInput(domain, schema, value)
	if err != nil {
		return uuid{}, err
	}
	hash := sha1.New()
	_, _ = hash.Write(identityNamespace[:])
	_, _ = hash.Write(input)
	var id uuid
	copy(id[:], hash.Sum(nil))
	id[6] = (id[6] & 0x0f) | 0x50
	id[8] = (id[8] & 0x3f) | 0x80
	return id, nil
}
