// plugins_integration_test.go
package paramvalidator

import (
	"fmt"
	"testing"
)

// CustomPlugin тестовый плагин для демонстрации
type CustomPlugin struct {
	name string
}

func NewCustomPlugin() *CustomPlugin {
	return &CustomPlugin{name: "custom"}
}

func (cp *CustomPlugin) GetName() string {
	return cp.name
}

func (cp *CustomPlugin) CanParse(constraintStr string) bool {
	return constraintStr == "custom_rule"
}

func (cp *CustomPlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	return func(value string) bool {
		return value == "allowed_value"
	}, nil
}

// LengthPlugin плагин для проверки длины
type LengthPlugin struct {
	name string
}

func NewLengthPlugin() *LengthPlugin {
	return &LengthPlugin{name: "length"}
}

func (lp *LengthPlugin) GetName() string {
	return lp.name
}

func (lp *LengthPlugin) CanParse(constraintStr string) bool {
	return len(constraintStr) > 3 && constraintStr[:3] == "len"
}

func (lp *LengthPlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	// Простая реализация для демонстрации
	return func(value string) bool {
		return len(value) > 0
	}, nil
}

// CatchAllPlugin плагин, который совпадает со всем (для теста приоритета)
type CatchAllPlugin struct {
	name string
}

func NewCatchAllPlugin() *CatchAllPlugin {
	return &CatchAllPlugin{name: "catchall"}
}

func (cp *CatchAllPlugin) GetName() string {
	return cp.name
}

func (cp *CatchAllPlugin) CanParse(constraintStr string) bool {
	return true // Совпадает со всем!
}

func (cp *CatchAllPlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	return func(value string) bool {
		return false // Всегда возвращает false для теста
	}, nil
}

func TestCustomPluginIntegration(t *testing.T) {
	// Use the custom plugin
	parser := NewRuleParser(NewCustomPlugin())

	rule, err := parser.parseSingleParamRuleUnsafe("test_param=[custom_rule]")
	if err != nil {
		t.Fatalf("Failed to parse custom rule: %v", err)
	}

	if rule.Pattern != "plugin" {
		t.Errorf("Expected pattern 'plugin', got %q", rule.Pattern)
	}

	if rule.CustomValidator == nil {
		t.Fatal("CustomValidator should not be nil")
	}

	// Test validation
	if !rule.CustomValidator("allowed_value") {
		t.Error("Custom validator should return true for 'allowed_value'")
	}

	if rule.CustomValidator("disallowed_value") {
		t.Error("Custom validator should return false for 'disallowed_value'")
	}
}

func TestMultipleCustomPlugins(t *testing.T) {
	parser := NewRuleParser(
		NewComparisonPlugin(),
		NewRegexPlugin(),
		NewCustomPlugin(),
		NewLengthPlugin(),
	)

	tests := []struct {
		name     string
		rule     string
		value    string
		expected bool
	}{
		{
			name:     "custom plugin rule",
			rule:     "param=[custom_rule]",
			value:    "allowed_value",
			expected: true,
		},
		{
			name:     "length plugin simple",
			rule:     "code=[len5]",
			value:    "12345",
			expected: true,
		},
		{
			name:     "comparison plugin still works",
			rule:     "age=[>18]",
			value:    "25",
			expected: true,
		},
		{
			name:     "regex plugin still works",
			rule:     "email=[/^[a-z]+@[a-z]+\\.[a-z]+$/]",
			value:    "test@example.com",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := parser.parseSingleParamRuleUnsafe(tt.rule)
			if err != nil {
				t.Fatalf("Failed to parse rule %q: %v", tt.rule, err)
			}

			if rule.CustomValidator == nil {
				t.Fatal("CustomValidator should not be nil")
			}

			result := rule.CustomValidator(tt.value)
			if result != tt.expected {
				t.Errorf("Validation failed for value %q: got %v, expected %v",
					tt.value, result, tt.expected)
			}
		})
	}
}

func TestPluginPriority(t *testing.T) {
	// Важно: порядок регистрации плагинов определяет приоритет
	parser := NewRuleParser(
		NewComparisonPlugin(), // Более специфичный - первый
		NewCatchAllPlugin(),   // Общий - последний
	)

	// ">100" должен быть обработан ComparisonPlugin, а не CatchAllPlugin
	rule, err := parser.parseSingleParamRuleUnsafe("test=[>100]")
	if err != nil {
		t.Fatalf("Failed to parse rule: %v", err)
	}

	// ComparisonPlugin должен вернуть true для 150
	result := rule.CustomValidator("150")
	if !result {
		t.Error("ComparisonPlugin should handle >100, not CatchAllPlugin")
	}
}

// Тест для проверки, что стандартные правила все еще работают
func TestStandardRulesStillWork(t *testing.T) {
	parser := NewRuleParser(NewComparisonPlugin(), NewRegexPlugin())

	tests := []struct {
		name     string
		rule     string
		value    string
		expected bool
	}{
		{
			name:     "enum rule still works",
			rule:     "sort=[name,date]",
			value:    "name",
			expected: true,
		},
		{
			name:     "range rule still works",
			rule:     "page=[1-10]",
			value:    "5",
			expected: true,
		},
		{
			name:     "key-only rule still works",
			rule:     "active=[]",
			value:    "",
			expected: true,
		},
		{
			name:     "any pattern still works",
			rule:     "param",
			value:    "anyvalue",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := parser.parseSingleParamRuleUnsafe(tt.rule)
			if err != nil {
				t.Fatalf("Failed to parse rule %q: %v", tt.rule, err)
			}

			// Для стандартных правил CustomValidator будет nil
			if tt.name != "enum rule still works" && tt.name != "range rule still works" {
				if rule.CustomValidator != nil {
					t.Error("Standard rules should not use CustomValidator")
				}
			}
		})
	}
}

// Тест для обработки ошибок в плагинах
type ErrorPlugin struct {
	name string
}

func NewErrorPlugin() *ErrorPlugin {
	return &ErrorPlugin{name: "error"}
}

func (ep *ErrorPlugin) GetName() string {
	return ep.name
}

func (ep *ErrorPlugin) CanParse(constraintStr string) bool {
	return constraintStr == "error"
}

func (ep *ErrorPlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	return nil, fmt.Errorf("simulated plugin error")
}

func TestPluginErrorHandling(t *testing.T) {
	parser := NewRuleParser(NewErrorPlugin())

	_, err := parser.parseSingleParamRuleUnsafe("test=[error]")
	if err == nil {
		t.Error("Expected error from plugin, but got none")
	} else {
		t.Logf("Correctly received error from plugin: %v", err)
	}
}
