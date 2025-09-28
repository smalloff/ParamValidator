package plugins

import (
	"fmt"
	"strings"
)

// PatternPlugin плагин для простых шаблонов с wildcard *
type PatternPlugin struct {
	name string
}

func NewPatternPlugin() *PatternPlugin {
	return &PatternPlugin{name: "pattern"}
}

func (pp *PatternPlugin) GetName() string {
	return pp.name
}

func (pp *PatternPlugin) CanParse(constraintStr string) bool {
	if constraintStr == "" {
		return false
	}

	// Быстрый поиск wildcard *
	for i := 0; i < len(constraintStr); i++ {
		if constraintStr[i] == '*' {
			return true
		}
	}

	return false
}

func (pp *PatternPlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	if constraintStr == "" {
		return nil, fmt.Errorf("empty pattern")
	}

	// Предварительно анализируем паттерн
	hasLeadingStar := constraintStr[0] == '*'
	hasTrailingStar := constraintStr[len(constraintStr)-1] == '*'

	// Если паттерн простой (один *), обрабатываем специально
	if hasLeadingStar && hasTrailingStar && len(constraintStr) == 2 {
		// Паттерн "**" - любая строка включая пустую
		return func(value string) bool { return true }, nil
	}

	if hasLeadingStar && !hasTrailingStar && strings.Count(constraintStr, "*") == 1 {
		// Паттерн "*suffix" - проверяем суффикс
		suffix := constraintStr[1:]
		return func(value string) bool {
			return strings.HasSuffix(value, suffix)
		}, nil
	}

	if !hasLeadingStar && hasTrailingStar && strings.Count(constraintStr, "*") == 1 {
		// Паттерн "prefix*" - проверяем префикс
		prefix := constraintStr[:len(constraintStr)-1]
		return func(value string) bool {
			return strings.HasPrefix(value, prefix)
		}, nil
	}

	// Для сложных паттернов используем strings.Split (1 аллокация)
	parts := strings.Split(constraintStr, "*")
	return pp.createValidator(parts), nil
}

func (pp *PatternPlugin) createValidator(parts []string) func(string) bool {
	return func(value string) bool {
		// Специальный случай: только wildcard "*" - совпадает с любой строкой
		if len(parts) == 2 && parts[0] == "" && parts[1] == "" {
			return true
		}

		// Проверяем паттерн часть за частью
		start := 0
		for i, part := range parts {
			if part == "" {
				continue
			}

			if i == 0 {
				if !strings.HasPrefix(value, part) {
					return false
				}
				start = len(part)
			} else if i == len(parts)-1 {
				if !strings.HasSuffix(value[start:], part) {
					return false
				}
			} else {
				pos := strings.Index(value[start:], part)
				if pos == -1 {
					return false
				}
				start += pos + len(part)
			}
		}
		return true
	}
}
