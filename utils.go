package paramvalidator

import (
	"path"
	"strings"
)

// normalizeURLPattern cleans and standardizes URL pattern
func NormalizeURLPattern(pattern string) string {
	if pattern == "" {
		return ""
	}

	// Быстрая проверка на wildcard
	if len(pattern) > 0 && pattern[0] == '*' {
		return pattern
	}

	// Избегать path.Clean для простых случаев
	if !strings.Contains(pattern, "./") && !strings.Contains(pattern, "//") {
		if pattern[0] != '/' {
			return "/" + pattern
		}
		return pattern
	}

	cleaned := path.Clean(pattern)
	if cleaned == "." {
		return "/"
	}
	return cleaned
}
