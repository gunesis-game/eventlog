package record

import (
	"strings"
	"testing"
)

// B-F01: encode and schema-validate one 1 KiB Event record. Smoke benchmark;
// no numeric targets are approved (D-040).
func BenchmarkEncodeEventRecord(b *testing.B) {
	validator, err := NewValidator()
	if err != nil {
		b.Fatal(err)
	}
	event := Event{SchemaVersion: 1, Sequence: 1, Kind: KindEvent, ID: "01010101-0101-0101-0101-010101010101", ChannelID: "02020202-0202-0202-0202-020202020202", Type: Type{PackagePath: "example.org/bench", Name: "Event"}, Fields: map[string]interface{}{"payload": map[string]interface{}{"type": "string", "value": strings.Repeat("x", 1024)}}}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Encode(KindEvent, event, validator); err != nil {
			b.Fatal(err)
		}
	}
}
