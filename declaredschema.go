package eventlog

import (
	"fmt"

	"github.com/gunesis-game/eventlog/internal/fieldschema"
)

type fieldsDeclarer interface {
	Fields() Fields
}

// resolveSchema resolves an Event or Channel instance's Fields declaration.
func resolveSchema(instance any) (*fieldschema.Schema, error) {
	declarer, ok := instance.(fieldsDeclarer)
	if !ok {
		return nil, fmt.Errorf("%T does not implement Fields() Fields", instance)
	}
	declared, err := callFields(declarer)
	if err != nil {
		return nil, fmt.Errorf("%T: %w", instance, err)
	}
	selections := make([]fieldschema.Selection, len(declared))
	for i, field := range declared {
		selections[i] = field.selection
	}
	return fieldschema.Resolve(instance, selections)
}

func callFields(declarer fieldsDeclarer) (fields Fields, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("Fields() panicked: %v", recovered)
		}
	}()
	return declarer.Fields(), nil
}
