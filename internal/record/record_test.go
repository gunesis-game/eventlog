package record

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func testValidator(t *testing.T) *Validator {
	t.Helper()
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}
	return v
}

func TestEncodeDecodeRecord(t *testing.T) {
	v := testValidator(t)
	record := Event{
		SchemaVersion: 1,
		Sequence:      2,
		Kind:          KindEvent,
		ID:            "12345678-1234-1234-1234-123456789abc",
		ChannelID:     "87654321-4321-4321-4321-cba987654321",
		Type:          Type{PackagePath: "example.org/app", Name: "Event"},
		Fields:        map[string]interface{}{"message": map[string]interface{}{"type": "string", "value": "hello"}},
	}
	frame, err := Encode(KindEvent, record, v)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !bytes.HasSuffix(frame, []byte{'\n'}) {
		t.Fatal("encoded frame must end with newline")
	}
	decoded, err := Decode(bytes.TrimSuffix(frame, []byte{'\n'}), v)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, ok := decoded.(Event)
	if !ok {
		t.Fatalf("decoded record has type %T, want Event", decoded)
	}
	if got.Sequence != record.Sequence || got.ID != record.ID || got.ChannelID != record.ChannelID || got.Type != record.Type {
		t.Fatalf("decoded record mismatch: %#v", got)
	}
}

func TestEncodeRecordRejectsSchemaInvalid(t *testing.T) {
	v := testValidator(t)
	record := Commit{SchemaVersion: 1, Sequence: 3, Kind: KindCommit, TransactionID: 1, RecordCount: 1, Checksum: "not-a-checksum"}
	if _, err := Encode(KindCommit, record, v); err == nil {
		t.Fatal("expected schema-invalid record to fail encoding")
	}
}

func TestDecodeRecordRejectsMalformedAndUnsupported(t *testing.T) {
	v := testValidator(t)
	tests := []struct {
		name string
		raw  string
	}{
		{name: "malformed json", raw: `{"schema_version":1`},
		{name: "unsupported version", raw: `{"schema_version":2,"kind":"event"}`},
		{name: "unsupported kind", raw: `{"schema_version":1,"kind":"other"}`},
		{name: "strict schema violation", raw: `{"schema_version":1,"sequence":1,"kind":"commit","transaction_id":1,"record_count":1,"checksum":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","extra":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decode([]byte(tt.raw), v); err == nil {
				t.Fatal("expected decode error")
			}
		})
	}
}

func TestRecordFramer(t *testing.T) {
	input := "one\ntwo\n"
	f := NewFramer(bytes.NewBufferString(input))
	for _, want := range []string{"one", "two"} {
		got, err := f.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if string(got) != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
	if _, err := f.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("got %v, want EOF", err)
	}
}

func TestRecordFramerIncompleteTail(t *testing.T) {
	f := NewFramer(bytes.NewBufferString("complete\npartial"))
	first, err := f.Next()
	if err != nil || string(first) != "complete" {
		t.Fatalf("first frame: %q, %v", first, err)
	}
	partial, err := f.Next()
	if !errors.Is(err, ErrIncompleteRecord) {
		t.Fatalf("got %v, want ErrIncompleteRecord", err)
	}
	if string(partial) != "partial" {
		t.Fatalf("partial bytes %q", partial)
	}
}

func TestRecordFramerEmptyFrame(t *testing.T) {
	f := NewFramer(bytes.NewBufferString("\n"))
	frame, err := f.Next()
	if err != nil {
		t.Fatalf("empty frame read: %v", err)
	}
	if len(frame) != 0 {
		t.Fatalf("frame should be empty, got %q", frame)
	}
	if _, err := Decode(frame, testValidator(t)); err == nil {
		t.Fatal("empty complete record must be malformed")
	}
}
