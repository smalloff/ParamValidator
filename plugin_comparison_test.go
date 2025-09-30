package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func TestComparisonEdgeCases(t *testing.T) {
	plugin := plugins.NewComparisonPlugin()

	tests := []struct {
		name        string
		constraint  string
		shouldParse bool
		shouldError bool
	}{
		{
			name:        "valid greater than",
			constraint:  ">100",
			shouldParse: true,
			shouldError: false,
		},
		{
			name:        "valid greater than or equal",
			constraint:  ">=100",
			shouldParse: true,
			shouldError: false,
		},
		{
			name:        "valid less than",
			constraint:  "<100",
			shouldParse: true,
			shouldError: false,
		},
		{
			name:        "valid less than or equal",
			constraint:  "<=100",
			shouldParse: true,
			shouldError: false,
		},
		{
			name:        "double greater than should fail",
			constraint:  ">>100",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "double less than should fail",
			constraint:  "<<100",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "mixed operators should fail",
			constraint:  "><100",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "operator with text should fail",
			constraint:  ">abc",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "empty after operator should fail",
			constraint:  ">",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "operator with equals only should fail",
			constraint:  ">=",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "negative number valid",
			constraint:  ">-100",
			shouldParse: true,
			shouldError: false,
		},
		{
			name:        "not comparison format",
			constraint:  "abc123",
			shouldParse: false,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := plugin.Parse("test", tt.constraint)

			if tt.shouldParse {
				if err != nil {
					t.Errorf("Parse(%q) failed but should succeed: %v", tt.constraint, err)
				} else if validator == nil {
					t.Errorf("Parse(%q) returned nil validator", tt.constraint)
				}
			} else {
				if err == nil {
					t.Errorf("Parse(%q) should fail but succeeded", tt.constraint)
				}
			}
		})
	}
}

func TestComparisonPlugin(t *testing.T) {
	plugin := plugins.NewComparisonPlugin()

	tests := []struct {
		name        string
		constraint  string
		value       string
		shouldParse bool
		expected    bool
	}{
		// Greater than
		{
			name:        "greater than valid",
			constraint:  ">5",
			value:       "6",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "greater than invalid",
			constraint:  ">5",
			value:       "5",
			shouldParse: true,
			expected:    false,
		},
		{
			name:        "greater than equal invalid",
			constraint:  ">5",
			value:       "4",
			shouldParse: true,
			expected:    false,
		},

		// Greater than or equal
		{
			name:        "greater than or equal valid",
			constraint:  ">=5",
			value:       "5",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "greater than or equal valid higher",
			constraint:  ">=5",
			value:       "6",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "greater than or equal invalid",
			constraint:  ">=5",
			value:       "4",
			shouldParse: true,
			expected:    false,
		},

		// Less than
		{
			name:        "less than valid",
			constraint:  "<10",
			value:       "9",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "less than invalid",
			constraint:  "<10",
			value:       "10",
			shouldParse: true,
			expected:    false,
		},
		{
			name:        "less than equal invalid",
			constraint:  "<10",
			value:       "11",
			shouldParse: true,
			expected:    false,
		},

		// Less than or equal
		{
			name:        "less than or equal valid",
			constraint:  "<=10",
			value:       "10",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "less than or equal valid lower",
			constraint:  "<=10",
			value:       "9",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "less than or equal invalid",
			constraint:  "<=10",
			value:       "11",
			shouldParse: true,
			expected:    false,
		},

		// Negative numbers
		{
			name:        "negative numbers valid",
			constraint:  ">-5",
			value:       "-4",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "negative numbers invalid",
			constraint:  ">-5",
			value:       "-6",
			shouldParse: true,
			expected:    false,
		},
		{
			name:        "negative range valid",
			constraint:  ">=-100",
			value:       "-50",
			shouldParse: true,
			expected:    true,
		},

		// Large numbers
		{
			name:        "large numbers valid",
			constraint:  ">100",
			value:       "150",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "large numbers invalid",
			constraint:  "<1000000",
			value:       "1000001",
			shouldParse: true,
			expected:    false,
		},

		// Equal boundary
		{
			name:        "equal boundary valid",
			constraint:  ">=0",
			value:       "0",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "equal boundary invalid",
			constraint:  ">=0",
			value:       "-1",
			shouldParse: true,
			expected:    false,
		},

		// Invalid formats
		{
			name:        "double operator should fail",
			constraint:  ">>10",
			shouldParse: false,
		},
		{
			name:        "double less than should fail",
			constraint:  "<<100",
			shouldParse: false,
		},
		{
			name:        "mixed operators should fail",
			constraint:  "><100",
			shouldParse: false,
		},
		{
			name:        "operator with text should fail",
			constraint:  ">abc",
			shouldParse: false,
		},
		{
			name:        "empty after operator should fail",
			constraint:  ">",
			shouldParse: false,
		},
		{
			name:        "operator with equals only should fail",
			constraint:  ">=",
			shouldParse: false,
		},
		{
			name:        "empty constraint",
			constraint:  "",
			shouldParse: false,
		},
		{
			name:        "very large number should fail",
			constraint:  ">9999999999",
			shouldParse: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Parse
			validator, err := plugin.Parse("test_param", tt.constraint)

			if tt.shouldParse {
				if err != nil {
					t.Errorf("Parse(%q) failed: %v", tt.constraint, err)
					return
				}

				// Test validation if we have a validator
				if validator != nil {
					result := validator(tt.value)
					if result != tt.expected {
						t.Errorf("Validator(%q) for constraint %q = %v, expected %v",
							tt.value, tt.constraint, result, tt.expected)
					}
				} else {
					t.Errorf("Parse(%q) returned nil validator", tt.constraint)
				}
			} else {
				if err == nil {
					t.Errorf("Parse(%q) should have failed but succeeded", tt.constraint)
				}
			}
		})
	}
}

// Остальные тесты остаются без изменений...
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

// Убираем бенчмарк CanParse
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

func BenchmarkComparisonPluginParse(b *testing.B) {
	plugin := plugins.NewComparisonPlugin()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.Parse("test", ">100")
	}
}

func BenchmarkComparisonPluginNormalization(b *testing.B) {
	comparisonPlugin := plugins.NewComparisonPlugin()
	pv, err := NewParamValidator("", WithPlugins(comparisonPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?score=[>50]&quantity=[<=10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterURL("/api?score=75&quantity=5&invalid=value")
	}
}

func BenchmarkComparisonPluginFilterQuery(b *testing.B) {
	comparisonPlugin := plugins.NewComparisonPlugin()
	pv, err := NewParamValidator("", WithPlugins(comparisonPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?score=[>50]&quantity=[<=10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterQuery("/api", "score=75&quantity=5&invalid=value")
	}
}

func BenchmarkComparisonPluginValidateQuery(b *testing.B) {
	comparisonPlugin := plugins.NewComparisonPlugin()
	pv, err := NewParamValidator("", WithPlugins(comparisonPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?score=[>50]&quantity=[<=10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateQuery("/api", "score=75&quantity=5&invalid=value")
	}
}
