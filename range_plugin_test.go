package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func TestRangePlugin(t *testing.T) {
	plugin := plugins.NewRangePlugin()

	tests := []struct {
		name        string
		constraint  string
		value       string
		shouldParse bool
		expected    bool
		shouldError bool
	}{
		// Valid ranges with hyphen
		{
			name:        "basic range valid",
			constraint:  "1-10",
			value:       "5",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "basic range lower bound",
			constraint:  "1-10",
			value:       "1",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "basic range upper bound",
			constraint:  "1-10",
			value:       "10",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "basic range below min",
			constraint:  "1-10",
			value:       "0",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "basic range above max",
			constraint:  "1-10",
			value:       "11",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Valid ranges with dots
		{
			name:        "dots range valid",
			constraint:  "1..10",
			value:       "5",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "dots range invalid",
			constraint:  "1..10",
			value:       "15",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Negative numbers
		{
			name:        "negative range valid",
			constraint:  "-10..10",
			value:       "-5",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "negative range invalid",
			constraint:  "-10..10",
			value:       "-15",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "all negative range",
			constraint:  "-50..-10",
			value:       "-25",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},

		// Large numbers
		{
			name:        "large range valid",
			constraint:  "1000-9999",
			value:       "5000",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},

		// Invalid formats
		{
			name:        "empty constraint",
			constraint:  "",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "single number",
			constraint:  "5",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "triple numbers",
			constraint:  "1-10-100",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "text instead of numbers",
			constraint:  "a-z",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "mixed text numbers",
			constraint:  "1-abc",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "min greater than max",
			constraint:  "10-1",
			shouldParse: true, // CanParse проходит
			shouldError: true, // Но Parse должен вернуть ошибку
		},
		{
			name:        "empty min value",
			constraint:  "-10",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "empty max value",
			constraint:  "10-",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "enum format should not parse",
			constraint:  "a,b,c",
			shouldParse: false,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test CanParse
			canParse := plugin.CanParse(tt.constraint)
			if canParse != tt.shouldParse {
				t.Errorf("CanParse(%q) = %v, expected %v",
					tt.constraint, canParse, tt.shouldParse)
				return
			}

			if !tt.shouldParse {
				// Если не должен парситься, проверяем что Parse возвращает ошибку
				_, err := plugin.Parse("test_param", tt.constraint)
				if err == nil {
					t.Errorf("Parse(%q) should fail for non-parsable constraint", tt.constraint)
				}
				return
			}

			// Test Parse
			validator, err := plugin.Parse("test_param", tt.constraint)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Parse(%q) should have failed but succeeded", tt.constraint)
				} else {
					t.Logf("Correctly got error for %q: %v", tt.constraint, err)
				}
				return
			} else {
				if err != nil {
					t.Errorf("Parse(%q) failed: %v", tt.constraint, err)
					return
				}
			}

			// Test validation
			result := validator(tt.value)
			if result != tt.expected {
				t.Errorf("Validator(%q) for constraint %q = %v, expected %v",
					tt.value, tt.constraint, result, tt.expected)
			}
		})
	}
}

func TestRangePluginIntegration(t *testing.T) {
	// Создаем парсер и явно регистрируем плагин
	rangePlugin := plugins.NewRangePlugin()
	parser := NewRuleParser(rangePlugin)

	tests := []struct {
		name     string
		rule     string
		value    string
		expected bool
	}{
		{
			name:     "range in param rule",
			rule:     "age=[18-65]",
			value:    "25",
			expected: true,
		},
		{
			name:     "range in param rule too young",
			rule:     "age=[18-65]",
			value:    "16",
			expected: false,
		},
		{
			name:     "range in param rule too old",
			rule:     "age=[18-65]",
			value:    "70",
			expected: false,
		},
		{
			name:     "dots range in param rule",
			rule:     "price=[100..1000]",
			value:    "500",
			expected: true,
		},
		{
			name:     "negative range in param rule",
			rule:     "temperature=[-20..40]",
			value:    "25",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Парсим полные правила
			globalParams, urlRules, err := parser.parseRulesUnsafe(tt.rule)
			if err != nil {
				t.Fatalf("Failed to parse rule %q: %v", tt.rule, err)
			}

			// Ищем правило параметра
			var paramRule *ParamRule
			for _, rule := range globalParams {
				if rule != nil {
					paramRule = rule
					break
				}
			}

			if paramRule == nil {
				// Проверяем URL rules
				for _, urlRule := range urlRules {
					for _, rule := range urlRule.Params {
						if rule != nil {
							paramRule = rule
							break
						}
					}
					if paramRule != nil {
						break
					}
				}
			}

			if paramRule == nil {
				t.Fatalf("No parameter rule found for %q", tt.rule)
			}

			if paramRule.Pattern != "plugin" {
				t.Errorf("Expected pattern 'plugin', got %q", paramRule.Pattern)
			}

			if paramRule.CustomValidator == nil {
				t.Fatal("CustomValidator should not be nil")
			}

			result := paramRule.CustomValidator(tt.value)
			if result != tt.expected {
				t.Errorf("Validation failed for value %q: got %v, expected %v",
					tt.value, result, tt.expected)
			}
		})
	}
}

func BenchmarkRangePlugin(b *testing.B) {
	plugin := plugins.NewRangePlugin()
	validator, err := plugin.Parse("test", "1-100")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator("50")
	}
}

func BenchmarkRangePluginCanParse(b *testing.B) {
	plugin := plugins.NewRangePlugin()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.CanParse("1-100")
	}
}
