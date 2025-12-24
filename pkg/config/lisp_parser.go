// Package config provides configuration parsing using Lisp S-expressions
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// SExpr represents an S-expression (atom or list)
type SExpr interface {
	isSExpr()
	String() string
}

// Atom represents an atomic value (string, number, symbol, bool)
type Atom struct {
	Value    interface{}
	RawValue string
}

func (a *Atom) isSExpr() {}
func (a *Atom) String() string {
	return fmt.Sprintf("%v", a.Value)
}

// IsSymbol returns true if the atom is a symbol
func (a *Atom) IsSymbol() bool {
	_, ok := a.Value.(string)
	return ok && !strings.HasPrefix(a.RawValue, "\"")
}

// IsString returns true if the atom is a quoted string
func (a *Atom) IsString() bool {
	_, ok := a.Value.(string)
	return ok && strings.HasPrefix(a.RawValue, "\"")
}

// IsBool returns true if the atom is a boolean
func (a *Atom) IsBool() bool {
	_, ok := a.Value.(bool)
	return ok
}

// IsNumber returns true if the atom is a number
func (a *Atom) IsNumber() bool {
	switch a.Value.(type) {
	case int, int64, float64:
		return true
	}
	return false
}

// AsString returns the atom as a string
func (a *Atom) AsString() string {
	if s, ok := a.Value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", a.Value)
}

// AsInt returns the atom as an integer
func (a *Atom) AsInt() int {
	switch v := a.Value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		i, _ := strconv.Atoi(v)
		return i
	}
	return 0
}

// AsBool returns the atom as a boolean
func (a *Atom) AsBool() bool {
	if b, ok := a.Value.(bool); ok {
		return b
	}
	if s, ok := a.Value.(string); ok {
		return s == "true" || s == "t" || s == "yes"
	}
	return false
}

// List represents a list of S-expressions
type List struct {
	Items []SExpr
}

func (l *List) isSExpr() {}
func (l *List) String() string {
	parts := make([]string, len(l.Items))
	for i, item := range l.Items {
		parts[i] = item.String()
	}
	return "(" + strings.Join(parts, " ") + ")"
}

// Head returns the first element of the list (usually a symbol)
func (l *List) Head() *Atom {
	if len(l.Items) == 0 {
		return nil
	}
	if atom, ok := l.Items[0].(*Atom); ok {
		return atom
	}
	return nil
}

// Tail returns all elements after the first
func (l *List) Tail() []SExpr {
	if len(l.Items) <= 1 {
		return nil
	}
	return l.Items[1:]
}

// Get returns a named property from the list
// e.g., for (foo (bar 1) (baz 2)), Get("bar") returns the value 1
func (l *List) Get(name string) SExpr {
	// Search in all items (for nested lists returned by GetList)
	for _, item := range l.Items {
		if list, ok := item.(*List); ok {
			if head := list.Head(); head != nil && head.AsString() == name {
				if len(list.Items) == 2 {
					return list.Items[1]
				}
				// Return the tail as a new list for multiple values
				return &List{Items: list.Tail()}
			}
		}
	}
	return nil
}

// GetString returns a string property
func (l *List) GetString(name string) string {
	if v := l.Get(name); v != nil {
		if atom, ok := v.(*Atom); ok {
			return atom.AsString()
		}
	}
	return ""
}

// GetInt returns an integer property
func (l *List) GetInt(name string) int {
	if v := l.Get(name); v != nil {
		if atom, ok := v.(*Atom); ok {
			return atom.AsInt()
		}
	}
	return 0
}

// GetBool returns a boolean property
func (l *List) GetBool(name string) bool {
	if v := l.Get(name); v != nil {
		if atom, ok := v.(*Atom); ok {
			return atom.AsBool()
		}
	}
	return false
}

// GetList returns a list property
func (l *List) GetList(name string) *List {
	if v := l.Get(name); v != nil {
		if list, ok := v.(*List); ok {
			return list
		}
	}
	return nil
}

// GetStringSlice returns a slice of strings from a list property
func (l *List) GetStringSlice(name string) []string {
	if v := l.Get(name); v != nil {
		switch val := v.(type) {
		case *Atom:
			return []string{val.AsString()}
		case *List:
			result := make([]string, 0, len(val.Items))
			for _, item := range val.Items {
				if atom, ok := item.(*Atom); ok {
					result = append(result, atom.AsString())
				}
			}
			return result
		}
	}
	return nil
}

// GetMap returns a map from list properties
func (l *List) GetMap(name string) map[string]string {
	result := make(map[string]string)
	if v := l.Get(name); v != nil {
		if list, ok := v.(*List); ok {
			for _, item := range list.Items {
				if pair, ok := item.(*List); ok {
					if head := pair.Head(); head != nil && len(pair.Items) >= 2 {
						if val, ok := pair.Items[1].(*Atom); ok {
							result[head.AsString()] = val.AsString()
						}
					}
				}
			}
		}
	}
	return result
}

// Parser for S-expressions
type LispParser struct {
	input  string
	pos    int
	length int
}

// NewLispParser creates a new parser
func NewLispParser(input string) *LispParser {
	return &LispParser{
		input:  input,
		pos:    0,
		length: len(input),
	}
}

