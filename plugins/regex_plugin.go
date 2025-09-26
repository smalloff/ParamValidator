package plugins

import (
	"fmt"
	"regexp"
	"sync"
)

// RegexPlugin плагин для регулярных выражений: /pattern/
type RegexPlugin struct {
	name    string
	cache   map[string]*regexp.Regexp
	cacheMu sync.RWMutex
}

func NewRegexPlugin() *RegexPlugin {
	return &RegexPlugin{
		name:  "regex",
		cache: make(map[string]*regexp.Regexp),
	}
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

	// Пытаемся получить из кэша
	rp.cacheMu.RLock()
	if re, exists := rp.cache[pattern]; exists {
		rp.cacheMu.RUnlock()
		return rp.createValidator(re), nil
	}
	rp.cacheMu.RUnlock()

	// Компилируем регулярное выражение
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %v", err)
	}

	// Сохраняем в кэш
	rp.cacheMu.Lock()
	rp.cache[pattern] = re
	rp.cacheMu.Unlock()

	return rp.createValidator(re), nil
}

func (rp *RegexPlugin) createValidator(re *regexp.Regexp) func(string) bool {
	return func(value string) bool {
		// Пустая строка не должна совпадать с любым regex, кроме явно разрешающего
		if value == "" {
			return false
		}
		return re.MatchString(value)
	}
}

func (rp *RegexPlugin) Close() error {
	rp.cacheMu.Lock()
	defer rp.cacheMu.Unlock()

	// Очищаем кэш
	rp.cache = make(map[string]*regexp.Regexp)
	return nil
}
