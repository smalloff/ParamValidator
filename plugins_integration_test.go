// plugins_integration_test.go
package paramvalidator

import (
	"fmt"
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
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

func TestPluginPriority(t *testing.T) {
	// Важно: порядок регистрации плагинов определяет приоритет
	parser := NewRuleParser(
		plugins.NewComparisonPlugin(), // Более специфичный - первый
		NewCatchAllPlugin(),           // Общий - последний
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
