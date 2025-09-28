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

	for strings.Contains(pattern, "**") {
		pattern = strings.ReplaceAll(pattern, "**", "*")
	}

	if len(pattern) > 0 && pattern[0] == '*' {
		return pattern
	}

	if !strings.Contains(pattern, "./") && !strings.Contains(pattern, "//") && !strings.Contains(pattern, "..") {
		if pattern[0] != '/' {
			return "/" + pattern
		}
		return pattern
	}

	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "/")
		var cleanedParts []string
		for _, part := range parts {
			if part == "" || part == "." {
				continue
			}
			if part == ".." {
				if len(cleanedParts) > 0 {
					cleanedParts = cleanedParts[:len(cleanedParts)-1]
				}
				continue
			}
			cleanedParts = append(cleanedParts, part)
		}
		result := strings.Join(cleanedParts, "/")
		if pattern[0] == '/' {
			result = "/" + result
		}
		return result
	}

	cleaned := path.Clean(pattern)
	if cleaned == "." {
		return "/"
	}
	if pattern[0] == '/' && cleaned[0] != '/' {
		return "/" + cleaned
	}
	return cleaned
}
