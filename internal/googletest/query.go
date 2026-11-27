package googletest

import (
	"fmt"
	"strings"
)

// predicate reports whether a file matches a query, for the account asking.
type predicate func(e *Emulator, f *file, account string) bool

// parseQuery parses the subset of Drive's query language eventlog uses:
// terms joined by "and", each one of
//
//	name = 'x'                 mimeType = 'x'    mimeType != 'x'
//	'id' in parents            'email' in owners trashed = true|false
//	appProperties has { key='k' and value='v' }
//
// Anything else is refused, as an unsupported query would be by Drive.
func parseQuery(q string) (predicate, error) {
	tokens, err := tokenize(q)
	if err != nil {
		return nil, err
	}
	p := &queryParser{tokens: tokens}
	var terms []predicate
	if len(tokens) > 0 {
		for {
			term, err := p.term()
			if err != nil {
				return nil, err
			}
			terms = append(terms, term)
			if p.done() {
				break
			}
			if !p.accept("and") {
				return nil, fmt.Errorf("expected and at %q", p.peek())
			}
		}
	}
	return func(e *Emulator, f *file, account string) bool {
		for _, term := range terms {
			if !term(e, f, account) {
				return false
			}
		}
		return true
	}, nil
}

type queryParser struct {
	tokens []token
	at     int
}

type token struct {
	text   string
	quoted bool
}

func (p *queryParser) done() bool { return p.at >= len(p.tokens) }

func (p *queryParser) peek() string {
	if p.done() {
		return ""
	}
	return p.tokens[p.at].text
}

func (p *queryParser) accept(word string) bool {
	if !p.done() && !p.tokens[p.at].quoted && p.tokens[p.at].text == word {
		p.at++
		return true
	}
	return false
}

func (p *queryParser) expect(word string) error {
	if !p.accept(word) {
		return fmt.Errorf("expected %q at %q", word, p.peek())
	}
	return nil
}

func (p *queryParser) literal() (string, error) {
	if p.done() || !p.tokens[p.at].quoted {
		return "", fmt.Errorf("expected a quoted value at %q", p.peek())
	}
	p.at++
	return p.tokens[p.at-1].text, nil
}

func (p *queryParser) term() (predicate, error) {
	if !p.done() && p.tokens[p.at].quoted {
		value, _ := p.literal()
		if err := p.expect("in"); err != nil {
			return nil, err
		}
		switch {
		case p.accept("parents"):
			return func(e *Emulator, f *file, account string) bool {
				id := value
				if id == "root" {
					id = e.state.Accounts[account].RootID
				}
				for _, parent := range f.Parents {
					if parent == id {
						return true
					}
				}
				return false
			}, nil
		case p.accept("owners"):
			return func(e *Emulator, f *file, _ string) bool { return sameEmail(f.Owner, value) }, nil
		default:
			return nil, fmt.Errorf("unsupported collection %q", p.peek())
		}
	}
	switch {
	case p.accept("name"):
		value, err := p.equals()
		return func(_ *Emulator, f *file, _ string) bool { return f.Name == value }, err
	case p.accept("mimeType"):
		negate := p.accept("!=")
		if !negate {
			if err := p.expect("="); err != nil {
				return nil, err
			}
		}
		value, err := p.literal()
		return func(_ *Emulator, f *file, _ string) bool { return (f.MimeType == value) != negate }, err
	case p.accept("trashed"):
		if err := p.expect("="); err != nil {
			return nil, err
		}
		switch {
		case p.accept("true"):
			return func(_ *Emulator, f *file, _ string) bool { return f.Trashed }, nil
		case p.accept("false"):
			return func(_ *Emulator, f *file, _ string) bool { return !f.Trashed }, nil
		default:
			return nil, fmt.Errorf("trashed takes true or false, not %q", p.peek())
		}
	case p.accept("appProperties"):
		if err := p.expect("has"); err != nil {
			return nil, err
		}
		if err := p.expect("{"); err != nil {
			return nil, err
		}
		if err := p.expect("key"); err != nil {
			return nil, err
		}
		key, err := p.equals()
		if err != nil {
			return nil, err
		}
		if err := p.expect("and"); err != nil {
			return nil, err
		}
		if err := p.expect("value"); err != nil {
			return nil, err
		}
		value, err := p.equals()
		if err != nil {
			return nil, err
		}
		if err := p.expect("}"); err != nil {
			return nil, err
		}
		return func(_ *Emulator, f *file, _ string) bool {
			stored, ok := f.AppProperties[key]
			return ok && stored == value
		}, nil
	default:
		return nil, fmt.Errorf("unsupported query term at %q", p.peek())
	}
}

func (p *queryParser) equals() (string, error) {
	if err := p.expect("="); err != nil {
		return "", err
	}
	return p.literal()
}

// tokenize splits a query into words, operators, braces, and quoted values,
// where a backslash escapes a quote or backslash inside quotes.
func tokenize(q string) ([]token, error) {
	var tokens []token
	for i := 0; i < len(q); {
		switch ch := q[i]; {
		case ch == ' ' || ch == '\t' || ch == '\n':
			i++
		case ch == '\'':
			var b strings.Builder
			i++
			for {
				if i >= len(q) {
					return nil, fmt.Errorf("unterminated quoted value")
				}
				if q[i] == '\\' && i+1 < len(q) {
					b.WriteByte(q[i+1])
					i += 2
					continue
				}
				if q[i] == '\'' {
					i++
					break
				}
				b.WriteByte(q[i])
				i++
			}
			tokens = append(tokens, token{text: b.String(), quoted: true})
		case ch == '{' || ch == '}' || ch == '=':
			tokens = append(tokens, token{text: string(ch)})
			i++
		case ch == '!' && i+1 < len(q) && q[i+1] == '=':
			tokens = append(tokens, token{text: "!="})
			i += 2
		default:
			start := i
			for i < len(q) && !strings.ContainsRune(" \t\n'{}=!", rune(q[i])) {
				i++
			}
			if start == i {
				return nil, fmt.Errorf("unexpected %q", q[i])
			}
			tokens = append(tokens, token{text: q[start:i]})
		}
	}
	return tokens, nil
}
