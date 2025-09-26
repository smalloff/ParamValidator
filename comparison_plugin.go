// comparison_plugin.go
package paramvalidator

import (
	"fmt"
	"regexp"
	"strconv"
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
	// Более "жадная" проверка: если строка начинается с < или >,
	// считаем что это может быть оператор сравнения
	if len(constraintStr) == 0 {
		return false
	}

	firstChar := constraintStr[0]
	return firstChar == '<' || firstChar == '>'
}

func (cp *ComparisonPlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	// Строгая валидация формата с помощью regex
	pattern := `^([<>]=?)(-?\d+)$`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(constraintStr)

	if len(matches) != 3 {
		// Даем подробное сообщение об ошибке для различных некорректных форматов
		if len(constraintStr) == 1 {
			return nil, fmt.Errorf("incomplete comparison operator '%s': missing number", constraintStr)
		}

		firstChar := constraintStr[0]
		secondChar := constraintStr[1]

		// Проверяем различные случаи ошибок
		if firstChar == '>' && secondChar == '>' {
			return nil, fmt.Errorf("invalid double operator '>>', use single '>'")
		}
		if firstChar == '<' && secondChar == '<' {
			return nil, fmt.Errorf("invalid double operator '<<', use single '<'")
		}
		if (firstChar == '>' && secondChar == '<') || (firstChar == '<' && secondChar == '>') {
			return nil, fmt.Errorf("invalid operator combination '%s', use either '>' or '<'", constraintStr[:2])
		}
		if secondChar == '=' && len(constraintStr) == 2 {
			return nil, fmt.Errorf("incomplete operator '%s': missing number after =", constraintStr)
		}

		// Общая ошибка для других случаев
		return nil, fmt.Errorf("invalid comparison format: '%s'. Expected formats: >N, <N, >=N, <=N where N is a number", constraintStr)
	}

	operator := matches[1]
	threshold, err := strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number in comparison: %s", matches[2])
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
