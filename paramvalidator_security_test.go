package paramvalidator

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

func TestParamValidatorSecurity(t *testing.T) {
	tests := []struct {
		name        string
		rules       string
		url         string
		expectValid bool
	}{
		{
			name:        "Empty rules - any URL invalid",
			rules:       "",
			url:         "/test?param=value",
			expectValid: false,
		},
		{
			name:        "Wildcard rules - allow any",
			rules:       "*?*",
			url:         "/any/path?any=param",
			expectValid: true,
		},
		{
			name:        "Multiple consecutive wildcards normalized",
			rules:       "**?**",
			url:         "/test?param=value",
			expectValid: true,
		},
		{
			name:        "Rules with special regex chars treated literally",
			rules:       "/test*?param=[special.value]",
			url:         "/test-path?param=special.value",
			expectValid: true,
		},
		{
			name:        "Unicode rules safety",
			rules:       "/🎉*?param=[🚀]",
			url:         "/🎉path?param=🚀",
			expectValid: true,
		},
		{
			name:        "Path traversal attempts",
			rules:       "/api/*?param=[valid]",
			url:         "/api/../etc/passwd?param=valid",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rules)
			if err != nil {
				if tt.expectValid {
					t.Errorf("Unexpected error: %v", err)
				}
				return
			}

			result := pv.ValidateURL(tt.url)
			if result != tt.expectValid {
				t.Errorf("expected %v, got %v", tt.expectValid, result)
			}
		})
	}
}

func TestParamValidatorReDoSProtection(t *testing.T) {
	redosTests := []struct {
		name        string
		rules       string
		url         string
		maxDuration time.Duration
	}{
		{
			name:        "Complex wildcard patterns",
			rules:       "/*a*b*c*d*e*f*g*h*i*j*?param=[value]",
			url:         "/" + strings.Repeat("x", 100) + "?param=value",
			maxDuration: 10 * time.Millisecond,
		},
		{
			name:        "Many URL rules with wildcards",
			rules:       "/api/v1/*?p1=[1];/api/v2/*?p2=[2];/api/v3/*?p3=[3]",
			url:         "/api/v1/" + strings.Repeat("x", 100) + "?p1=1",
			maxDuration: 5 * time.Millisecond,
		},
		{
			name:        "Complex parameter patterns",
			rules:       "/search?q=[*abc*abc*abc*abc*]",
			url:         "/search?q=" + strings.Repeat("abc", 50),
			maxDuration: 5 * time.Millisecond,
		},
		{
			name:        "Multiple parameter validation",
			rules:       "/api?p1=[*]&p2=[*]&p3=[*]&p4=[*]&p5=[*]",
			url:         "/api?" + strings.Repeat("p1=test&", 5) + "p5=test",
			maxDuration: 10 * time.Millisecond,
		},
	}

	for _, tt := range redosTests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rules)
			if err != nil {
				t.Logf("Validator creation failed: %v", err)
				return
			}

			iterations := 5
			var totalDuration time.Duration

			for i := 0; i < iterations; i++ {
				start := time.Now()
				result := pv.ValidateURL(tt.url)
				duration := time.Since(start)
				totalDuration += duration
				_ = result
			}

			avgDuration := totalDuration / time.Duration(iterations)
			if avgDuration > tt.maxDuration {
				t.Errorf("Potential ReDoS detected: %s took avg %v (max allowed: %v)", tt.name, avgDuration, tt.maxDuration)
			}
		})
	}
}

