// comparison_plugin_optimized.go
package plugins

import (
	"fmt"
	"strings"
)

const (
	maxComparisonValue = 1000000 // Максимальное значение для сравнения
)

// ComparisonPlugin плагин для операторов сравнения: >5, <10, >=100, <=50
type ComparisonPlugin struct {
	name string
}

func NewComparisonPlugin() *ComparisonPlugin {
	return &ComparisonPlugin{name: "cmp"}
}

func (cp *ComparisonPlugin) GetName() string {
	return cp.name
}

func (cp *ComparisonPlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	lenConstraintStr := len(constraintStr)
	if lenConstraintStr == 0 {
		return nil, fmt.Errorf("not for this plugin: empty constraint")
	}

	// Проверяем формат используя имя плагина
	prefix := cp.name + ":"
	if len(constraintStr) < len(prefix) || !strings.HasPrefix(constraintStr, prefix) {
		return nil, fmt.Errorf("not for this plugin: comparison constraint must start with '%s:'", cp.name)
	}

	// Извлекаем часть после префикса
	rest := strings.TrimSpace(constraintStr[len(prefix):])
	if rest == "" {
		return nil, fmt.Errorf("empty comparison expression")
	}

	// Определяем оператор
	operator, numStart := cp.parseOperator(rest)
	if operator == "" {
		return nil, fmt.Errorf("invalid operator format: must start with >, <, >=, or <=")
	}

	// Проверяем что есть число после оператора
	if numStart >= len(rest) {
		return nil, fmt.Errorf("missing number value after operator")
	}

	numStr := strings.TrimSpace(rest[numStart:])
	if numStr == "" {
		return nil, fmt.Errorf("missing number value")
	}

	// Парсим число
	threshold, ok := parseNumber(numStr)
	if !ok {
		return nil, fmt.Errorf("invalid number format: '%s'", numStr)
	}

	// Проверяем ограничения на числа
	if threshold > maxComparisonValue || threshold < -maxComparisonValue {
		return nil, fmt.Errorf("value out of range: %d (allowed: -%d to %d)",
			threshold, maxComparisonValue, maxComparisonValue)
	}

	return cp.createValidator(operator, threshold), nil
}

// parseOperator определяет оператор и возвращает позицию начала числа
func (cp *ComparisonPlugin) parseOperator(str string) (string, int) {
	if len(str) >= 2 {
		switch str[0:2] {
		case ">=":
			return ">=", 2
		case "<=":
			return "<=", 2
		}
	}

	if len(str) >= 1 {
		switch str[0] {
		case '>':
			return ">", 1
		case '<':
			return "<", 1
		}
	}

	return "", 0
}

// createValidator создает функцию валидации
func (cp *ComparisonPlugin) createValidator(operator string, threshold int) func(string) bool {
	switch operator {
	case ">":
		return func(value string) bool {
			num, ok := parseNumber(value)
			return ok && num > threshold
		}
	case ">=":
		return func(value string) bool {
			num, ok := parseNumber(value)
			return ok && num >= threshold
		}
	case "<":
		return func(value string) bool {
			num, ok := parseNumber(value)
			return ok && num < threshold
		}
	case "<=":
		return func(value string) bool {
			num, ok := parseNumber(value)
			return ok && num <= threshold
		}
	default:
		return func(value string) bool { return false }
	}
}

// Закрытие ресурсов (если нужно)
func (cp *ComparisonPlugin) Close() error {
	return nil
}
