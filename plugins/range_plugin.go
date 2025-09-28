package plugins

import (
	"fmt"
)

// RangePlugin плагин для диапазонов чисел: 1-10, 5..100, -50..50
type RangePlugin struct {
	name string
}

func NewRangePlugin() *RangePlugin {
	return &RangePlugin{name: "range"}
}

func (rp *RangePlugin) GetName() string {
	return rp.name
}

func (rp *RangePlugin) CanParse(constraintStr string) bool {
	if len(constraintStr) < 3 {
		return false
	}

	// Просто ищем любой из разделителей
	for i := 0; i < len(constraintStr); i++ {
		if constraintStr[i] == '-' || constraintStr[i] == '.' {
			return true
		}
	}

	return false
}

func (rp *RangePlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	// Находим разделитель за один проход
	sepPos := -1
	sepType := byte(0)

	for i := 1; i < len(constraintStr)-1; i++ {
		if constraintStr[i] == '.' && i < len(constraintStr)-1 && constraintStr[i+1] == '.' {
			sepPos = i
			sepType = '.'
			break
		}
		if constraintStr[i] == '-' && (constraintStr[i-1] >= '0' && constraintStr[i-1] <= '9') {
			sepPos = i
			sepType = '-'
			// continue, ищем ".." в приоритете
		}
	}

	if sepPos == -1 {
		return nil, fmt.Errorf("invalid range format: %s", constraintStr)
	}

	// Быстро извлекаем подстроки без триминга
	var minStr, maxStr string
	if sepType == '.' {
		minStr = constraintStr[:sepPos]
		maxStr = constraintStr[sepPos+2:]
	} else {
		minStr = constraintStr[:sepPos]
		maxStr = constraintStr[sepPos+1:]
	}

	// Парсим числа как в LengthPlugin
	min, minOk := fastAtoi(minStr)
	max, maxOk := fastAtoi(maxStr)

	if !minOk || !maxOk || min > max {
		return nil, fmt.Errorf("invalid range: %s", constraintStr)
	}

	return func(value string) bool {
		num, ok := fastAtoi(value)
		return ok && num >= min && num <= max
	}, nil
}

// fastAtoi как в LengthPlugin
func fastAtoi(s string) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}

	result := 0
	start := 0
	if s[0] == '-' {
		start = 1
	}

	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		result = result*10 + int(s[i]-'0')
	}

	if start == 1 {
		return -result, true
	}
	return result, true
}
