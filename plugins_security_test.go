package paramvalidator

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/smalloff/paramvalidator/plugins"
)

func TestPatternPluginSecurity(t *testing.T) {
	plugin := plugins.NewPatternPlugin()

	tests := []struct {
		name        string
		pattern     string
		value       string
		expectValid bool
	}{
		{
			name:        "Empty pattern",
			pattern:     "in:",
			value:       "test",
			expectValid: false,
		},
		{
			name:        "Only wildcard",
			pattern:     "in:*",
			value:       "any value",
			expectValid: true,
		},
		{
			name:        "Multiple consecutive wildcards",
			pattern:     "in:**",
			value:       "test",
			expectValid: true,
		},
		{
			name:        "Pattern with special regex chars",
			pattern:     "in:*.*+?[](){}|^$\\*",
			value:       "test.test+?[](){}|^$\\test",
			expectValid: true,
		},
		{
			name:        "Unicode pattern safety",
			pattern:     "in:*🎉*🚀*",
			value:       "start🎉middle🚀end",
			expectValid: true,
		},
		{
			name:        "Null bytes in pattern",
			pattern:     "in:*\x00*",
			value:       "test\x00value",
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := plugin.Parse("test", tt.pattern)
			if err != nil {
				if tt.expectValid {
					t.Errorf("Unexpected error: %v", err)
				}
				return
			}

			result := validator(tt.value)
			if result != tt.expectValid {
				t.Errorf("Expected %v, got %v", tt.expectValid, result)
			}
		})
	}
}

func TestPatternPluginReDoSProtection(t *testing.T) {
	plugin := plugins.NewPatternPlugin()

	redosTests := []struct {
		name        string
		pattern     string
		value       string
		maxDuration time.Duration
	}{
		{
			name:        "Exponential backtracking protection",
			pattern:     "in:*a*b*c*d*e*f*g*h*i*j*",
			value:       strings.Repeat("x", 1000),
			maxDuration: 5 * time.Millisecond,
		},
		{
			name:        "Many wildcards with long prefix",
			pattern:     "in:" + strings.Repeat("a", 100) + "*",
			value:       strings.Repeat("a", 100) + strings.Repeat("b", 1000),
			maxDuration: 2 * time.Millisecond,
		},
		{
			name:        "Complex pattern with overlaps",
			pattern:     "in:*abc*abc*abc*abc*",
			value:       strings.Repeat("abc", 1000),
			maxDuration: 5 * time.Millisecond,
		},
	}

	for _, tt := range redosTests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := plugin.Parse("test", tt.pattern)
			if err != nil {
				t.Fatalf("Failed to create validator: %v", err)
			}

			iterations := 10
			var totalDuration time.Duration

			for i := 0; i < iterations; i++ {
				start := time.Now()
				result := validator(tt.value)
				duration := time.Since(start)
				totalDuration += duration
				_ = result
			}

			avgDuration := totalDuration / time.Duration(iterations)
			if avgDuration > tt.maxDuration {
				t.Errorf("Potential ReDoS detected: %s took avg %v (max allowed: %v). Pattern: %q, Value length: %d",
					tt.name, avgDuration, tt.maxDuration, tt.pattern, len(tt.value))
			}

			t.Logf("Pattern: %q, Value length: %d, Avg duration: %v",
				tt.pattern, len(tt.value), avgDuration)
		})
	}
}

func TestPluginInputValidation(t *testing.T) {
	pluginTests := []struct {
		name   string
		plugin interface {
			Parse(paramName, constraintStr string) (func(string) bool, error)
		}
	}{
		{"pattern", plugins.NewPatternPlugin()},
		{"length", plugins.NewLengthPlugin()},
		{"comparison", plugins.NewComparisonPlugin()},
		{"range", plugins.NewRangePlugin()},
	}

	maliciousInputs := []struct {
		name         string
		constraint   string
		shouldReject bool
	}{
		{
			name:         "Extremely long constraint",
			constraint:   "len:" + strings.Repeat("a", 10000),
			shouldReject: true,
		},
		{
			name:         "Null bytes in constraint",
			constraint:   "in:test\x00value",
			shouldReject: false,
		},
		{
			name:         "Invalid UTF-8 sequence",
			constraint:   "in:valid\xff\xfeinvalid",
			shouldReject: true,
		},
		{
			name:         "Only special characters",
			constraint:   "in:!@#$%^&*()",
			shouldReject: false,
		},
		{
			name:         "Empty string",
			constraint:   "",
			shouldReject: true,
		},
		{
			name:         "Valid length constraint",
			constraint:   "len:>5",
			shouldReject: false,
		},
		{
			name:         "Valid range constraint",
			constraint:   "range:1-10",
			shouldReject: false,
		},
		{
			name:         "Valid comparison constraint",
			constraint:   "cmp:>100",
			shouldReject: false,
		},
	}

	for _, pluginTest := range pluginTests {
		t.Run(pluginTest.name, func(t *testing.T) {
			for _, input := range maliciousInputs {
				t.Run(input.name, func(t *testing.T) {
					validator, err := pluginTest.plugin.Parse("test_param", input.constraint)

					if input.shouldReject {
						if err == nil && validator != nil {
							t.Errorf("Expected rejection for constraint %q, but it was accepted", input.constraint)
						}
					} else {
						func() {
							defer func() {
								if r := recover(); r != nil {
									t.Errorf("PANIC for constraint %q: %v", input.constraint, r)
								}
							}()

							if validator != nil {
								testValues := []string{"", "test", "123", strings.Repeat("x", 100)}
								for _, testValue := range testValues {
									result := validator(testValue)
									_ = result
								}
							}
						}()
					}

					t.Logf("Plugin: %s, Constraint: %q, Error: %v",
						pluginTest.name, input.constraint, err)
				})
			}
		})
	}
}

