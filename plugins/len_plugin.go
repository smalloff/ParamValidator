// length_plugin_zero_alloc.go
package plugins

import (
	"fmt"
	"unicode/utf8"
)

type LengthPlugin struct {
	name string
}

func NewLengthPlugin() *LengthPlugin {
	return &LengthPlugin{name: "length"}
}

func (lp *LengthPlugin) GetName() string {
	return lp.name
}

func (lp *LengthPlugin) CanParse(constraintStr string) bool {
	return len(constraintStr) >= 4 &&
		constraintStr[0] == 'l' &&
		constraintStr[1] == 'e' &&
		constraintStr[2] == 'n' &&
		(lp.isValidNextChar(constraintStr[3]))
}

func (lp *LengthPlugin) isValidNextChar(c byte) bool {
	return c == '>' || c == '<' || c == '=' || c == '!' ||
		(c >= '0' && c <= '9') || c == '.'
}

func (lp *LengthPlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	if len(constraintStr) < 4 || constraintStr[0:3] != "len" {
		return nil, fmt.Errorf("length constraint must start with 'len'")
	}

	rest := constraintStr[3:]

	if dotPos := lp.findDoubleDot(rest); dotPos != -1 {
		return lp.parseRangeNoAlloc(rest, dotPos)
	}

	return lp.parseOperatorNoAlloc(rest)
}

func (lp *LengthPlugin) findDoubleDot(s string) int {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '.' && s[i+1] == '.' {
			return i
		}
	}
	return -1
}

// parseRangeNoAlloc - полностью без аллокаций
func (lp *LengthPlugin) parseRangeNoAlloc(s string, dotPos int) (func(string) bool, error) {
	min, minOk := lp.atoiNoAlloc(s[:dotPos])
	max, maxOk := lp.atoiNoAlloc(s[dotPos+2:])

	if !minOk || !maxOk {
		return nil, fmt.Errorf("invalid range format: '%s'", s)
	}

	if min < 0 || max < 0 || min > max {
		return nil, fmt.Errorf("invalid range: %d..%d", min, max)
	}

	return func(value string) bool {
		return utf8.RuneCountInString(value) >= min &&
			utf8.RuneCountInString(value) <= max
	}, nil
}

// parseOperatorNoAlloc - полностью без аллокаций
func (lp *LengthPlugin) parseOperatorNoAlloc(expr string) (func(string) bool, error) {
	if len(expr) == 0 {
		return nil, fmt.Errorf("empty expression")
	}

	operator, numStart := lp.parseOperator(expr)
	if numStart >= len(expr) {
		return nil, fmt.Errorf("missing length value")
	}

	numStr := expr[numStart:]
	length, ok := lp.atoiNoAlloc(numStr)
	if !ok {
		return nil, fmt.Errorf("invalid length value: '%s'", numStr)
	}

	if length < 0 {
		return nil, fmt.Errorf("length cannot be negative: %d", length)
	}

	return lp.createValidator(operator, length), nil
}

func (lp *LengthPlugin) parseOperator(expr string) (string, int) {
	if len(expr) >= 2 {
		switch expr[0:2] {
		case ">=":
			return ">=", 2
		case "<=":
			return "<=", 2
		case "!=":
			return "!=", 2
		}
	}

	if len(expr) >= 1 {
		switch expr[0] {
		case '>':
			return ">", 1
		case '<':
			return "<", 1
		case '=':
			return "=", 1
		}
	}

	return "=", 0
}

// atoiNoAlloc - преобразование строки в int без аллокаций
func (lp *LengthPlugin) atoiNoAlloc(s string) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}

	result := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		result = result*10 + int(ch-'0')
	}
	return result, true
}

func (lp *LengthPlugin) createValidator(operator string, length int) func(string) bool {
	switch operator {
	case "=":
		return func(value string) bool { return utf8.RuneCountInString(value) == length }
	case ">":
		return func(value string) bool { return utf8.RuneCountInString(value) > length }
	case ">=":
		return func(value string) bool { return utf8.RuneCountInString(value) >= length }
	case "<":
		return func(value string) bool { return utf8.RuneCountInString(value) < length }
	case "<=":
		return func(value string) bool { return utf8.RuneCountInString(value) <= length }
	case "!=":
		return func(value string) bool { return utf8.RuneCountInString(value) != length }
	default:
		return func(value string) bool { return false }
	}
}