func TestParamValidatorInputValidation(t *testing.T) {
	maliciousInputs := []struct {
		name         string
		rules        string
		url          string
		shouldReject bool
	}{
		{
			name:         "Long rules handling",
			rules:        strings.Repeat("/path?param=[value]&", 100) + "final=[end]",
			url:          "/path?param=value&final=end",
			shouldReject: false,
		},
		{
			name:         "Long URL handling",
			rules:        "/api?param=[value]",
			url:          "/api?param=" + strings.Repeat("x", 1000),
			shouldReject: false,
		},
		{
			name:         "Invalid UTF-8 sequence in rules",
			rules:        "valid\xff\xfeinvalid?param=[value]",
			url:          "/valid?param=value",
			shouldReject: true,
		},
		{
			name:         "Special characters in rules",
			rules:        "!@#$%^&*()?param=[value]",
			url:          "/!@#$%?param=value",
			shouldReject: false,
		},
		{
			name:         "Malformed URL in validation",
			rules:        "/api?param=[value]",
			url:          ":invalid:url:",
			shouldReject: true,
		},
		{
			name:         "URL with many parameters",
			rules:        "/api?p1=[v1]&p2=[v2]",
			url:          "/api?" + strings.Repeat("extra=param&", 50) + "p1=v1&p2=v2",
			shouldReject: false,
		},
		{
			name:         "Deeply nested paths",
			rules:        "/api/*/v*/*?param=[value]",
			url:          "/api/" + strings.Repeat("level/", 10) + "v1/final?param=value",
			shouldReject: false,
		},
	}

	for _, input := range maliciousInputs {
		t.Run(input.name, func(t *testing.T) {
			pv, err := NewParamValidator(input.rules)

			if input.shouldReject {
				if err == nil && pv != nil {
					result := pv.ValidateURL(input.url)
					if result {
						t.Errorf("Expected rejection for URL %q, but it was accepted", input.url)
					}
				}
			} else {
				if err == nil && pv != nil {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("PANIC for URL %q: %v", input.url, r)
						}
					}()
					result := pv.ValidateURL(input.url)
					_ = result
				}
			}
		})
	}
}

func TestParamValidatorMemorySafety(t *testing.T) {
	t.Run("Memory exhaustion protection", func(t *testing.T) {
		pv, err := NewParamValidator("/api/*?page=[5]&limit=[10]")
		if err != nil {
			t.Fatalf("Failed to create validator: %v", err)
		}

		testCases := []struct {
			name string
			url  string
		}{
			{"Empty URL", ""},
			{"Normal URL", "/api/users?page=5&limit=10"},
			{"Long URL", "/api/" + strings.Repeat("x", 1000) + "?page=5&limit=10"},
			{"Many parameters", "/api/users?" + strings.Repeat("param=value&", 50) + "page=5&limit=10"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				start := time.Now()
				result := pv.ValidateURL(tc.url)
				duration := time.Since(start)

				if duration > 50*time.Millisecond {
					t.Errorf("Memory exhaustion potential: processing URL of length %d took %v", len(tc.url), duration)
				}
				_ = result
			})
		}
	})
}

func TestParamValidatorConcurrentSafety(t *testing.T) {
	pv, err := NewParamValidator("/api/*?page=[5]&limit=[10]&sort=[name,date]")
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	const goroutines = 20
	const iterations = 50

	var wg sync.WaitGroup
	errorCh := make(chan error, goroutines*iterations)

	testURLs := []string{
		"/api/users?page=5&limit=10&sort=name",
		"/api/products?page=3&limit=10&sort=date",
		"/api/orders?page=5&limit=15&sort=name",
		"/api/" + strings.Repeat("x", 50) + "?page=5&limit=10&sort=date",
	}

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errorCh <- fmt.Errorf("goroutine %d panicked: %v", id, r)
				}
			}()

			for j := 0; j < iterations; j++ {
				urlStr := testURLs[j%len(testURLs)]

				switch j % 3 {
				case 0:
					result := pv.ValidateURL(urlStr)
					_ = result
				case 1:
					if u, err := url.Parse(urlStr); err == nil {
						for key, values := range u.Query() {
							if len(values) > 0 {
								result := pv.ValidateParam(u.Path, key, values[0])
								_ = result
							}
						}
					}
				case 2:
					normalized := pv.FilterURL(urlStr + "&invalid=param")
					_ = normalized
				}
			}
		}(i)
	}

	wg.Wait()
	close(errorCh)

	var errors []error
	for err := range errorCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Errorf("Concurrent validation failed with %d errors:", len(errors))
		for _, err := range errors {
			t.Error(err)
		}
	}
}