// Parse parses the input and returns an S-expression
func (p *LispParser) Parse() (SExpr, error) {
	p.skipWhitespaceAndComments()
	if p.pos >= p.length {
		return nil, fmt.Errorf("unexpected end of input")
	}
	return p.parseExpr()
}

func (p *LispParser) parseExpr() (SExpr, error) {
	p.skipWhitespaceAndComments()
	if p.pos >= p.length {
		return nil, fmt.Errorf("unexpected end of input")
	}

	ch := p.input[p.pos]
	switch {
	case ch == '(':
		return p.parseList()
	case ch == '"':
		return p.parseString()
	case ch == '-' || unicode.IsDigit(rune(ch)):
		return p.parseNumberOrSymbol()
	default:
		return p.parseAtom()
	}
}

func (p *LispParser) parseList() (*List, error) {
	p.pos++ // skip '('
	list := &List{Items: []SExpr{}}

	for {
		p.skipWhitespaceAndComments()
		if p.pos >= p.length {
			return nil, fmt.Errorf("unexpected end of input in list")
		}
		if p.input[p.pos] == ')' {
			p.pos++ // skip ')'
			break
		}
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		list.Items = append(list.Items, expr)
	}

	return list, nil
}

func (p *LispParser) parseString() (*Atom, error) {
	p.pos++ // skip opening quote
	start := p.pos
	var result strings.Builder

	for p.pos < p.length {
		ch := p.input[p.pos]
		if ch == '\\' && p.pos+1 < p.length {
			// Handle escape sequences
			p.pos++
			switch p.input[p.pos] {
			case 'n':
				result.WriteByte('\n')
			case 't':
				result.WriteByte('\t')
			case 'r':
				result.WriteByte('\r')
			case '"':
				result.WriteByte('"')
			case '\\':
				result.WriteByte('\\')
			default:
				result.WriteByte(p.input[p.pos])
			}
			p.pos++
		} else if ch == '"' {
			p.pos++ // skip closing quote
			return &Atom{Value: result.String(), RawValue: "\"" + p.input[start:p.pos-1] + "\""}, nil
		} else {
			result.WriteByte(ch)
			p.pos++
		}
	}

	return nil, fmt.Errorf("unterminated string")
}

func (p *LispParser) parseNumberOrSymbol() (*Atom, error) {
	start := p.pos

	// Check for negative number
	if p.input[p.pos] == '-' {
		p.pos++
		if p.pos >= p.length || !unicode.IsDigit(rune(p.input[p.pos])) {
			p.pos = start
			return p.parseAtom()
		}
	}

	// Parse digits
	hasDecimal := false
	for p.pos < p.length {
		ch := p.input[p.pos]
		if ch == '.' && !hasDecimal {
			hasDecimal = true
			p.pos++
		} else if unicode.IsDigit(rune(ch)) {
			p.pos++
		} else {
			break
		}
	}

	raw := p.input[start:p.pos]

	// Check if it's actually a number or just starts with digits
	if p.pos < p.length && !isDelimiter(p.input[p.pos]) {
		// It's part of a symbol, continue parsing as atom
		p.pos = start
		return p.parseAtom()
	}

	if hasDecimal {
		val, _ := strconv.ParseFloat(raw, 64)
		return &Atom{Value: val, RawValue: raw}, nil
	}
	val, _ := strconv.ParseInt(raw, 10, 64)
	return &Atom{Value: int(val), RawValue: raw}, nil
}

func (p *LispParser) parseAtom() (*Atom, error) {
	start := p.pos

	for p.pos < p.length && !isDelimiter(p.input[p.pos]) {
		p.pos++
	}

	if p.pos == start {
		return nil, fmt.Errorf("expected atom at position %d", p.pos)
	}

	raw := p.input[start:p.pos]

	// Check for special atoms
	switch raw {
	case "true", "t", "#t":
		return &Atom{Value: true, RawValue: raw}, nil
	case "false", "nil", "#f":
		return &Atom{Value: false, RawValue: raw}, nil
	}

	return &Atom{Value: raw, RawValue: raw}, nil
}

func (p *LispParser) skipWhitespaceAndComments() {
	for p.pos < p.length {
		ch := p.input[p.pos]
		if unicode.IsSpace(rune(ch)) {
			p.pos++
		} else if ch == ';' {
			// Skip comment until end of line
			for p.pos < p.length && p.input[p.pos] != '\n' {
				p.pos++
			}
		} else {
			break
		}
	}
}

func isDelimiter(ch byte) bool {
	return ch == '(' || ch == ')' || ch == '"' || ch == ';' || unicode.IsSpace(rune(ch))
}

// ParseFile parses a Lisp file and returns the root S-expression
func ParseLispFile(filePath string) (SExpr, error) {
	// Expand ~ to home directory
	if strings.HasPrefix(filePath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		filePath = home + filePath[1:]
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Expand environment variables in the content
	contentStr := expandEnvVars(string(content))

	parser := NewLispParser(contentStr)
	return parser.Parse()
}

// expandEnvVars expands ${VAR_NAME} patterns in the content
func expandEnvVars(content string) string {
	result := content
	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start
		varName := result[start+2 : end]
		varValue := os.Getenv(varName)
		result = result[:start] + varValue + result[end+1:]
	}
	return result
}
