// range_plugin.go
package plugins

import (
	"fmt"
	"strings"
)

const (
	maxRangeNumberLength = 10      // Максимальная длина числа (включая знак)
	maxRangeValue        = 1000000 // Максимальное значение для диапазона
)

// RangePlugin плагин для диапазонов чисел: range1-10, range5..100, range-50..50
type RangePlugin struct {
	name string
}

func NewRangePlugin() *RangePlugin {
	return &RangePlugin{name: "range"}
}

func (rp *RangePlugin) GetName() string {
	return rp.name
}

func (rp *RangePlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	prefix := rp.name + ":"
	if len(constraintStr) < len(prefix) || !strings.HasPrefix(constraintStr, prefix) {
		return nil, fmt.Errorf("not for this plugin: range constraint must start with '%s:'", rp.name)
	}

	rest := strings.TrimSpace(constraintStr[6:])
	if len(rest) < 3 {
		return nil, fmt.Errorf("not for this plugin: range too short")
	}

	// Находим разделитель за один проход
	sepPos := -1
	sepType := byte(0)

	for i := 1; i < len(rest)-1; i++ {
		if rest[i] == '.' && i < len(rest)-1 && rest[i+1] == '.' {
			sepPos = i
			sepType = '.'
			break
		}
		if rest[i] == '-' && (rest[i-1] >= '0' && rest[i-1] <= '9') {
			sepPos = i
			sepType = '-'
			// continue, ищем ".." в приоритете
		}
	}

	if sepPos == -1 {
		return nil, fmt.Errorf("invalid range format: %s", constraintStr)
	}

	// Быстро извлекаем подстроки
	var minStr, maxStr string
	if sepType == '.' {
		minStr = rest[:sepPos]
		maxStr = rest[sepPos+2:]
	} else {
		minStr = rest[:sepPos]
		maxStr = rest[sepPos+1:]
	}

	// Проверяем пустые значения
	if minStr == "" || maxStr == "" {
		return nil, fmt.Errorf("invalid range format: %s", constraintStr)
	}

	// Проверяем длину чисел
	if len(minStr) > maxRangeNumberLength || len(maxStr) > maxRangeNumberLength {
		return nil, fmt.Errorf("number too long in range: %s", constraintStr)
	}

	// Парсим числа
	min, minOk := parseNumber(minStr)
	max, maxOk := parseNumber(maxStr)

	if !minOk || !maxOk {
		return nil, fmt.Errorf("invalid range: %s", constraintStr)
	}

	// Проверяем корректность диапазона
	if min > max {
		return nil, fmt.Errorf("invalid range: %d..%d (min > max)", min, max)
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

// Закрытие ресурсов
func (rp *RangePlugin) Close() error {
	return nil
}
