package record

import "testing"

func TestRecordSchemaValidator(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatalf("compile canonical schemas: %v", err)
	}

	tests := []struct {
		name      string
		kind      string
		json      string
		wantValid bool
	}{
		{
			name: "channel valid", kind: KindChannel, wantValid: true,
			json: `{"schema_version":1,"sequence":1,"kind":"channel","id":"12345678-1234-1234-1234-123456789abc","type":{"package_path":"example.com/app","name":"MyChannel"},"fields":{"enabled":{"type":"bool","value":true},"count":{"type":"uint64","value":"18446744073709551615"}}}`,
		},
		{
			name: "event valid", kind: KindEvent, wantValid: true,
			json: `{"schema_version":1,"sequence":2,"kind":"event","id":"12345678-1234-1234-1234-123456789abc","channel_id":"87654321-4321-4321-4321-cba987654321","type":{"package_path":"example.com/app","name":"MyEvent"},"fields":{"message":{"type":"string","value":"hello"},"number":{"type":"int64","value":"-9223372036854775808"}}}`,
		},
		{
			name: "commit valid", kind: KindCommit, wantValid: true,
			json: `{"schema_version":1,"sequence":3,"kind":"commit","transaction_id":1,"record_count":2,"checksum":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`,
		},
		{
			name: "unknown field property rejected", kind: KindEvent,
			json: `{"schema_version":1,"sequence":2,"kind":"event","id":"12345678-1234-1234-1234-123456789abc","channel_id":"87654321-4321-4321-4321-cba987654321","type":{"package_path":"example.com/app","name":"MyEvent"},"fields":{"value":{"type":"string","value":"x","extra":true}}}`,
		},
		{
			name: "noncanonical signed integer rejected", kind: KindEvent,
			json: `{"schema_version":1,"sequence":2,"kind":"event","id":"12345678-1234-1234-1234-123456789abc","channel_id":"87654321-4321-4321-4321-cba987654321","type":{"package_path":"example.com/app","name":"MyEvent"},"fields":{"number":{"type":"int64","value":"01"}}}`,
		},
		{
			name: "wrong float32 bit width rejected", kind: KindEvent,
			json: `{"schema_version":1,"sequence":2,"kind":"event","id":"12345678-1234-1234-1234-123456789abc","channel_id":"87654321-4321-4321-4321-cba987654321","type":{"package_path":"example.com/app","name":"MyEvent"},"fields":{"number":{"type":"float32","value":"000000000"}}}`,
		},
		{
			name: "uppercase uuid rejected", kind: KindChannel,
			json: `{"schema_version":1,"sequence":1,"kind":"channel","id":"12345678-1234-1234-1234-123456789ABC","type":{"package_path":"example.com/app","name":"MyChannel"},"fields":{}}`,
		},
		{
			name: "unknown envelope property rejected", kind: KindCommit,
			json: `{"schema_version":1,"sequence":3,"kind":"commit","transaction_id":1,"record_count":2,"checksum":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","extra":false}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.kind, []byte(tt.json))
			if tt.wantValid && err != nil {
				t.Fatalf("expected valid record, got error: %v", err)
			}
			if !tt.wantValid && err == nil {
				t.Fatal("expected invalid record to be rejected")
			}
		})
	}
}

func TestRecordSchemaValidatorRejectsUnknownKind(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatalf("compile canonical schemas: %v", err)
	}
	if err := validator.Validate("unknown", []byte(`{}`)); err == nil {
		t.Fatal("expected unsupported kind to fail")
	}
}