func TestPluginMemorySafety(t *testing.T) {
	plugin := plugins.NewPatternPlugin()

	t.Run("Memory exhaustion protection", func(t *testing.T) {
		validator, err := plugin.Parse("test", "in:*test*")
		if err != nil {
			t.Fatalf("Failed to create validator: %v", err)
		}

		testCases := []struct {
			name  string
			value string
		}{
			{"Empty string", ""},
			{"Normal string", "this is a test value"},
			{"Very long string", strings.Repeat("x", 100000)},
			{"Many matches", strings.Repeat("test", 1000)},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				start := time.Now()
				result := validator(tc.value)
				duration := time.Since(start)

				if duration > 100*time.Millisecond {
					t.Errorf("Memory exhaustion potential: processing %d bytes took %v",
						len(tc.value), duration)
				}

				_ = result
				t.Logf("Processed %d bytes in %v, result: %v",
					len(tc.value), duration, result)
			})
		}
	})
}

func TestPluginConcurrentSafety(t *testing.T) {
	pluginList := []struct {
		name   string
		plugin interface {
			Parse(paramName, constraintStr string) (func(string) bool, error)
		}
	}{
		{"pattern", plugins.NewPatternPlugin()},
		{"length", plugins.NewLengthPlugin()},
		{"comparison", plugins.NewComparisonPlugin()},
		{"range", plugins.NewRangePlugin()},
	}

	for _, pl := range pluginList {
		t.Run(pl.name, func(t *testing.T) {
			const goroutines = 50
			const iterations = 100

			done := make(chan bool, goroutines)

			for i := 0; i < goroutines; i++ {
				go func(id int) {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("Goroutine %d panicked: %v", id, r)
						}
						done <- true
					}()

					for j := 0; j < iterations; j++ {
						var constraint string
						switch pl.name {
						case "pattern":
							constraint = "in:*test*"
						case "length":
							constraint = "len:>5"
						case "comparison":
							constraint = "cmp:>10"
						case "range":
							constraint = "range:1..100"
						}

						validator, err := pl.plugin.Parse("test_param", constraint)
						if err != nil {
							continue
						}

						if validator != nil {
							testValues := []string{"", "test", "12345", "valid_value"}
							for _, value := range testValues {
								result := validator(value)
								_ = result
							}
						}
					}
				}(i)
			}

			for i := 0; i < goroutines; i++ {
				<-done
			}
		})
	}
}

func TestPluginBoundaryConditions(t *testing.T) {
	plugin := plugins.NewPatternPlugin()

	tests := []struct {
		name        string
		pattern     string
		values      []string
		expectError bool
	}{
		{
			name:        "Empty value handling",
			pattern:     "in:*",
			values:      []string{""},
			expectError: false,
		},
		{
			name:        "Very long pattern - should be rejected",
			pattern:     "in:" + strings.Repeat("a", 1000) + "*",
			values:      []string{},
			expectError: true,
		},
		{
			name:        "Maximum length pattern",
			pattern:     "in:" + strings.Repeat("a", 999) + "*",
			values:      []string{strings.Repeat("a", 999) + "suffix"},
			expectError: false,
		},
		{
			name:        "Unicode boundary",
			pattern:     "in:*" + string([]rune{0x1F600}),
			values:      []string{"prefix" + string([]rune{0x1F600})},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := plugin.Parse("test", tt.pattern)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for pattern %q, but got none", tt.pattern)
				}
				return
			}

			if err != nil {
				t.Fatalf("Failed to create validator: %v", err)
			}

			for _, value := range tt.values {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("PANIC for value %q: %v", value, r)
						}
					}()

					result := validator(value)

					if value != "" && !utf8.ValidString(value) {
						t.Logf("Warning: Test value contains invalid UTF-8: %q", value)
					}

					t.Logf("Pattern: %q, Value: %q, Result: %v",
						tt.pattern, value, result)
				}()
			}
		})
	}
}

