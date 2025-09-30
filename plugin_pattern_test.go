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
	}{
		// Префиксные паттерны
		{
			name:        "prefix pattern match",
			constraint:  "in:start*",
			value:       "start_value",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "prefix pattern exact match",
			constraint:  "in:start*",
			value:       "start",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "prefix pattern no match",
			constraint:  "in:start*",
			value:       "wrong_start",
			shouldParse: true,
			expected:    false,
		},
		{
			name:        "prefix pattern empty value",
			constraint:  "in:start*",
			value:       "",
			shouldParse: true,
			expected:    false,
		},

		// Суффиксные паттерны
		{
			name:        "suffix pattern match",
			constraint:  "in:*end",
			value:       "value_end",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "suffix pattern exact match",
			constraint:  "in:*end",
			value:       "end",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "suffix pattern no match",
			constraint:  "in:*end",
			value:       "end_wrong",
			shouldParse: true,
			expected:    false,
		},

		// Паттерны содержания
		{
			name:        "contains pattern match",
			constraint:  "in:*val*",
			value:       "some_val_here",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "contains pattern exact match",
			constraint:  "in:*val*",
			value:       "val",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "contains pattern no match",
			constraint:  "in:*val*",
			value:       "nothing",
			shouldParse: true,
			expected:    false,
		},

		// Множественные части
		{
			name:        "multiple parts match",
			constraint:  "in:*one*two*three*",
			value:       "blablaoneblablatwoblathreeblabla",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "multiple parts exact match",
			constraint:  "in:*one*two*three*",
			value:       "onetwothree",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "multiple parts partial match",
			constraint:  "in:*one*two*three*",
			value:       "one_two",
			shouldParse: true,
			expected:    false,
		},

		// Любая строка
		{
			name:        "any string match",
			constraint:  "in:*",
			value:       "any_value",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "any string empty",
			constraint:  "in:*",
			value:       "",
			shouldParse: true,
			expected:    true,
		},

		// Сложные паттерны
		{
			name:        "complex pattern match",
			constraint:  "in:pre*mid*post",
			value:       "pre123mid456post",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "complex pattern no match",
			constraint:  "in:pre*mid*post",
			value:       "pre123mid456",
			shouldParse: true,
			expected:    false,
		},

		// Невалидные форматы
		{
			name:        "empty constraint",
			constraint:  "",
			shouldParse: false,
		},
		{
			name:        "only prefix without pattern",
			constraint:  "in:",
			shouldParse: false,
		},
		{
			name:        "only wildcard",
			constraint:  "in:*",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "multiple wildcards only",
			constraint:  "in:**",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "too long pattern",
			constraint:  "in:a*" + string(make([]byte, 1001)), // Создаем слишком длинный паттерн
			shouldParse: false,
		},
		{
			name:        "pattern without wildcard",
			constraint:  "in:nowildcard",
			shouldParse: false,
		},
		{
			name:        "wrong prefix",
			constraint:  "len:*",
			shouldParse: false,
		},
		{
			name:        "wrong prefix range",
			constraint:  "range*",
			shouldParse: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			rule:     "file=[in:img_*]",
			value:    "img_photo.jpg",
			expected: true,
		},
		{
			name:     "prefix in param rule no match",
			rule:     "file=[in:img_*]",
			value:    "doc_file.pdf",
			expected: false,
		},
		{
			name:     "suffix in param rule",
			rule:     "file=[in:*.jpg]",
			value:    "photo.jpg",
			expected: true,
		},
		{
			name:     "suffix in param rule no match",
			rule:     "file=[in:*.jpg]",
			value:    "document.pdf",
			expected: false,
		},
		{
			name:     "contains in param rule",
			rule:     "id=[in:*user*]",
			value:    "new_user_123",
			expected: true,
		},
		{
			name:     "contains in param rule no match",
			rule:     "id=[in:*user*]",
			value:    "admin_123",
			expected: false,
		},
		{
			name:     "complex pattern in param rule",
			rule:     "key=[in:prefix_*_suffix]",
			value:    "prefix_value_suffix",
			expected: true,
		},
		{
			name:     "complex pattern in param rule no match",
			rule:     "key=[in:prefix_*_suffix]",
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
	}{
		{
			name:        "valid prefix pattern",
			constraint:  "in:start*",
			shouldParse: true,
		},
		{
			name:        "valid suffix pattern",
			constraint:  "in:*end",
			shouldParse: true,
		},
		{
			name:        "valid contains pattern",
			constraint:  "in:*val*",
			shouldParse: true,
		},
		{
			name:        "any string pattern",
			constraint:  "in:*",
			shouldParse: true,
		},
		{
			name:        "multiple wildcards only",
			constraint:  "in:**",
			shouldParse: true,
		},
		{
			name:        "empty constraint should not parse",
			constraint:  "",
			shouldParse: false,
		},
		{
			name:        "only prefix should not parse",
			constraint:  "in:",
			shouldParse: false,
		},
		{
			name:        "complex multiple parts",
			constraint:  "in:*one*two*three*",
			shouldParse: true,
		},
		{
			name:        "pattern with special characters",
			constraint:  "in:*.*+?[](){}|^$\\*",
			shouldParse: true,
		},
		{
			name:        "pattern without wildcard should not parse",
			constraint:  "in:nowildcard",
			shouldParse: false,
		},
		{
			name:        "too long pattern should not parse",
			constraint:  "in:a*" + string(make([]byte, 1001)),
			shouldParse: false,
		},
		{
			name:        "wrong prefix should not parse",
			constraint:  "len:*",
			shouldParse: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := plugin.Parse("test", tt.constraint)

			if tt.shouldParse {
				if err != nil {
					t.Errorf("Parse(%q) failed: %v", tt.constraint, err)
				} else if validator == nil {
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

// Бенчмарки - убираем CanParse бенчмарк
func BenchmarkPatternPlugin(b *testing.B) {
	plugin := plugins.NewPatternPlugin()
	validator, err := plugin.Parse("test", "in:img_*")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator("img_photo.jpg")
	}
}

func BenchmarkPatternPluginParse(b *testing.B) {
	plugin := plugins.NewPatternPlugin()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.Parse("test", "in:img_*")
	}
}

func BenchmarkPatternPluginNormalization(b *testing.B) {
	patternPlugin := plugins.NewPatternPlugin()
	pv, err := NewParamValidator("", WithPlugins(patternPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?file=[in:img_*]&id=[in:*user*]")
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterURL("/api?file=img_photo.jpg&id=new_user_123&invalid=value")
	}
}

func BenchmarkPatternPluginFilterQuery(b *testing.B) {
	patternPlugin := plugins.NewPatternPlugin()

	pv, err := NewParamValidator("", WithPlugins(patternPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?file=[in:img_*]&id=[in:*user*]")

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
	err = pv.ParseRules("/api?file=[in:img_*]&id=[in:*user*]")
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateQuery("/api", "file=img_photo.jpg&id=new_user_123&invalid=value")
	}
}
