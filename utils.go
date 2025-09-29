package paramvalidator

import (
	"strings"
)

// normalizeURLPattern cleans and standardizes URL pattern
func NormalizeURLPattern(pattern string) string {
	if pattern == "" {
		return ""
	}

	// Быстрая проверка на простой случай (большинство URL)
	if isSimplePattern(pattern) {
		if pattern[0] != '/' {
			return "/" + pattern
		}
		return pattern
	}

	return normalizeComplexPattern(pattern)
}

// isSimplePattern проверяет, нуждается ли паттерн в сложной нормализации
func isSimplePattern(pattern string) bool {
	if len(pattern) == 0 {
		return true
	}

	// Быстрая проверка на наличие специальных символов
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*', '.', '/':
			// Проверяем конкретные случаи
			switch pattern[i] {
			case '*':
				// Проверяем двойные **
				if i+1 < len(pattern) && pattern[i+1] == '*' {
					return false
				}
			case '.':
				// Проверяем "./" или ".."
				if i+1 < len(pattern) && pattern[i+1] == '.' {
					return false // ".."
				}
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					return false // "./"
				}
			case '/':
				// Проверяем "//"
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					return false
				}
			}
		}
	}

	return pattern[0] == '/'
}

// normalizeComplexPattern обрабатывает сложные случаи
func normalizeComplexPattern(pattern string) string {
	// 1. Обработка двойных **
	pattern = removeDoubleStars(pattern)

	// 2. Если начинается с *, возвращаем как есть
	if len(pattern) > 0 && pattern[0] == '*' {
		return pattern
	}

	// 3. Обработка с path segments
	return cleanPathSegments(pattern)
}

// removeDoubleStars удаляет последовательные **
func removeDoubleStars(pattern string) string {
	// Быстрая проверка - есть ли двойные ** вообще
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

	// Ручная замена "**" на "*"
	var result strings.Builder
	result.Grow(len(pattern))

	for i := 0; i < len(pattern); i++ {
		if i < len(pattern)-1 && pattern[i] == '*' && pattern[i+1] == '*' {
			result.WriteByte('*')
			i++ // Пропускаем следующий *
		} else {
			result.WriteByte(pattern[i])
		}
	}

	return result.String()
}

// cleanPathSegments очищает path segments
func cleanPathSegments(pattern string) string {
	var segments []string
	start := 0

	// Ручной split по '/'
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

	// Собираем результат
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
	return "/" + result
}
