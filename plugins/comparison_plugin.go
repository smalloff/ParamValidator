package plugins

import (
	"fmt"
	"strconv"
	"strings"
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
	if len(constraintStr) == 0 {
		return nil, fmt.Errorf("empty constraint")
	}

	// Быстрая проверка с помощью строкового поиска
	operator, numStr, err := cp.parseComparison(constraintStr)
	if err != nil {
		return nil, err
	}

	threshold, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number in comparison: %s", numStr)
	}

	return func(value string) bool {
		num, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
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