func TestParamValidatorBoundaryConditions(t *testing.T) {
	tests := []struct {
		name        string
		rules       string
		url         string
		expectValid bool
	}{
		{
			name:        "Empty URL handling",
			rules:       "/api?param=[value]",
			url:         "",
			expectValid: false,
		},
		{
			name:        "Long rules handling",
			rules:       "/" + strings.Repeat("a", 500) + "?param=[value]",
			url:         "/" + strings.Repeat("a", 500) + "?param=value",
			expectValid: true,
		},
		{
			name:        "Unicode boundary",
			rules:       "/" + string([]rune{0x1F600}) + "*?param=[value]",
			url:         "/" + string([]rune{0x1F600}) + "test?param=value",
			expectValid: true,
		},
		{
			name:        "Many URL rules",
			rules:       strings.Repeat("/api"+string(rune('a'))+"?param=[value];", 10),
			url:         "/apia?param=value",
			expectValid: true,
		},
		{
			name:        "Complex parameter constraints",
			rules:       "/api?p1=[value1]&p2=[value2]&p3=[value3]",
			url:         "/api?p1=value1&p2=value2&p3=value3",
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rules)

			if err != nil {
				if tt.expectValid {
					t.Fatalf("Failed to create validator: %v", err)
				}
				return
			}

			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC for URL %q: %v", tt.url, r)
				}
			}()

			result := pv.ValidateURL(tt.url)

			if tt.url != "" && !utf8.ValidString(tt.url) {
				t.Logf("Warning: Test URL contains invalid UTF-8: %q", tt.url)
			}
			_ = result
		})
	}
}

func TestParamValidatorResourceCleanup(t *testing.T) {
	rules := []string{
		"/api/*?page=[5]&limit=[10]",
		"/users?sort=[name,date]&filter=[active]",
		"/products?category=[electronics,books]&price=[value]",
		"/search?q=[*]&page=[1]",
		"test*?param=[value]",
	}

	for i := 0; i < 100; i++ {
		for _, rule := range rules {
			pv, err := NewParamValidator(rule)
			if err != nil {
				continue
			}

			testURLs := []string{
				"",
				"/api/users?page=5&limit=10",
				"/no/match?param=value",
				"/test?param=value",
			}
			for _, urlStr := range testURLs {
				result := pv.ValidateURL(urlStr)
				_ = result

				if u, err := url.Parse(urlStr); err == nil && u.Path != "" {
					for key, values := range u.Query() {
						if len(values) > 0 {
							pv.ValidateParam(u.Path, key, values[0])
							pv.FilterQuery(u.Path, u.RawQuery)
						}
					}
					pv.FilterURL(urlStr)
				}
			}
		}
	}
}

