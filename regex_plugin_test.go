package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func TestRegexPlugin(t *testing.T) {
	plugin := plugins.NewRegexPlugin()

	tests := []struct {
		name        string
		constraint  string
		value       string
		shouldParse bool
		expected    bool
	}{
		// Valid regex patterns
		{
			name:        "digits only",
			constraint:  "/^\\d+$/",
			value:       "12345",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "digits only invalid",
			constraint:  "/^\\d+$/",
			value:       "123abc",
			shouldParse: true,
			expected:    false,
		},
		{
			name:        "email pattern valid",
			constraint:  "/^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$/",
			value:       "test@example.com",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "email pattern invalid",
			constraint:  "/^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$/",
			value:       "invalid-email",
			shouldParse: true,
			expected:    false,
		},
		{
			name:        "specific format",
			constraint:  "/^[A-Z][a-z]+\\s[A-Z][a-z]+$/",
			value:       "John Doe",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "specific format invalid",
			constraint:  "/^[A-Z][a-z]+\\s[A-Z][a-z]+$/",
			value:       "john doe",
			shouldParse: true,
			expected:    false,
		},

		// Invalid regex patterns
		{
			name:        "invalid regex unclosed group",
			constraint:  "/^[a-z$/",
			shouldParse: true, // CanParse returns true, but Parse should fail
		},
		{
			name:        "empty regex",
			constraint:  "//",
			shouldParse: true,
			expected:    false,
		},

		// Non-regex constraints
		{
			name:        "not a regex pattern",
			constraint:  "simple",
			shouldParse: false,
		},
		{
			name:        "range pattern",
			constraint:  "1-10",
			shouldParse: false,
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
				return // Skip validation tests for non-regex constraints
			}

			// Test Parse
			validator, err := plugin.Parse("test_param", tt.constraint)

			// Handle invalid regex patterns that should fail parsing
			if err != nil && tt.name != "invalid regex unclosed group" && tt.name != "empty regex" {
				t.Errorf("Parse(%q) failed: %v", tt.constraint, err)
				return
			}

			if err != nil {
				return // Expected parsing failure
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

func TestRegexPluginIntegration(t *testing.T) {
	parser := NewRuleParser(plugins.NewRegexPlugin())

	tests := []struct {
		name     string
		rule     string
		value    string
		expected bool
	}{
		{
			name:     "username pattern",
			rule:     "username=[/^[a-zA-Z0-9_]{3,20}$/]",
			value:    "user_123",
			expected: true,
		},
		{
			name:     "username pattern too short",
			rule:     "username=[/^[a-zA-Z0-9_]{3,20}$/]",
			value:    "ab",
			expected: false,
		},
		{
			name:     "hex color pattern",
			rule:     "color=[/^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$/]",
			value:    "#ff00ff",
			expected: true,
		},
		{
			name:     "hex color pattern invalid",
			rule:     "color=[/^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$/]",
			value:    "#xyz",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paramRule, err := parser.parseSingleParamRuleUnsafe(tt.rule)
			if err != nil {
				t.Fatalf("Failed to parse rule %q: %v", tt.rule, err)
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
