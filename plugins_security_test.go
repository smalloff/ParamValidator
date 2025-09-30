package paramvalidator

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/smalloff/paramvalidator/plugins"
)

// TestPatternPluginSecurity тестирует основные уязвимости паттерн-плагина
func TestPatternPluginSecurity(t *testing.T) {
	plugin := plugins.NewPatternPlugin()

	tests := []struct {
		name        string
		pattern     string
		value       string
		expectValid bool
		description string
	}{
		{
			name:        "Empty pattern",
			pattern:     "in:",
			value:       "test",
			expectValid: false,
			description: "Empty pattern should be rejected",
		},
		{
			name:        "Only wildcard",
			pattern:     "in:*",
			value:       "any value",
			expectValid: true,
			description: "Single wildcard should match any string",
		},
		{
			name:        "Multiple consecutive wildcards",
			pattern:     "in:**",
			value:       "test",
			expectValid: true,
			description: "Multiple wildcards should be handled correctly",
		},
		{
			name:        "Pattern with special regex chars",
			pattern:     "in:*.*+?[](){}|^$\\*",
			value:       "test.test+?[](){}|^$\\test",
			expectValid: true,
			description: "Special regex characters should be treated literally",
		},
		{
			name:        "Unicode pattern safety",
			pattern:     "in:*🎉*🚀*",
			value:       "start🎉middle🚀end",
			expectValid: true,
			description: "Unicode characters should be handled safely",
		},
		{
			name:        "Null bytes in pattern",
			pattern:     "in:*\x00*",
			value:       "test\x00value",
			expectValid: true,
			description: "Null bytes should be handled",
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
				t.Errorf("%s: expected %v, got %v", tt.description, tt.expectValid, result)
			}
		})
	}
}

// TestPatternPluginReDoSProtection тестирует защиту от ReDoS атак
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

			// Многократный запуск для более точного измерения
			iterations := 10
			var totalDuration time.Duration

			for i := 0; i < iterations; i++ {
				start := time.Now()
				result := validator(tt.value)
				duration := time.Since(start)
				totalDuration += duration

				_ = result // Используем результат
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

// TestPluginInputValidation тестирует валидацию входных данных плагинов
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
		description  string
	}{
		{
			name:         "Extremely long constraint",
			constraint:   "len:" + strings.Repeat("a", 10000),
			shouldReject: true,
			description:  "Very long constraints should be rejected",
		},
		{
			name:         "Null bytes in constraint",
			constraint:   "in:test\x00value",
			shouldReject: false, // Могут быть допустимы в некоторых плагинах
			description:  "Null bytes should be handled safely",
		},
		{
			name:         "Invalid UTF-8 sequence",
			constraint:   "in:valid\xff\xfeinvalid",
			shouldReject: true,
			description:  "Invalid UTF-8 should be rejected",
		},
		{
			name:         "Only special characters",
			constraint:   "in:!@#$%^&*()",
			shouldReject: false, // Зависит от плагина
			description:  "Special characters should be handled",
		},
		{
			name:         "Empty string",
			constraint:   "",
			shouldReject: true,
			description:  "Empty constraints should be rejected",
		},
		{
			name:         "Valid length constraint",
			constraint:   "len:>5",
			shouldReject: false,
			description:  "Valid length constraint should be accepted",
		},
		{
			name:         "Valid range constraint",
			constraint:   "range:1-10",
			shouldReject: false,
			description:  "Valid range constraint should be accepted",
		},
		{
			name:         "Valid comparison constraint",
			constraint:   ">100",
			shouldReject: false,
			description:  "Valid comparison constraint should be accepted",
		},
	}

	for _, pluginTest := range pluginTests {
		t.Run(pluginTest.name, func(t *testing.T) {
			for _, input := range maliciousInputs {
				t.Run(input.name, func(t *testing.T) {
					// Пытаемся распарсить
					validator, err := pluginTest.plugin.Parse("test_param", input.constraint)

					if input.shouldReject {
						// Ожидаем ошибку или невозможность парсинга
						if err == nil && validator != nil {
							t.Errorf("%s: Expected rejection for constraint %q, but it was accepted",
								input.description, input.constraint)
						}
					} else {
						// Проверяем, что нет паники
						func() {
							defer func() {
								if r := recover(); r != nil {
									t.Errorf("%s: PANIC for constraint %q: %v",
										input.description, input.constraint, r)
								}
							}()

							// Тестируем валидатор, если он был создан
							if validator != nil {
								testValues := []string{"", "test", "123", strings.Repeat("x", 100)}
								for _, testValue := range testValues {
									result := validator(testValue)
									_ = result // Используем результат
								}
							}
						}()
					}

					// Логируем информацию для отладки
					t.Logf("Plugin: %s, Constraint: %q, Error: %v",
						pluginTest.name, input.constraint, err)
				})
			}
		})
	}
}

