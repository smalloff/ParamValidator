package plugins

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	maxComparisonNumberLength = 10      // Максимальная длина числа (включая знак)
	maxComparisonValue        = 1000000 // Максимальное значение для сравнения
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

	// Быстрая проверка с помощью строкового поиска
	operator, numStr, err := cp.parseComparison(constraintStr)
	if err != nil {
		return nil, err
	}

	// Проверяем длину числа
	if len(numStr) > maxComparisonNumberLength {
		return nil, fmt.Errorf("number too long in comparison: %s", numStr)
	}

	threshold, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number in comparison: %s", numStr)
	}

	// Проверяем ограничения на числа
	if threshold > maxComparisonValue || threshold < -maxComparisonValue {
		return nil, fmt.Errorf("comparison value out of range: %d (allowed: -%d to %d)",
			threshold, maxComparisonValue, maxComparisonValue)
	}

	return func(value string) bool {
		// Проверяем длину входного значения
		if len(value) > maxComparisonNumberLength {
			return false
		}
		num, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return false
		}

		// Проверяем ограничения на числа
		if num > maxComparisonValue || num < -maxComparisonValue {
			return false
		}

		switch operator {
		case ">":
			return num > threshold
		case ">=":
			return num >= threshold
		case "<":
			return num < threshold
		case "<=":
			return num <= threshold
		default:
			return false
		}
	}, nil
}

// parseComparison парсит оператор сравнения с помощью быстрого строкового поиска
func (cp *ComparisonPlugin) parseComparison(constraintStr string) (string, string, error) {
	str := strings.TrimSpace(constraintStr)

	// Проверяем различные случаи ошибок
	if len(str) == 1 {
		return "", "", fmt.Errorf("incomplete comparison operator '%s': missing number", str)
	}

	if strings.HasPrefix(str, ">>") {
		return "", "", fmt.Errorf("invalid double operator '>>', use single '>'")
	}
	if strings.HasPrefix(str, "<<") {
		return "", "", fmt.Errorf("invalid double operator '<<', use single '<'")
	}
	if strings.HasPrefix(str, "><") || strings.HasPrefix(str, "<>") {
		return "", "", fmt.Errorf("invalid operator combination '%s', use either '>' or '<'", str[:2])
	}

	// Операторы без числа
	if str == ">" || str == "<" || str == ">=" || str == "<=" {
		return "", "", fmt.Errorf("incomplete comparison operator '%s': missing number", str)
	}

	// Парсим оператор и число
	var operator string
	var numStr string

	if strings.HasPrefix(str, ">=") {
		operator = ">="
		numStr = str[2:]
	} else if strings.HasPrefix(str, "<=") {
		operator = "<="
		numStr = str[2:]
	} else if strings.HasPrefix(str, ">") {
		operator = ">"
		numStr = str[1:]
	} else if strings.HasPrefix(str, "<") {
		operator = "<"
		numStr = str[1:]
	} else {
		return "", "", fmt.Errorf("invalid comparison format: '%s'. Expected formats: >N, <N, >=N, <=N where N is a number", str)
	}

	// Проверяем что число валидно
	numStr = strings.TrimSpace(numStr)
	if numStr == "" {
		return "", "", fmt.Errorf("missing number after operator '%s'", operator)
	}

	// Проверяем длину числа
	if len(numStr) > maxComparisonNumberLength {
		return "", "", fmt.Errorf("number too long: '%s'", numStr)
	}

	// Проверяем что в строке только цифры и возможный знак минуса
	for i, ch := range numStr {
		if ch == '-' && i == 0 {
			continue // минус только в начале
		}
		if ch < '0' || ch > '9' {
			return "", "", fmt.Errorf("invalid character in number: '%c'", ch)
		}
	}

	return operator, numStr, nil
}
