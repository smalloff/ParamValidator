// utils.go
package paramvalidator

import (
	"strings"
)

// normalizeURLPattern cleans and standardizes URL pattern
func normalizeURLPattern(pattern string) string {
	if pattern == "" {
		return ""
	}

	// Fast check for simple case (most URLs)
	if isSimplePattern(pattern) {
		if pattern[0] != '/' {
			return "/" + pattern
		}
		return pattern
	}

	return normalizeComplexPattern(pattern)
}

// isSimplePattern checks if pattern needs complex normalization
func isSimplePattern(pattern string) bool {
	if len(pattern) == 0 {
		return true
	}

	// Fast check for special characters
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*', '.', '/':
			// Check specific cases
			switch pattern[i] {
			case '*':
				// Check for double **
				if i+1 < len(pattern) && pattern[i+1] == '*' {
					return false
				}
			case '.':
				// Check for "./" or ".."
				if i+1 < len(pattern) && pattern[i+1] == '.' {
					return false // ".."
				}
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					return false // "./"
				}
			case '/':
				// Check for "//"
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					return false
				}
			}
		}
	}

	return pattern[0] == '/'
}

// normalizeComplexPattern handles complex cases
func normalizeComplexPattern(pattern string) string {
	// 1. Handle double **
	pattern = removeDoubleStars(pattern)

	// 2. If starts with *, return as is
	if len(pattern) > 0 && pattern[0] == '*' {
		return pattern
	}

	// 3. Handle path segments
	return cleanPathSegments(pattern)
}

// removeDoubleStars removes consecutive **
func removeDoubleStars(pattern string) string {
	// Fast check - are there any double ** at all
	hasDoubleStar := false
	for i := 0; i < len(pattern)-1; i++ {
		if pattern[i] == '*' && pattern[i+1] == '*' {
			hasDoubleStar = true
			break
		}
	}

	if !hasDoubleStar {
		return pattern
	}

	// Manual replacement of "**" with "*"
	var result strings.Builder
	result.Grow(len(pattern))

	for i := 0; i < len(pattern); i++ {
		if i < len(pattern)-1 && pattern[i] == '*' && pattern[i+1] == '*' {
			result.WriteByte('*')
			i++ // Skip next *
		} else {
			result.WriteByte(pattern[i])
		}
	}

	return result.String()
}

// cleanPathSegments cleans path segments
func cleanPathSegments(pattern string) string {
	var segments []string
	start := 0

	// Manual split by '/'
	for i := 0; i <= len(pattern); i++ {
		if i == len(pattern) || pattern[i] == '/' {
			if start < i {
				segment := pattern[start:i]
				if segment == ".." {
					if len(segments) > 0 {
						segments = segments[:len(segments)-1]
					}
				} else {
					segments = append(segments, segment)
				}
			}
			start = i + 1
		}
	}

	// Build result
	if len(segments) == 0 {
		if pattern[0] == '/' {
			return "/"
		}
		return "/"
	}

	result := strings.Join(segments, "/")
	if pattern[0] == '/' {
		return "/" + result
	}
	return result
}

// containsPathTraversal checks for path traversal patterns
func containsPathTraversal(path string) bool {
	n := len(path)
	if n < 2 {
		return false
	}

	for i := 0; i < n-1; i++ {
		// Check for ".."
		if path[i] == '.' && path[i+1] == '.' {
			return true
		}
		// Check for "//"
		if path[i] == '/' && path[i+1] == '/' {
			return true
		}
		// Check for "./"
		if path[i] == '.' && path[i+1] == '/' {
			return true
		}
		// Check for "/." followed by end or another slash
		if path[i] == '/' && path[i+1] == '.' {
			if i+2 == n || path[i+2] == '/' {
				return true
			}
		}
	}
	return false
}

// findSpecialCharsUltraFast highly optimized search for *
func findSpecialCharsUltraFast(s string) bool {
	n := len(s)
	i := 0

	// Process 4 characters at a time
	for ; i <= n-4; i += 4 {
		if s[i] == '*' || s[i+1] == '*' || s[i+2] == '*' || s[i+3] == '*' {
			return true
		}
	}

	// Process remaining characters
	for ; i < n; i++ {
		if s[i] == '*' {
			return true
		}
	}

	return false
}

// bytesEqual compares two byte slices for equality
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
