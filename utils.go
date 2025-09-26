package paramvalidator

import (
	"path"
	"strings"
)

// normalizeURLPattern cleans and standardizes URL pattern
func NormalizeURLPattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return ""
	}

	if strings.Contains(pattern, "*") {
		return pattern
	}

	if !strings.HasPrefix(pattern, "/") {
		pattern = "/" + pattern
	}

	cleaned := path.Clean(pattern)
	if cleaned == "." {
		return "/"
	}
	return cleaned
}
