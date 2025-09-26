// comparison_plugin_test.go
package paramvalidator

import (
	"testing"
)

func TestComparisonPlugin(t *testing.T) {
	plugin := NewComparisonPlugin()

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

		// Invalid formats
		{
			name:        "invalid format multiple operators",
			constraint:  ">>5",
			shouldParse: true, // CanParse returns true (starts with >)
			shouldError: true, // But Parse should fail
		},
		{
			name:        "invalid format no number",
			constraint:  ">",
			shouldParse: true, // CanParse returns true (starts with >)
			shouldError: true, // But Parse should fail
		},
		{
			name:        "invalid format text",
			constraint:  ">abc",
			shouldParse: true, // CanParse returns true (starts with >)
			shouldError: true, // But Parse should fail
		},
		{
			name:        "empty constraint",
			constraint:  "",
			shouldParse: false, // Empty string - CanParse returns false
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
				return // Skip validation tests for constraints that shouldn't parse
			}

			// Test Parse
			validator, err := plugin.Parse("test_param", tt.constraint)

			if tt.shouldError {
				// We expect an error
				if err == nil {
					t.Errorf("Parse(%q) should have failed but succeeded", tt.constraint)
				} else {
					t.Logf("Correctly got error for %q: %v", tt.constraint, err)
				}
				return
			} else {
				// We don't expect an error
				if err != nil {
					t.Errorf("Parse(%q) failed: %v", tt.constraint, err)
					return
				}
			}

			// Test validation (only for successful parses)
			result := validator(tt.value)
			if result != tt.expected {
				t.Errorf("Validator(%q) for constraint %q = %v, expected %v",
					tt.value, tt.constraint, result, tt.expected)
			}
		})
	}
}

func TestComparisonPluginIntegration(t *testing.T) {
	// Create parser with comparison plugin
	parser := NewRuleParser(NewComparisonPlugin())

	tests := []struct {
		name     string
		rule     string
		value    string
		expected bool
	}{
		{
			name:     "greater than in rule",
			rule:     "age=[>18]",
			value:    "20",
			expected: true,
		},
		{
			name:     "less than in rule",
			rule:     "price=[<1000]",
			value:    "500",
			expected: true,
		},
		{
			name:     "greater than or equal in URL rule",
			rule:     "/users?min_age=[>=21]",
			value:    "21",
			expected: true,
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
