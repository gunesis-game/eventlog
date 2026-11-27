package eventlog

import (
	"strings"
	"testing"
)

type nestedInner struct {
	Value int64
}

type schemaProbe struct {
	ID      OrderID
	Other   OrderID
	Count   int32
	Label   string
	Maybe   *uint16
	Inner   nestedInner
	private float64
	fields  func(*schemaProbe) Fields
}

func (p *schemaProbe) Fields() Fields {
	return p.fields(p)
}

var foreignInt int32

func resolveProbe(t *testing.T, declare func(*schemaProbe) Fields) (*typeSchema, error) {
	t.Helper()
	return resolveTypeSchema(&schemaProbe{fields: declare})
}

func TestResolveTypeSchemaBindsDirectFieldsSortedByName(t *testing.T) {
	schema, err := resolveProbe(t, func(*schemaProbe) Fields {
		return Fields{
			FieldOf(func(p *schemaProbe) *string { return &p.Label }),
			IdentityOf(func(p *schemaProbe) *OrderID { return &p.ID }),
			FieldOfNullable(func(p *schemaProbe) **uint16 { return &p.Maybe }),
			FieldOf(func(p *schemaProbe) *float64 { return &p.private }),
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(schema.names, ","); got != "ID,Label,Maybe,private" {
		t.Fatalf("names = %s", got)
	}
	if schema.identityField().name != "ID" {
		t.Fatalf("identity = %s", schema.identityField().name)
	}
	if schema.pkgPath != "github.com/gunesis-game/eventlog" || schema.name != "schemaProbe" {
		t.Fatalf("type identity = %s.%s", schema.pkgPath, schema.name)
	}
}

func TestResolveTypeSchemaRejectsInvalidDeclarations(t *testing.T) {
	identity := IdentityOf(func(p *schemaProbe) *OrderID { return &p.ID })
	cases := map[string]func(*schemaProbe) Fields{
		"missing identity": func(*schemaProbe) Fields {
			return Fields{FieldOf(func(p *schemaProbe) *string { return &p.Label })}
		},
		"two identities": func(*schemaProbe) Fields {
			return Fields{identity, IdentityOf(func(p *schemaProbe) *OrderID { return &p.Other })}
		},
		"duplicate field": func(*schemaProbe) Fields {
			return Fields{identity, FieldOf(func(p *schemaProbe) *string { return &p.Label }), FieldOf(func(p *schemaProbe) *string { return &p.Label })}
		},
		"nested field": func(*schemaProbe) Fields {
			return Fields{identity, FieldOf(func(p *schemaProbe) *int64 { return &p.Inner.Value })}
		},
		"foreign pointer": func(*schemaProbe) Fields {
			return Fields{identity, FieldOf(func(p *schemaProbe) *int32 { return &foreignInt })}
		},
		"nil selector": func(*schemaProbe) Fields {
			return Fields{identity, FieldOf[schemaProbe, int32](nil)}
		},
		"zero Field": func(*schemaProbe) Fields {
			return Fields{identity, {}}
		},
		"nil pointer returned": func(*schemaProbe) Fields {
			return Fields{identity, FieldOf(func(p *schemaProbe) *int32 { return nil })}
		},
		"selector panics": func(*schemaProbe) Fields {
			return Fields{identity, FieldOfNullable(func(p *schemaProbe) **uint16 { _ = *p.Maybe; return &p.Maybe })}
		},
		"wrong owner type": func(*schemaProbe) Fields {
			return Fields{identity, FieldOf(func(o *OrderPlaced) *int64 { return &o.Amount })}
		},
		"Fields panics": func(*schemaProbe) Fields {
			panic("boom")
		},
	}
	for name, declare := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := resolveProbe(t, declare); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestResolveTypeSchemaRejectsNonStructPointers(t *testing.T) {
	for _, value := range []any{nil, schemaProbe{}, (*schemaProbe)(nil), new(int)} {
		if _, err := resolveTypeSchema(value); err == nil {
			t.Fatalf("expected error for %T", value)
		}
	}
}

func TestTypeSchemaSameShape(t *testing.T) {
	a, err := resolveTypeSchema(&OrderPlaced{})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := resolveTypeSchema(&OrderPlaced{})
	if !a.sameShape(b) {
		t.Fatal("identical declarations must have the same shape")
	}
	c, _ := resolveTypeSchema(&OrderShipped{})
	if a.sameShape(c) {
		t.Fatal("different types must differ")
	}
}
