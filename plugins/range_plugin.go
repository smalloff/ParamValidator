// range_plugin.go
package plugins

import (
	"fmt"
)

const (
	maxRangeNumberLength = 10      // Максимальная длина числа (включая знак)
	maxRangeValue        = 1000000 // Максимальное значение для диапазона
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

	// Полная проверка валидности формата
	err := rp.parseRangeForCanParse(constraintStr)
	return err == nil
}

// Упрощенная версия для CanParse - только проверка синтаксиса
func (rp *RangePlugin) parseRangeForCanParse(constraintStr string) error {
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
		return fmt.Errorf("invalid range format")
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

	// Проверяем длину чисел
	if len(minStr) > maxRangeNumberLength || len(maxStr) > maxRangeNumberLength {
		return fmt.Errorf("number too long")
	}

	// Проверяем пустые значения
	if minStr == "" || maxStr == "" {
		return fmt.Errorf("empty min or max value")
	}

	// Парсим числа
	min, minOk := parseNumber(minStr)
	max, maxOk := parseNumber(maxStr)

	if !minOk || !maxOk {
		return fmt.Errorf("invalid numbers")
	}

	// Проверяем корректность диапазона
	if min > max {
		return fmt.Errorf("min greater than max")
	}

	// Проверяем ограничения на числа
	if min > maxRangeValue || max > maxRangeValue || min < -maxRangeValue || max < -maxRangeValue {
		return fmt.Errorf("range values out of range")
	}

	return nil
}

func (rp *RangePlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	if len(constraintStr) < 3 {
		return nil, fmt.Errorf("range too short")
	}

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

	// Проверяем длину чисел
	if len(minStr) > maxRangeNumberLength || len(maxStr) > maxRangeNumberLength {
		return nil, fmt.Errorf("number too long in range: %s", constraintStr)
	}

	// Парсим числа как в LengthPlugin
	min, minOk := parseNumber(minStr)
	max, maxOk := parseNumber(maxStr)

	if !minOk || !maxOk || min > max {
		return nil, fmt.Errorf("invalid range: %s", constraintStr)
	}

	// Проверяем ограничения на числа
	if min > maxRangeValue || max > maxRangeValue || min < -maxRangeValue || max < -maxRangeValue {
		return nil, fmt.Errorf("range values out of range: %d..%d (allowed: -%d to %d)",
			min, max, maxRangeValue, maxRangeValue)
	}

	return func(value string) bool {
		// Проверяем длину входного значения
		if len(value) > maxRangeNumberLength {
			return false
		}
		num, ok := parseNumber(value)
		if !ok {
			return false
		}
		// Проверяем ограничения на числа
		if num > maxRangeValue || num < -maxRangeValue {
			return false
		}
		return num >= min && num <= max
	}, nil
}
