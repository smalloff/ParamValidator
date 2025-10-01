// length_plugin.go
package plugins

import (
	"fmt"
	"strings"
)

const maxLengthValue = 1000000

type LengthPlugin struct {
	name string
}

func NewLengthPlugin() *LengthPlugin {
	return &LengthPlugin{name: "len"}
}

func (lp *LengthPlugin) GetName() string {
	return lp.name
}

func (lp *LengthPlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	prefix := lp.name + ":"
	if len(constraintStr) < len(prefix) || !strings.HasPrefix(constraintStr, prefix) {
		return nil, fmt.Errorf("not for this plugin: length constraint must start with '%s:'", lp.name)
	}

	rest := strings.TrimSpace(constraintStr[len(prefix):])
	if rest == "" {
		return nil, fmt.Errorf("empty length expression")
	}

	validator, err := lp.parseConstraint(rest)
	if err != nil {
		return nil, err
	}

	return validator, nil
}

func (lp *LengthPlugin) parseConstraint(rest string) (func(string) bool, error) {
	if dotPos := lp.findDoubleDot(rest); dotPos != -1 {
		return lp.parseRange(rest, dotPos)
	}
	return lp.parseOperatorOrNumber(rest)
}

func (lp *LengthPlugin) findDoubleDot(s string) int {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '.' && s[i+1] == '.' {
			return i
		}
	}
	return -1
}

func (lp *LengthPlugin) parseRange(s string, dotPos int) (func(string) bool, error) {
	minStr := strings.TrimSpace(s[:dotPos])
	maxStr := strings.TrimSpace(s[dotPos+2:])

	if minStr == "" || maxStr == "" {
		return nil, fmt.Errorf("invalid range format: '%s'", s)
	}

	min, minOk := parseNumber(minStr)
	max, maxOk := parseNumber(maxStr)

	if !minOk || !maxOk {
		return nil, fmt.Errorf("invalid range format: '%s'", s)
	}

	if min > maxLengthValue || max > maxLengthValue {
		return nil, fmt.Errorf("length value too large: max allowed is %d", maxLengthValue)
	}

	if min < 0 || max < 0 {
		return nil, fmt.Errorf("length cannot be negative: %d..%d", min, max)
	}

	if min > max {
		return nil, fmt.Errorf("invalid range: %d..%d (min > max)", min, max)
	}

	return func(value string) bool {
		length := stringLength(value)
		return length >= min && length <= max
	}, nil
}

func (lp *LengthPlugin) parseOperatorOrNumber(expr string) (func(string) bool, error) {
	if len(expr) == 0 {
		return nil, fmt.Errorf("empty expression")
	}

	operator, numStart := lp.parseOperator(expr)

	if numStart > 0 {
		if numStart >= len(expr) {
			return nil, fmt.Errorf("missing length value")
		}

		numStr := strings.TrimSpace(expr[numStart:])
		if numStr == "" {
			return nil, fmt.Errorf("missing length value")
		}

		length, ok := parseNumber(numStr)
		if !ok {
			return nil, fmt.Errorf("invalid length value: '%s'", numStr)
		}

		if length > maxLengthValue {
			return nil, fmt.Errorf("length value too large: %d (max allowed is %d)", length, maxLengthValue)
		}

		if length < 0 {
			return nil, fmt.Errorf("length cannot be negative: %d", length)
		}

		return lp.createValidator(operator, length), nil
	} else {
		numStr := strings.TrimSpace(expr)
		length, ok := parseNumber(numStr)
		if !ok {
			return nil, fmt.Errorf("invalid length value: '%s'", numStr)
		}

		if length > maxLengthValue {
			return nil, fmt.Errorf("length value too large: %d (max allowed is %d)", length, maxLengthValue)
		}

		if length < 0 {
			return nil, fmt.Errorf("length cannot be negative: %d", length)
		}

		return lp.createValidator("=", length), nil
	}
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

	return "", 0
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

func (lp *LengthPlugin) Close() error {
	return nil
}
