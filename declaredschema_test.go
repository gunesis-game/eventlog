package eventlog

import (
	"strings"
	"testing"
)

type panickingFields struct {
	ID OrderID
}

func (p *panickingFields) Fields() Fields {
	panic("boom")
}

type noFields struct {
	ID OrderID
}

func TestResolveSchemaReadsFieldsDeclaration(t *testing.T) {
	schema, err := resolveSchema(&OrderPlaced{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(schema.Names, ",") != "Amount,ID,Note,Status" {
		t.Fatalf("names = %v", schema.Names)
	}
	if _, err := resolveSchema(&panickingFields{}); err == nil || !strings.Contains(err.Error(), "Fields() panicked") {
		t.Fatalf("panicking Fields: %v", err)
	}
	if _, err := resolveSchema(&noFields{}); err == nil {
		t.Fatal("a type without Fields() must be rejected")
	}
	if _, err := resolveSchema(&OrderPlaced{}); err != nil {
		t.Fatal(err)
	}
}