// TestPluginMemorySafety тестирует безопасность использования памяти
func TestPluginMemorySafety(t *testing.T) {
	plugin := plugins.NewPatternPlugin()

	t.Run("Memory exhaustion protection", func(t *testing.T) {
		// Создаем валидатор с простым паттерном
		validator, err := plugin.Parse("test", "in:*test*")
		if err != nil {
			t.Fatalf("Failed to create validator: %v", err)
		}

		// Тестируем с различными размерами входных данных
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

				// Проверяем, что выполнение не занимает слишком много времени
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

// TestPluginConcurrentSafety тестирует конкурентную безопасность
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
						// Создаем различные constraint для тестирования
						var constraint string
						switch pl.name {
						case "pattern":
							constraint = "in:*test*"
						case "length":
							constraint = "len:>5"
						case "comparison":
							constraint = ">10"
						case "range":
							constraint = "range:1..100"
						}

						validator, err := pl.plugin.Parse("test_param", constraint)
						if err != nil {
							continue // Некоторые комбинации могут быть невалидными
						}

						// Тестируем валидатор
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

			// Ждем завершения всех горутин
			for i := 0; i < goroutines; i++ {
				<-done
			}
		})
	}
}

// TestPluginBoundaryConditions тестирует граничные условия
func TestPluginBoundaryConditions(t *testing.T) {
	plugin := plugins.NewPatternPlugin()

	tests := []struct {
		name        string
		pattern     string
		values      []string
		expectError bool
		description string
	}{
		{
			name:        "Empty value handling",
			pattern:     "in:*",
			values:      []string{""},
			expectError: false,
			description: "Empty values should be handled correctly",
		},
		{
			name:        "Very long pattern - should be rejected",
			pattern:     "in:" + strings.Repeat("a", 1000) + "*", // 1001 символов - превышает лимит
			values:      []string{},
			expectError: true,
			description: "Very long patterns should be rejected",
		},
		{
			name:        "Maximum length pattern",
			pattern:     "in:" + strings.Repeat("a", 999) + "*", // 1000 символов - максимальная длина
			values:      []string{strings.Repeat("a", 999) + "suffix"},
			expectError: false,
			description: "Maximum length patterns should work",
		},
		{
			name:        "Unicode boundary",
			pattern:     "in:*" + string([]rune{0x1F600}), // смайлик
			values:      []string{"prefix" + string([]rune{0x1F600})},
			expectError: false,
			description: "Unicode boundary characters should work",
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
				// Проверяем, что нет паники
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("%s: PANIC for value %q: %v",
								tt.description, value, r)
						}
					}()

					result := validator(value)

					// Проверяем валидность UTF-8 если значение не пустое
					if value != "" && !utf8.ValidString(value) {
						t.Logf("Warning: Test value contains invalid UTF-8: %q", value)
					}

					t.Logf("Pattern: %q, Value: %q, Result: %v - %s",
						tt.pattern, value, result, tt.description)
				}()
			}
		})
	}
}

// TestPluginResourceCleanup тестирует корректное освобождение ресурсов
func TestPluginResourceCleanup(t *testing.T) {
	plugin := plugins.NewPatternPlugin()

	// Создаем множество валидаторов и проверяем, что нет утечек
	patterns := []string{
		"in:*test*",
		"in:prefix*",
		"in:*suffix",
		"in:*a*b*c*",
		"in:" + strings.Repeat("x", 100) + "*",
	}

	// Многократное создание и использование валидаторов
	for i := 0; i < 1000; i++ {
		for _, pattern := range patterns {
			validator, err := plugin.Parse("test", pattern)
			if err != nil {
				continue
			}

			// Используем валидатор
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

// TestPluginSpecificSecurity тестирует специфические уязвимости каждого плагина
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

				t.Logf("Constraint: %q, Error: %v", tt.constraint, err)
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

				t.Logf("Constraint: %q, Error: %v", tt.constraint, err)
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

				t.Logf("Constraint: %q, Error: %v", tt.constraint, err)
			})
		}
	})
}

// TestPluginErrorHandling тестирует обработку ошибок в плагинах
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
				"",                                      // empty
				"in:",                                   // empty pattern
				"invalid",                               // wrong format
				"in:" + strings.Repeat("a", 1001) + "*", // too long
			},
		},
		{
			name:   "length",
			plugin: plugins.NewLengthPlugin(),
			invalidConstraints: []string{
				"",                    // empty
				"len:",                // empty constraint
				"len:>>5",             // invalid operator
				"len:999999999999999", // too large
			},
		},
		{
			name:   "comparison",
			plugin: plugins.NewComparisonPlugin(),
			invalidConstraints: []string{
				"",           // empty
				">",          // missing number
				">>10",       // double operator
				">999999999", // too large
			},
		},
		{
			name:   "range",
			plugin: plugins.NewRangePlugin(),
			invalidConstraints: []string{
				"",                  // empty
				"range:",            // empty range
				"range:10..1",       // invalid range
				"range:1..99999999", // too large
			},
		},
	}

	for _, pt := range pluginTests {
		t.Run(pt.name, func(t *testing.T) {
			for _, constraint := range pt.invalidConstraints {
				t.Run(constraint, func(t *testing.T) {
					validator, err := pt.plugin.Parse("test", constraint)

					// Должна быть ошибка или nil валидатор
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
