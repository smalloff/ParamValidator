// pattern_plugin.go
package plugins

import (
	"fmt"
	"strings"
)

const maxPatternLength = 1000 // Максимальная длина паттерна

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

func (pp *PatternPlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	if len(constraintStr) < 4 || constraintStr[0:3] != "in:" {
		return nil, fmt.Errorf("pattern constraint must start with 'in:'")
	}

	pattern := constraintStr[3:]
	if pattern == "" {
		return nil, fmt.Errorf("empty pattern")
	}

	if len(pattern) > maxPatternLength {
		return nil, fmt.Errorf("pattern too long: %d characters", len(pattern))
	}

	// Проверяем валидность UTF-8
	if !isValidUTF8(pattern) {
		return nil, fmt.Errorf("invalid UTF-8 in pattern")
	}

	// Проверяем наличие wildcard *
	hasWildcard := false
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '*' {
			hasWildcard = true
			break
		}
	}
	if !hasWildcard {
		return nil, fmt.Errorf("pattern must contain at least one wildcard '*'")
	}

	// Предварительно анализируем паттерн
	hasLeadingStar := pattern[0] == '*'
	hasTrailingStar := pattern[len(pattern)-1] == '*'

	// Если паттерн простой (один *), обрабатываем специально
	if hasLeadingStar && hasTrailingStar && len(pattern) == 2 {
		// Паттерн "**" - любая строка включая пустую
		return func(value string) bool {
			// Проверяем длину значения
			return len(value) <= maxPatternLength*10 // Разумное ограничение
		}, nil
	}

	if hasLeadingStar && !hasTrailingStar && strings.Count(pattern, "*") == 1 {
		// Паттерн "*suffix" - проверяем суффикс
		suffix := pattern[1:]
		return func(value string) bool {
			if len(value) > maxPatternLength*10 {
				return false
			}
			return strings.HasSuffix(value, suffix)
		}, nil
	}

	if !hasLeadingStar && hasTrailingStar && strings.Count(pattern, "*") == 1 {
		// Паттерн "prefix*" - проверяем префикс
		prefix := pattern[:len(pattern)-1]
		return func(value string) bool {
			if len(value) > maxPatternLength*10 {
				return false
			}
			return strings.HasPrefix(value, prefix)
		}, nil
	}

	// Для сложных паттернов используем strings.Split (1 аллокация)
	parts := strings.Split(pattern, "*")
	return pp.createValidator(parts), nil
}

func (pp *PatternPlugin) createValidator(parts []string) func(string) bool {
	return func(value string) bool {
		// Проверяем длину значения
		if len(value) > maxPatternLength*10 {
			return false
		}

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

// Закрытие ресурсов
func (pp *PatternPlugin) Close() error {
	return nil
}
