// regex_plugin.go
package paramvalidator

import (
	"fmt"
	"regexp"
)

// RegexPlugin плагин для регулярных выражений: /pattern/
type RegexPlugin struct {
	name string
}

func NewRegexPlugin() *RegexPlugin {
	return &RegexPlugin{name: "regex"}
}

func (rp *RegexPlugin) GetName() string {
	return rp.name
}

func (rp *RegexPlugin) CanParse(constraintStr string) bool {
	// Проверяем, что constraintStr начинается и заканчивается на '/'
	return len(constraintStr) >= 2 && constraintStr[0] == '/' && constraintStr[len(constraintStr)-1] == '/'
}

func (rp *RegexPlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	// Извлекаем паттерн между слешами
	pattern := constraintStr[1 : len(constraintStr)-1]

	// Проверяем, что паттерн не пустой
	if pattern == "" {
		return nil, fmt.Errorf("empty regex pattern")
	}

	// Компилируем регулярное выражение
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %v", err)
	}

	return func(value string) bool {
		// Пустая строка не должна совпадать с любым regex, кроме явно разрешающего
		if value == "" {
			return false
		}
		return re.MatchString(value)
	}, nil
}
