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
	return &ComparisonPlugin{name: "comparison"}
}

func (cp *ComparisonPlugin) GetName() string {
	return cp.name
}

// parseOperatorAndNumber извлекает оператор и числовую часть
func (cp *ComparisonPlugin) parseOperatorAndNumber(constraintStr string) (string, string, bool) {
	str := strings.TrimSpace(constraintStr)

	if len(str) < 2 {
		return "", "", false
	}

	// Определяем оператор
	var operator string
	var numStart int

	if len(str) >= 2 && str[1] == '=' {
		operator = str[:2]
		numStart = 2
	} else {
		operator = str[:1]
		numStart = 1
	}

	// Проверяем что есть содержимое после оператора
	if numStart >= len(str) {
		return "", "", false
	}

	numStr := str[numStart:]
	return operator, numStr, true
}

func (cp *ComparisonPlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	lenConstraintStr := len(constraintStr)
	if lenConstraintStr == 0 {
		return nil, fmt.Errorf("empty constraint")
	}

	// Единый парсинг - сразу получаем оператор и число
	operator, threshold, err := cp.parseComparisonOptimized(constraintStr)
	if err != nil {
		return nil, err
	}

	// Проверяем ограничения на числа
	if threshold > maxComparisonValue || threshold < -maxComparisonValue {
		return nil, fmt.Errorf("comparison value out of range: %d (allowed: -%d to %d)",
			threshold, maxComparisonValue, maxComparisonValue)
	}

	return cp.createValidator(operator, threshold), nil
}

// parseComparisonOptimized парсит оператор и число за один проход
func (cp *ComparisonPlugin) parseComparisonOptimized(constraintStr string) (string, int, error) {
	str := strings.TrimSpace(constraintStr)

	operator, numStr, isValid := cp.parseOperatorAndNumber(str)
	if !isValid {
		return "", 0, fmt.Errorf("invalid comparison format: '%s'. Expected formats: >N, <N, >=N, <=N where N is a number", str)
	}

	// Проверяем валидность оператора
	if operator != ">" && operator != ">=" && operator != "<" && operator != "<=" {
		return "", 0, fmt.Errorf("invalid comparison operator: '%s'", operator)
	}

	// Быстрый парсинг числа без аллокаций
	threshold, ok := parseNumber(numStr)
	if !ok {
		return "", 0, fmt.Errorf("invalid number in comparison: '%s'", numStr)
	}

	return operator, threshold, nil
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
