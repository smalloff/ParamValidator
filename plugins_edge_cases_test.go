// plugins_edge_cases_test.go
package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

// plugins_edge_cases_test.go - в функции TestPluginEdgeCases
func TestPluginEdgeCases(t *testing.T) {
	parser := NewRuleParser(plugins.NewComparisonPlugin(), plugins.NewRegexPlugin())

	tests := []struct {
		name        string
		rule        string
		shouldError bool
	}{
		{
			name:        "empty constraint in brackets",
			rule:        "param=[]",
			shouldError: false, // Это должно быть key-only pattern, не плагин
		},
		{
			name:        "question mark constraint",
			rule:        "param=[?]",
			shouldError: false, // Это должно быть callback pattern, не плагин
		},
		{
			name:        "very large number for comparison",
			rule:        "param=[>999999999999]",
			shouldError: false,
		},
		{
			name:        "complex regex with special characters",
			rule:        "param=[/^[\\w\\-\\.]+@([\\w-]+\\.)+[\\w-]{2,}$/]",
			shouldError: false,
		},
		{
			name:        "multiple operators should fail",
			rule:        "param=[>>100]",
			shouldError: true, // Parse должен вернуть ошибку
		},
		{
			name:        "invalid operator combination",
			rule:        "param=[<>100]",
			shouldError: true, // Parse должен вернуть ошибку
		},
		{
			name:        "operator without number",
			rule:        "param=[>]",
			shouldError: true, // Parse должен вернуть ошибку
		},
		{
			name:        "just operator symbols",
			rule:        "param=[><]",
			shouldError: true, // Parse должен вернуть ошибку
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.parseSingleParamRuleUnsafe(tt.rule)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for rule %q, but got none", tt.rule)
				} else {
					t.Logf("Correctly got error for rule %q: %v", tt.rule, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for rule %q: %v", tt.rule, err)
				}
			}
		})
	}
}

// Дополнительный тест для специфических случаев сравнения
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
		},
		{
			name:        "valid less than",
			constraint:  "<100",
			shouldParse: true,
		},
		{
			name:        "valid less than or equal",
			constraint:  "<=100",
			shouldParse: true,
		},
		{
			name:        "double greater than should fail",
			constraint:  ">>100",
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
			name:        "negative number valid",
			constraint:  ">-100",
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

			validator, err := plugin.Parse("test", tt.constraint)

			if tt.shouldError {
				// Ожидаем ошибку
				if err == nil {
					t.Errorf("Parse(%q) should fail but succeeded", tt.constraint)
				}
			} else {
				// Ожидаем успех
				if err != nil {
					t.Errorf("Parse(%q) failed but should succeed: %v", tt.constraint, err)
				} else if validator == nil {
					t.Errorf("Parse(%q) returned nil validator", tt.constraint)
				}
			}
		})
	}
}

// Тест для проверки что стандартные правила не ломаются
func TestStandardRulesNotBroken(t *testing.T) {
	parser := NewRuleParser(plugins.NewComparisonPlugin(), plugins.NewRegexPlugin())

	// Эти правила должны обрабатываться стандартным парсером, не плагинами
	standardRules := []string{
		"param=[1-10]",  // range
		"param=[a,b,c]", // enum
		"param=[]",      // key-only
		"param",         // any
		"param=[?]",     // callback
	}

	for _, rule := range standardRules {
		t.Run(rule, func(t *testing.T) {
			paramRule, err := parser.parseSingleParamRuleUnsafe(rule)
			if err != nil {
				t.Errorf("Failed to parse standard rule %q: %v", rule, err)
				return
			}

			// Стандартные правила не должны использовать плагины
			if paramRule.Pattern == "plugin" {
				t.Errorf("Standard rule %q should not use plugin pattern", rule)
			}
		})
	}
}
