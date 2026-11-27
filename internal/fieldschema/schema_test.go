package fieldschema

import (
	"strings"
	"testing"
)

type nestedInner struct {
	Value int64
}

type schemaProbe struct {
	ID      ID
	Other   ID
	Count   int32
	Label   string
	Maybe   *uint16
	Inner   nestedInner
	private float64
}

type otherProbe struct {
	ID     ID
	Amount int64
}

var foreignInt int32

func TestResolveBindsDirectFieldsSortedByName(t *testing.T) {
	schema, err := Resolve(&schemaProbe{}, []Selection{
		fieldOf(func(p *schemaProbe) *string { return &p.Label }),
		identityOf(func(p *schemaProbe) *ID { return &p.ID }),
		nullableOf(func(p *schemaProbe) **uint16 { return &p.Maybe }),
		fieldOf(func(p *schemaProbe) *float64 { return &p.private }),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(schema.Names, ","); got != "ID,Label,Maybe,private" {
		t.Fatalf("names = %s", got)
	}
	if schema.IdentityField().Name != "ID" {
		t.Fatalf("identity = %s", schema.IdentityField().Name)
	}
	if schema.PackagePath != "github.com/gunesis-game/eventlog/internal/fieldschema" || schema.Name != "schemaProbe" {
		t.Fatalf("type identity = %s.%s", schema.PackagePath, schema.Name)
	}
	if d := schema.Descriptor(); d.IdentityField != "ID" || len(d.FieldNames) != 4 || d.TypeName != "schemaProbe" {
		t.Fatalf("descriptor = %+v", d)
	}
}

func TestResolveRejectsInvalidSelections(t *testing.T) {
	identity := identityOf(func(p *schemaProbe) *ID { return &p.ID })
	cases := map[string][]Selection{
		"missing identity":     {fieldOf(func(p *schemaProbe) *string { return &p.Label })},
		"two identities":       {identity, identityOf(func(p *schemaProbe) *ID { return &p.Other })},
		"duplicate field":      {identity, fieldOf(func(p *schemaProbe) *string { return &p.Label }), fieldOf(func(p *schemaProbe) *string { return &p.Label })},
		"nested field":         {identity, fieldOf(func(p *schemaProbe) *int64 { return &p.Inner.Value })},
		"foreign pointer":      {identity, fieldOf(func(p *schemaProbe) *int32 { return &foreignInt })},
		"nil selector":         {identity, fieldOf[schemaProbe, int32](nil)},
		"zero Selection":       {identity, {}},
		"nil pointer returned": {identity, fieldOf(func(p *schemaProbe) *int32 { return nil })},
		"selector panics":      {identity, nullableOf(func(p *schemaProbe) **uint16 { _ = *p.Maybe; return &p.Maybe })},
		"wrong owner type":     {identity, fieldOf(func(o *otherProbe) *int64 { return &o.Amount })},
	}
	for name, selections := range cases {
		if _, err := Resolve(&schemaProbe{}, selections); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestResolveRejectsNonStructPointers(t *testing.T) {
	selections := []Selection{identityOf(func(p *schemaProbe) *ID { return &p.ID })}
	for _, value := range []any{nil, schemaProbe{}, (*schemaProbe)(nil), new(int)} {
		if _, err := Resolve(value, selections); err == nil {
			t.Fatalf("expected error for %T", value)
		}
	}
}

func TestSchemaSameShape(t *testing.T) {
	probe := []Selection{identityOf(func(p *schemaProbe) *ID { return &p.ID }), fieldOf(func(p *schemaProbe) *int32 { return &p.Count })}
	a, err := Resolve(&schemaProbe{}, probe)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Resolve(&schemaProbe{}, probe)
	if !a.SameShape(b) {
		t.Fatal("identical selections must have the same shape")
	}
	c, _ := Resolve(&schemaProbe{}, probe[:1])
	if a.SameShape(c) {
		t.Fatal("different selections must differ")
	}
}
