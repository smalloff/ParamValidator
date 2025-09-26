package paramvalidator

import (
	"fmt"
	"strconv"
	"strings"
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

// range_plugin.go

func (rp *RangePlugin) CanParse(constraintStr string) bool {
	if constraintStr == "" {
		return false
	}

	// Быстрая проверка на наличие запрещенных символов
	if strings.Contains(constraintStr, ",") {
		return false
	}

	// Проверяем разделители
	hasDots := strings.Contains(constraintStr, "..")
	hasDash := strings.Contains(constraintStr, "-")

	if !hasDots && !hasDash {
		return false
	}

	// Используем индексы вместо Split чтобы избежать аллокаций
	var minStr, maxStr string

	if hasDots {
		dotIndex := strings.Index(constraintStr, "..")
		if dotIndex == -1 || dotIndex == 0 || dotIndex == len(constraintStr)-2 {
			return false
		}
		minStr = strings.TrimSpace(constraintStr[:dotIndex])
		maxStr = strings.TrimSpace(constraintStr[dotIndex+2:])
	} else {
		dashIndex := strings.Index(constraintStr, "-")
		if dashIndex == -1 || dashIndex == 0 || dashIndex == len(constraintStr)-1 {
			return false
		}
		minStr = strings.TrimSpace(constraintStr[:dashIndex])
		maxStr = strings.TrimSpace(constraintStr[dashIndex+1:])
	}

	return rp.looksLikeNumber(minStr) && rp.looksLikeNumber(maxStr)
}

// looksLikeNumber можно сделать еще быстрее
func (rp *RangePlugin) looksLikeNumber(s string) bool {
	if s == "" {
		return false
	}

	start := 0
	if s[0] == '-' {
		if len(s) == 1 {
			return false
		}
		start = 1
	}

	// Быстрая проверка без range
	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

func (rp *RangePlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	// Определяем разделитель и разбиваем
	var parts []string
	if strings.Contains(constraintStr, "..") {
		parts = strings.Split(constraintStr, "..")
	} else {
		parts = strings.Split(constraintStr, "-")
	}

	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range format: %s, expected min-max or min..max", constraintStr)
	}

	minStr := strings.TrimSpace(parts[0])
	maxStr := strings.TrimSpace(parts[1])

	if minStr == "" || maxStr == "" {
		return nil, fmt.Errorf("empty min or max value in range: %s", constraintStr)
	}

	min, err := strconv.ParseInt(minStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid min value in range: %s", minStr)
	}

	max, err := strconv.ParseInt(maxStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid max value in range: %s", maxStr)
	}

	if min > max {
		return nil, fmt.Errorf("min value cannot be greater than max value: %d > %d", min, max)
	}

	// Создаем функцию валидации
	return func(value string) bool {
		num, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return false
		}
		return num >= min && num <= max
	}, nil
}
