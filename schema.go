package eventlog

import (
	"bytes"
	"embed"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schemas/*.schema.json
var canonicalSchemas embed.FS

const schemaBase = "https://eventlog.gunesis-game.dev/schemas/v1/"

const (
	recordKindChannel = "channel"
	recordKindEvent   = "event"
	recordKindCommit  = "commit"
)

var recordSchemaFiles = map[string]string{
	recordKindChannel: "channel.schema.json",
	recordKindEvent:   "event.schema.json",
	recordKindCommit:  "commit.schema.json",
}

var canonicalSchemaFiles = []string{
	"field-value.schema.json",
	"event-type.schema.json",
	"channel.schema.json",
	"event.schema.json",
	"commit.schema.json",
}

// recordSchemaValidator validates persisted JSON records against the canonical,
// embedded JSON Schema documents. It is internal so validation does not expand
// eventlog's consumer-facing API.
type recordSchemaValidator struct {
	compiled map[string]*jsonschema.Schema
}

func newRecordSchemaValidator() (*recordSchemaValidator, error) {
	compiler := jsonschema.NewCompiler()

	// Register all schemas first, allowing relative references to resolve locally.
	// No schema compilation or validation requires network access.
	for _, name := range canonicalSchemaFiles {
		raw, err := canonicalSchemas.ReadFile("schemas/" + name)
		if err != nil {
			return nil, fmt.Errorf("read embedded schema %s: %w", name, err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("parse embedded schema %s: %w", name, err)
		}
		if err := compiler.AddResource(schemaBase+name, doc); err != nil {
			return nil, fmt.Errorf("register embedded schema %s: %w", name, err)
		}
	}

	compiled := make(map[string]*jsonschema.Schema, len(recordSchemaFiles))
	for kind, name := range recordSchemaFiles {
		sch, err := compiler.Compile(schemaBase + name)
		if err != nil {
			return nil, fmt.Errorf("compile %s schema: %w", kind, err)
		}
		compiled[kind] = sch
	}
	return &recordSchemaValidator{compiled: compiled}, nil
}

func (v *recordSchemaValidator) validate(kind string, raw []byte) error {
	sch, ok := v.compiled[kind]
	if !ok {
		return fmt.Errorf("unsupported record kind %q", kind)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("decode %s record JSON: %w", kind, err)
	}
	if err := sch.Validate(instance); err != nil {
		return fmt.Errorf("validate %s record: %w", kind, err)
	}
	return nil
}
