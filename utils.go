// utils.go
package paramvalidator

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