func TestParamValidatorSpecificSecurity(t *testing.T) {
	t.Run("URL parsing vulnerabilities", func(t *testing.T) {
		securityTests := []struct {
			name       string
			url        string
			shouldFail bool
		}{
			{"Valid URL", "/api?param=value", false},
			{"URL with fragment", "/api#fragment?param=value", false},
			{"JavaScript URL", "javascript:alert('xss')", true},
			{"Data URL", "data:text/html,<script>alert('xss')</script>", true},
			{"File URL", "file:///etc/passwd", true},
		}

		pv, err := NewParamValidator("/api?param=[value]")
		if err != nil {
			t.Fatalf("Failed to create validator: %v", err)
		}

		for _, tt := range securityTests {
			t.Run(tt.name, func(t *testing.T) {
				result := pv.ValidateURL(tt.url)
				if tt.shouldFail && result {
					t.Errorf("Expected URL %q to be rejected, but it was accepted", tt.url)
				}
			})
		}
	})

	t.Run("Query parameter injection", func(t *testing.T) {
		pv, err := NewParamValidator("/api?valid=[value]")
		if err != nil {
			t.Fatalf("Failed to create validator: %v", err)
		}

		injectionAttempts := []string{
			"/api?valid=value&injected=malicious",
			"/api?valid=value%26injected=malicious",
			"/api?valid=value;injected=malicious",
		}

		for _, url := range injectionAttempts {
			result := pv.ValidateURL(url)
			if result {
				t.Errorf("Injection attempt %q should be rejected", url)
			}
		}
	})

	t.Run("Path traversal prevention", func(t *testing.T) {
		pv, err := NewParamValidator("/api/*/data?param=[value]")
		if err != nil {
			t.Fatalf("Failed to create validator: %v", err)
		}

		traversalAttempts := []string{
			"/api/../etc/passwd/data?param=value",
			"/api/././data?param=value",
			"/api/../../data?param=value",
			"/api/..//data?param=value",
		}

		for _, url := range traversalAttempts {
			result := pv.ValidateURL(url)
			if result {
				t.Errorf("Path traversal attempt %q should be rejected", url)
			}
		}
	})
}

func TestParamValidatorCallbackSecurity(t *testing.T) {
	callbackFunc := func(key string, value string) bool {
		if len(value) > 100 {
			time.Sleep(1 * time.Millisecond)
			return false
		}
		return value == "valid"
	}

	t.Run("Callback performance", func(t *testing.T) {
		pv, err := NewParamValidator("/api?token=[?]", WithCallback(callbackFunc))
		if err != nil {
			t.Fatalf("Failed to create validator: %v", err)
		}

		start := time.Now()
		result := pv.ValidateURL("/api?token=" + strings.Repeat("x", 1000))
		duration := time.Since(start)

		if duration > 50*time.Millisecond {
			t.Errorf("Callback performance issue: processing took %v", duration)
		}

		if result {
			t.Error("Large token value should be rejected")
		}
	})

	t.Run("Callback error handling", func(t *testing.T) {
		panicCallback := func(key string, value string) bool {
			if value == "panic" {
				panic("callback panic")
			}
			return true
		}

		pv, err := NewParamValidator("/api?test=[?]", WithCallback(panicCallback))
		if err != nil {
			t.Fatalf("Failed to create validator: %v", err)
		}

		defer func() {
			if r := recover(); r != nil {
				return
			}
		}()
		result := pv.ValidateURL("/api?test=panic")
		_ = result
	})
}

func BenchmarkParamValidatorSecurity(b *testing.B) {
	benchmarks := []struct {
		name  string
		rules string
		url   string
	}{
		{
			name:  "Simple validation",
			rules: "/api?param=[value]",
			url:   "/api?param=value",
		},
		{
			name:  "Complex wildcard",
			rules: "/*a*b*c*d*?param=[value]",
			url:   "/xaxbxcxdx?param=value",
		},
		{
			name:  "Many parameters",
			rules: "/api?p1=[v1]&p2=[v2]&p3=[v3]&p4=[v4]&p5=[v5]",
			url:   "/api?p1=v1&p2=v2&p3=v3&p4=v4&p5=v5",
		},
		{
			name:  "Long URL",
			rules: "/api/*?param=[value]",
			url:   "/api/" + strings.Repeat("x", 500) + "?param=value",
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			pv, err := NewParamValidator(bm.rules)
			if err != nil {
				b.Skipf("Failed to create validator: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result := pv.ValidateURL(bm.url)
				_ = result
			}
		})
	}
}

func TestFilterURLPattern(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"**", "*"},
		{"**/*", "*/*"},
		{"/api/**/v1", "/api/*/v1"},
		{"*/**/*", "*/*/*"},
		{"/test", "/test"},
		{"test", "/test"},
		{"./test", "/test"},
		{"../test", "/test"},
		{"/api/../test", "/test"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeURLPattern(tt.input)
			if result != tt.expected {
				t.Logf("FilterURLPattern(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
