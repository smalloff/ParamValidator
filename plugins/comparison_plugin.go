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

func (cp *ComparisonPlugin) CanParse(constraintStr string) bool {
	if len(constraintStr) == 0 {
		return false
	}

	firstChar := constraintStr[0]
	return firstChar == '<' || firstChar == '>'
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

	// Быстрая проверка различных случаев ошибок
	if len(str) == 1 {
		return "", 0, fmt.Errorf("incomplete comparison operator '%s': missing number", str)
	}

	// Проверяем недопустимые комбинации операторов
	if strings.HasPrefix(str, ">>") {
		return "", 0, fmt.Errorf("invalid double operator '>>', use single '>'")
	}
	if strings.HasPrefix(str, "<<") {
		return "", 0, fmt.Errorf("invalid double operator '<<', use single '<'")
	}
	if strings.HasPrefix(str, "><") || strings.HasPrefix(str, "<>") {
		return "", 0, fmt.Errorf("invalid operator combination '%s', use either '>' or '<'", str[:2])
	}

	// Операторы без числа
	if str == ">" || str == "<" || str == ">=" || str == "<=" {
		return "", 0, fmt.Errorf("incomplete comparison operator '%s': missing number", str)
	}

	// Парсим оператор и число за один проход
	var operator string
	var numStart int

	if strings.HasPrefix(str, ">=") {
		operator = ">="
		numStart = 2
	} else if strings.HasPrefix(str, "<=") {
		operator = "<="
		numStart = 2
	} else if strings.HasPrefix(str, ">") {
		operator = ">"
		numStart = 1
	} else if strings.HasPrefix(str, "<") {
		operator = "<"
		numStart = 1
	} else {
		return "", 0, fmt.Errorf("invalid comparison format: '%s'. Expected formats: >N, <N, >=N, <=N where N is a number", str)
	}

	// Извлекаем числовую часть
	if numStart >= len(str) {
		return "", 0, fmt.Errorf("missing number after operator '%s'", operator)
	}

	numStr := str[numStart:]

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
