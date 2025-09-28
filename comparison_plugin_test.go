package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func TestComparisonPlugin(t *testing.T) {
	plugin := plugins.NewComparisonPlugin()

	tests := []struct {
		name        string
		constraint  string
		value       string
		shouldParse bool
		expected    bool
		shouldError bool
	}{
		// Greater than
		{
			name:        "greater than valid",
			constraint:  ">5",
			value:       "6",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "greater than invalid",
			constraint:  ">5",
			value:       "5",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "greater than equal invalid",
			constraint:  ">5",
			value:       "4",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Greater than or equal
		{
			name:        "greater than or equal valid",
			constraint:  ">=5",
			value:       "5",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "greater than or equal valid higher",
			constraint:  ">=5",
			value:       "6",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "greater than or equal invalid",
			constraint:  ">=5",
			value:       "4",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Less than
		{
			name:        "less than valid",
			constraint:  "<10",
			value:       "9",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "less than invalid",
			constraint:  "<10",
			value:       "10",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "less than equal invalid",
			constraint:  "<10",
			value:       "11",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Less than or equal
		{
			name:        "less than or equal valid",
			constraint:  "<=10",
			value:       "10",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "less than or equal valid lower",
			constraint:  "<=10",
			value:       "9",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "less than or equal invalid",
			constraint:  "<=10",
			value:       "11",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Negative numbers
		{
			name:        "negative numbers valid",
			constraint:  ">-5",
			value:       "-4",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "negative numbers invalid",
			constraint:  ">-5",
			value:       "-6",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "negative range valid",
			constraint:  ">=-100",
			value:       "-50",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},

		// Large numbers
		{
			name:        "large numbers valid",
			constraint:  ">100",
			value:       "150",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "large numbers invalid",
			constraint:  "<1000000",
			value:       "1000001",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Equal boundary
		{
			name:        "equal boundary valid",
			constraint:  ">=0",
			value:       "0",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "equal boundary invalid",
			constraint:  ">=0",
			value:       "-1",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Invalid formats
		{
			name:        "double operator should fail",
			constraint:  ">>10",
			shouldParse: true, // CanParse returns true
			shouldError: true, // But Parse should fail
		},
		{
			name:        "double less than should fail",
			constraint:  "<<100",
			shouldParse: true, // CanParse returns true
			shouldError: true, // But Parse should fail
		},
		{
			name:        "mixed operators should fail",
			constraint:  "><100",
			shouldParse: true, // CanParse returns true
			shouldError: true, // But Parse should fail
		},
		{
			name:        "operator with text should fail",
			constraint:  ">abc",
			shouldParse: true, // CanParse returns true
			shouldError: true, // But Parse should fail
		},
		{
			name:        "empty after operator should fail",
			constraint:  ">",
			shouldParse: true, // CanParse returns true
			shouldError: true, // But Parse should fail
		},
		{
			name:        "operator with equals only should fail",
			constraint:  ">=",
			shouldParse: true, // CanParse returns true
			shouldError: true, // But Parse should fail
		},
		{
			name:        "empty constraint",
			constraint:  "",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "very large number should fail",
			constraint:  ">9999999999",
			shouldParse: true, // CanParse returns true
			shouldError: true, // But Parse should fail
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

func TestComparisonPluginIntegration(t *testing.T) {
	// Создаем парсер и явно регистрируем плагин
	comparisonPlugin := plugins.NewComparisonPlugin()
	parser := NewRuleParser(comparisonPlugin)

	tests := []struct {
		name     string
		rule     string
		value    string
		expected bool
	}{
		{
			name:     "greater than in param rule",
			rule:     "age=[>18]",
			value:    "25",
			expected: true,
		},
		{
			name:     "greater than in param rule too young",
			rule:     "age=[>18]",
			value:    "16",
			expected: false,
		},
		{
			name:     "less than in param rule",
			rule:     "price=[<1000]",
			value:    "500",
			expected: true,
		},
		{
			name:     "less than in param rule too expensive",
			rule:     "price=[<1000]",
			value:    "1500",
			expected: false,
		},
		{
			name:     "greater or equal in param rule",
			rule:     "score=[>=50]",
			value:    "50",
			expected: true,
		},
		{
			name:     "greater or equal in param rule below",
			rule:     "score=[>=50]",
			value:    "49",
			expected: false,
		},
		{
			name:     "less or equal in param rule",
			rule:     "quantity=[<=10]",
			value:    "10",
			expected: true,
		},
		{
			name:     "less or equal in param rule above",
			rule:     "quantity=[<=10]",
			value:    "11",
			expected: false,
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

func BenchmarkComparisonPlugin(b *testing.B) {
	plugin := plugins.NewComparisonPlugin()
	validator, err := plugin.Parse("test", ">100")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator("150")
	}
}

func BenchmarkComparisonPluginCanParse(b *testing.B) {
	plugin := plugins.NewComparisonPlugin()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.CanParse(">100")
	}
}

func BenchmarkComparisonPluginParse(b *testing.B) {
	plugin := plugins.NewComparisonPlugin()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.Parse("test", ">100")
	}
}
