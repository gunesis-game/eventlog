package googletest

import (
	"fmt"
	"strings"
)

// fieldSet is a parsed fields parameter: the selected keys, each with the
// keys selected inside it, or nil to keep the whole value.
type fieldSet map[string]fieldSet

// Default fields Drive returns when a request names none.
const (
	defaultFileFields       = "kind,id,name,mimeType"
	defaultFileListFields   = "kind,incompleteSearch,nextPageToken,files(kind,id,name,mimeType)"
	defaultPermissionFields = "kind,id,type,role"
	defaultPermissionList   = "kind,nextPageToken,permissions(kind,id,type,role)"
)

// parseFields parses a fields parameter such as
// "nextPageToken,files(id,name,owners/emailAddress)"; "*" selects everything.
func parseFields(spec string) (fieldSet, error) {
	if strings.TrimSpace(spec) == "*" {
		return nil, nil
	}
	set, rest, err := parseFieldList(spec)
	if err != nil {
		return nil, err
	}
	if rest != "" {
		return nil, fmt.Errorf("unexpected %q in fields", rest)
	}
	return set, nil
}

// parseFieldList parses comma-separated fields up to a closing parenthesis
// or the end, returning what is left.
func parseFieldList(s string) (fieldSet, string, error) {
	set := fieldSet{}
	for {
		name, rest, err := fieldName(s)
		if err != nil {
			return nil, rest, err
		}
		s = rest
		var inner fieldSet
		switch {
		case strings.HasPrefix(s, "("):
			if inner, s, err = parseFieldList(s[1:]); err != nil {
				return nil, s, err
			}
			if !strings.HasPrefix(s, ")") {
				return nil, s, fmt.Errorf("unclosed ( in fields")
			}
			s = s[1:]
		case strings.HasPrefix(s, "/"):
			if inner, s, err = parseFieldPath(s[1:]); err != nil {
				return nil, s, err
			}
		}
		if earlier, seen := set[name]; seen {
			set[name] = union(earlier, inner)
		} else {
			set[name] = inner
		}
		if !strings.HasPrefix(s, ",") {
			return set, s, nil
		}
		s = s[1:]
	}
}

// parseFieldPath parses the rest of a slash path such as owners/emailAddress.
func parseFieldPath(s string) (fieldSet, string, error) {
	name, s, err := fieldName(s)
	if err != nil {
		return nil, s, err
	}
	var inner fieldSet
	if strings.HasPrefix(s, "/") {
		if inner, s, err = parseFieldPath(s[1:]); err != nil {
			return nil, s, err
		}
	}
	return fieldSet{name: inner}, s, nil
}

func fieldName(s string) (string, string, error) {
	s = strings.TrimLeft(s, " ")
	end := strings.IndexAny(s, ",()/")
	if end < 0 {
		end = len(s)
	}
	name := strings.TrimSpace(s[:end])
	if name == "" {
		return "", s, fmt.Errorf("empty field name in fields")
	}
	return name, s[end:], nil
}

// union combines two selections of one key; selecting a key whole wins.
func union(a, b fieldSet) fieldSet {
	if a == nil || b == nil {
		return nil
	}
	for key, inner := range b {
		if earlier, seen := a[key]; seen {
			a[key] = union(earlier, inner)
		} else {
			a[key] = inner
		}
	}
	return a
}

// selectFields keeps only the selected parts of a JSON-shaped value.
func selectFields(value any, set fieldSet) any {
	if set == nil {
		return value
	}
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(set))
		for key, inner := range set {
			if field, ok := v[key]; ok {
				out[key] = selectFields(field, inner)
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, element := range v {
			out[i] = selectFields(element, set)
		}
		return out
	default:
		return value
	}
}