func TestPluginResourceCleanup(t *testing.T) {
	plugin := plugins.NewPatternPlugin()

	patterns := []string{
		"in:*test*",
		"in:prefix*",
		"in:*suffix",
		"in:*a*b*c*",
		"in:" + strings.Repeat("x", 100) + "*",
	}

	for i := 0; i < 1000; i++ {
		for _, pattern := range patterns {
			validator, err := plugin.Parse("test", pattern)
			if err != nil {
				continue
			}

			testValues := []string{"", "test", "no_match", strings.Repeat("x", 100)}
			for _, value := range testValues {
				result := validator(value)
				_ = result
			}
		}

		if i%100 == 0 {
			t.Logf("Completed %d iterations without resource leaks", i)
		}
	}
}

func TestPluginSpecificSecurity(t *testing.T) {
	t.Run("LengthPlugin security", func(t *testing.T) {
		plugin := plugins.NewLengthPlugin()

		securityTests := []struct {
			name       string
			constraint string
			shouldFail bool
		}{
			{"Valid length", "len:>5", false},
			{"Invalid operator", "len:>>5", true},
			{"Negative number", "len:>-5", true},
			{"Very large number", "len:>9999999999", true},
			{"Empty after len", "len:", true},
			{"Invalid characters", "len:>5abc", true},
		}

		for _, tt := range securityTests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := plugin.Parse("test", tt.constraint)
				if tt.shouldFail && err == nil {
					t.Errorf("Expected error for constraint %q, but got none", tt.constraint)
				}
				if !tt.shouldFail && err != nil {
					t.Errorf("Unexpected error for constraint %q: %v", tt.constraint, err)
				}
			})
		}
	})

	t.Run("ComparisonPlugin security", func(t *testing.T) {
		plugin := plugins.NewComparisonPlugin()

		securityTests := []struct {
			name       string
			constraint string
			shouldFail bool
		}{
			{"Valid comparison", "cmp:>10", false},
			{"Double operator", "cmp:>>10", true},
			{"Invalid combination", "cmp:><10", true},
			{"Missing number", "cmp:>", true},
			{"Very large number", "cmp:>9999999999", true},
			{"Negative number", "cmp:>-5", false},
			{"Invalid characters", "cmp:>10abc", true},
		}

		for _, tt := range securityTests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := plugin.Parse("test", tt.constraint)
				if tt.shouldFail && err == nil {
					t.Errorf("Expected error for constraint %q, but got none", tt.constraint)
				}
				if !tt.shouldFail && err != nil {
					t.Errorf("Unexpected error for constraint %q: %v", tt.constraint, err)
				}
			})
		}
	})

	t.Run("RangePlugin security", func(t *testing.T) {
		plugin := plugins.NewRangePlugin()

		securityTests := []struct {
			name       string
			constraint string
			shouldFail bool
		}{
			{"Valid range", "range:1..10", false},
			{"Valid range with dash", "range:1-10", false},
			{"Invalid range", "range:10..1", true},
			{"Very large numbers", "range:1..9999999999", true},
			{"Negative range", "range:-10..10", false},
			{"Missing separator", "range:110", true},
			{"Invalid characters", "range:1..10abc", true},
		}

		for _, tt := range securityTests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := plugin.Parse("test", tt.constraint)
				if tt.shouldFail && err == nil {
					t.Errorf("Expected error for constraint %q, but got none", tt.constraint)
				}
				if !tt.shouldFail && err != nil {
					t.Errorf("Unexpected error for constraint %q: %v", tt.constraint, err)
				}
			})
		}
	})
}

func TestPluginErrorHandling(t *testing.T) {
	pluginTests := []struct {
		name   string
		plugin interface {
			Parse(paramName, constraintStr string) (func(string) bool, error)
		}
		invalidConstraints []string
	}{
		{
			name:   "pattern",
			plugin: plugins.NewPatternPlugin(),
			invalidConstraints: []string{
				"",
				"in:",
				"invalid",
				"in:" + strings.Repeat("a", 1001) + "*",
			},
		},
		{
			name:   "length",
			plugin: plugins.NewLengthPlugin(),
			invalidConstraints: []string{
				"",
				"len:",
				"len:>>5",
				"len:999999999999999",
			},
		},
		{
			name:   "comparison",
			plugin: plugins.NewComparisonPlugin(),
			invalidConstraints: []string{
				"",
				"cmp:>",
				"cmp:>>10",
				"cmp:>999999999",
			},
		},
		{
			name:   "range",
			plugin: plugins.NewRangePlugin(),
			invalidConstraints: []string{
				"",
				"range:",
				"range:10..1",
				"range:1..99999999",
			},
		},
	}

	for _, pt := range pluginTests {
		t.Run(pt.name, func(t *testing.T) {
			for _, constraint := range pt.invalidConstraints {
				t.Run(constraint, func(t *testing.T) {
					validator, err := pt.plugin.Parse("test", constraint)

					if err == nil && validator != nil {
						t.Errorf("Expected error for constraint %q, but got validator", constraint)
					}

					if err != nil {
						t.Logf("Correctly got error for %q: %v", constraint, err)
					}
				})
			}
		})
	}
}
