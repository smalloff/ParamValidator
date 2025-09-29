package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func TestPatternPlugin(t *testing.T) {
	plugin := plugins.NewPatternPlugin()

	tests := []struct {
		name        string
		constraint  string
		value       string
		shouldParse bool
		expected    bool
		shouldError bool
	}{
		// Префиксные паттерны
		{
			name:        "prefix pattern match",
			constraint:  "start*",
			value:       "start_value",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "prefix pattern exact match",
			constraint:  "start*",
			value:       "start",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "prefix pattern no match",
			constraint:  "start*",
			value:       "wrong_start",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "prefix pattern empty value",
			constraint:  "start*",
			value:       "",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Суффиксные паттерны
		{
			name:        "suffix pattern match",
			constraint:  "*end",
			value:       "value_end",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "suffix pattern exact match",
			constraint:  "*end",
			value:       "end",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "suffix pattern no match",
			constraint:  "*end",
			value:       "end_wrong",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Паттерны содержания
		{
			name:        "contains pattern match",
			constraint:  "*val*",
			value:       "some_val_here",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "contains pattern exact match",
			constraint:  "*val*",
			value:       "val",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "contains pattern no match",
			constraint:  "*val*",
			value:       "nothing",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Множественные части
		{
			name:        "multiple parts match",
			constraint:  "*one*two*three*",
			value:       "blablaoneblablatwoblathreeblabla",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "multiple parts exact match",
			constraint:  "*one*two*three*",
			value:       "onetwothree",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "multiple parts partial match",
			constraint:  "*one*two*three*",
			value:       "one_two",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Любая строка
		{
			name:        "any string match",
			constraint:  "*",
			shouldParse: true, // Есть wildcard - парсится
			expected:    true,
			shouldError: false,
		},
		{
			name:        "any string empty",
			constraint:  "*",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},

		// Сложные паттерны
		{
			name:        "complex pattern match",
			constraint:  "pre*mid*post",
			value:       "pre123mid456post",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "complex pattern no match",
			constraint:  "pre*mid*post",
			value:       "pre123mid456",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Невалидные форматы
		{
			name:        "empty constraint",
			constraint:  "",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "only wildcard",
			constraint:  "*",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "multiple wildcards only",
			constraint:  "**",
			shouldParse: true,
			expected:    true,
			shouldError: false,
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

func TestPatternPluginIntegration(t *testing.T) {
	// Создаем парсер и явно регистрируем плагин
	patternPlugin := plugins.NewPatternPlugin()
	parser := NewRuleParser(patternPlugin)

	tests := []struct {
		name     string
		rule     string
		value    string
		expected bool
	}{
		{
			name:     "prefix in param rule",
			rule:     "file=[img_*]",
			value:    "img_photo.jpg",
			expected: true,
		},
		{
			name:     "prefix in param rule no match",
			rule:     "file=[img_*]",
			value:    "doc_file.pdf",
			expected: false,
		},
		{
			name:     "suffix in param rule",
			rule:     "file=[*.jpg]",
			value:    "photo.jpg",
			expected: true,
		},
		{
			name:     "suffix in param rule no match",
			rule:     "file=[*.jpg]",
			value:    "document.pdf",
			expected: false,
		},
		{
			name:     "contains in param rule",
			rule:     "id=[*user*]",
			value:    "new_user_123",
			expected: true,
		},
		{
			name:     "contains in param rule no match",
			rule:     "id=[*user*]",
			value:    "admin_123",
			expected: false,
		},
		{
			name:     "complex pattern in param rule",
			rule:     "key=[prefix_*_suffix]",
			value:    "prefix_value_suffix",
			expected: true,
		},
		{
			name:     "complex pattern in param rule no match",
			rule:     "key=[prefix_*_suffix]",
			value:    "prefix_value",
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

func TestPatternEdgeCases(t *testing.T) {
	plugin := plugins.NewPatternPlugin()

	tests := []struct {
		name        string
		constraint  string
		shouldParse bool
		shouldError bool
	}{
		{
			name:        "valid prefix pattern",
			constraint:  "start*",
			shouldParse: true,
			shouldError: false,
		},
		{
			name:        "valid suffix pattern",
			constraint:  "*end",
			shouldParse: true,
			shouldError: false,
		},
		{
			name:        "valid contains pattern",
			constraint:  "*val*",
			shouldParse: true,
			shouldError: false,
		},
		{
			name:        "any string pattern",
			constraint:  "*",
			shouldParse: true,
			shouldError: false,
		},
		{
			name:        "multiple wildcards only",
			constraint:  "**",
			shouldParse: true,
			shouldError: false,
		},
		{
			name:        "empty constraint should not parse",
			constraint:  "",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "complex multiple parts",
			constraint:  "*one*two*three*",
			shouldParse: true,
			shouldError: false,
		},
		{
			name:        "pattern with special characters",
			constraint:  "*.*+?[](){}|^$\\*",
			shouldParse: true,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canParse := plugin.CanParse(tt.constraint)
			if canParse != tt.shouldParse {
				t.Errorf("CanParse(%q) = %v, expected %v",
					tt.constraint, canParse, tt.shouldParse)
			}

			_, err := plugin.Parse("test", tt.constraint)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Parse(%q) should fail but succeeded", tt.constraint)
				}
			} else {
				if err != nil {
					t.Errorf("Parse(%q) failed but should succeed: %v", tt.constraint, err)
				}
			}
		})
	}
}

func BenchmarkPatternPlugin(b *testing.B) {
	plugin := plugins.NewPatternPlugin()
	validator, err := plugin.Parse("test", "img_*")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator("img_photo.jpg")
	}
}

func BenchmarkPatternPluginCanParse(b *testing.B) {
	plugin := plugins.NewPatternPlugin()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.CanParse("img_*")
	}
}

func BenchmarkPatternPluginParse(b *testing.B) {
	plugin := plugins.NewPatternPlugin()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.Parse("test", "img_*")
	}
}

func BenchmarkPatternPluginNormalization(b *testing.B) {
	patternPlugin := plugins.NewPatternPlugin()
	pv, err := NewParamValidator("", WithPlugins(patternPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?file=[img_*]&id=[*user*]")
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.NormalizeURL("/api?file=img_photo.jpg&id=new_user_123&invalid=value")
	}
}

func BenchmarkPatternPluginFilterQuery(b *testing.B) {
	patternPlugin := plugins.NewPatternPlugin()

	pv, err := NewParamValidator("", WithPlugins(patternPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?file=[img_*]&id=[*user*]")

	if err != nil {
		b.Fatalf(
			"Failed to parse rules: %v",
			err,
		)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterQuery("/api", "file=img_photo.jpg&id=new_user_123&invalid=value")
	}
}

func BenchmarkPatternPluginValidateQuery(b *testing.B) {
	patternPlugin := plugins.NewPatternPlugin()
	pv, err := NewParamValidator("", WithPlugins(patternPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	pv.initialized.Store(true)
	err = pv.ParseRules("/api?file=[img_*]&id=[*user*]")
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateQuery("/api", "file=img_photo.jpg&id=new_user_123&invalid=value")
	}
}
