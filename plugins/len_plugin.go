// length_plugin_optimized.go
package plugins

import (
	"fmt"
)

const maxLengthValue = 1000000 // Максимальное значение для длины

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
	min, minOk := parseNumber(s[:dotPos])
	max, maxOk := parseNumber(s[dotPos+2:])

	if !minOk || !maxOk {
		return nil, fmt.Errorf("invalid range format: '%s'", s)
	}

	// Проверяем ограничения на числа
	if min > maxLengthValue || max > maxLengthValue {
		return nil, fmt.Errorf("length value too large: max allowed is %d", maxLengthValue)
	}

	if min < 0 || max < 0 || min > max {
		return nil, fmt.Errorf("invalid range: %d..%d", min, max)
	}

	return func(value string) bool {
		length := stringLength(value)
		return length >= min && length <= max
	}, nil
}

func (lp *LengthPlugin) parseOperatorNoAlloc(expr string) (func(string) bool, error) {
	if len(expr) == 0 {
		return nil, fmt.Errorf("empty expression")
	}

	operator, numStart := lp.parseOperator(expr)
	if numStart >= len(expr) {
		return nil, fmt.Errorf("missing length value")
	}

	numStr := expr[numStart:]
	length, ok := parseNumber(numStr)
	if !ok {
		return nil, fmt.Errorf("invalid length value: '%s'", numStr)
	}

	// Проверяем ограничения на числа
	if length > maxLengthValue {
		return nil, fmt.Errorf("length value too large: %d (max allowed is %d)", length, maxLengthValue)
	}

	if length < 0 {
		return nil, fmt.Errorf("length cannot be negative: %d", length)
	}

	return lp.createValidator(operator, length), nil
}

func (lp *LengthPlugin) parseOperator(expr string) (string, int) {
	if len(expr) >= 2 {
		if expr[0] == '>' && expr[1] == '=' {
			return ">=", 2
		}
		if expr[0] == '<' && expr[1] == '=' {
			return "<=", 2
		}
		if expr[0] == '!' && expr[1] == '=' {
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

func (lp *LengthPlugin) createValidator(operator string, length int) func(string) bool {
	switch operator {
	case "=":
		return func(value string) bool { return stringLength(value) == length }
	case ">":
		return func(value string) bool { return stringLength(value) > length }
	case ">=":
		return func(value string) bool { return stringLength(value) >= length }
	case "<":
		return func(value string) bool { return stringLength(value) < length }
	case "<=":
		return func(value string) bool { return stringLength(value) <= length }
	case "!=":
		return func(value string) bool { return stringLength(value) != length }
	default:
		return func(value string) bool { return false }
	}
}

// Закрытие ресурсов
func (lp *LengthPlugin) Close() error {
	return nil
}
