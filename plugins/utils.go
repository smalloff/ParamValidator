package plugins

import (
	"unicode/utf8"
)

func isValidUTF8(s string) bool {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			return false
		}
		i += size
	}
	return true
}

func parseNumber(s string) (int, bool) {
	if len(s) == 0 || len(s) > 10 {
		return 0, false
	}

	var result int
	start := 0
	if s[0] == '-' {
		start = 1
	}

	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		result = result*10 + int(s[i]-'0')
	}

	if start == 1 {
		return -result, true
	}
	return result, true
}

func stringLength(s string) int {
	// Быстрый путь для ASCII строк
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			// Нашли не-ASCII символ, используем полный подсчет
			return utf8.RuneCountInString(s)
		}
	}
	// Вся строка ASCII - возвращаем длину в байтах
	return len(s)
}
