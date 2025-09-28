package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func TestLengthPlugin(t *testing.T) {
	plugin := plugins.NewLengthPlugin()

	tests := []struct {
		name        string
		constraint  string
		value       string
		shouldParse bool
		expected    bool
		shouldError bool
	}{
		// Операторы с префиксом len
		{
			name:        "len greater than valid",
			constraint:  "len>5",
			value:       "hello!",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "len greater than invalid",
			constraint:  "len>5",
			value:       "hello",
			shouldParse: true,
			expected:    false,
		},
		{
			name:        "len greater than or equal valid",
			constraint:  "len>=5",
			value:       "hello",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "len greater than or equal invalid",
			constraint:  "len>=5",
			value:       "test",
			shouldParse: true,
			expected:    false,
		},
		{
			name:        "len less than valid",
			constraint:  "len<10",
			value:       "short",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "len less than invalid",
			constraint:  "len<10",
			value:       "this is too long",
			shouldParse: true,
			expected:    false,
		},
		{
			name:        "len less than or equal valid",
			constraint:  "len<=5",
			value:       "hello",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "len less than or equal invalid",
			constraint:  "len<=5",
			value:       "hello!",
			shouldParse: true,
			expected:    false,
		},
		{
			name:        "len equal valid",
			constraint:  "len=5",
			value:       "hello",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "len equal invalid",
			constraint:  "len=5",
			value:       "hi",
			shouldParse: true,
			expected:    false,
		},
		{
			name:        "len not equal valid",
			constraint:  "len!=5",
			value:       "hi",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "len not equal invalid",
			constraint:  "len!=5",
			value:       "hello",
			shouldParse: true,
			expected:    false,
		},

		// Диапазоны
		{
			name:        "len range valid",
			constraint:  "len5..10",
			value:       "hello!",
			shouldParse: true,
			expected:    true,
		},
		{
			name:        "len range invalid",
			constraint:  "len5..10",
			value:       "hi",
			shouldParse: true,
			expected:    false,
		},

		// Unicode строки
		{
			name:        "unicode string valid",
			constraint:  "len=3",
			value:       "привет", // 6 байт, но 6 символов
			shouldParse: true,
			expected:    false, // 6 символов ≠ 3
		},
		{
			name:        "unicode string range valid",
			constraint:  "len2..4",
			value:       "世界", // 2 символа
			shouldParse: true,
			expected:    true,
		},

		// Ошибочные форматы
		{
			name:        "invalid format no number",
			constraint:  "len>",
			shouldParse: true,
			shouldError: true,
		},
		{
			name:        "invalid format text",
			constraint:  "len>abc",
			shouldParse: true,
			shouldError: true,
		},
		{
			name:        "invalid range format",
			constraint:  "len5..",
			shouldParse: true,
			shouldError: true,
		},
		{
			name:        "invalid range min greater than max",
			constraint:  "len10..5",
			shouldParse: true,
			shouldError: true,
		},
		{
			name:        "negative length",
			constraint:  "len>-5",
			shouldParse: true,
			shouldError: true,
		},
		{
			name:        "empty constraint",
			constraint:  "",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "unsupported prefix",
			constraint:  "width>5",
			shouldParse: false,
			shouldError: true,
		},
		// НЕ поддерживаемые форматы (без len)
		{
			name:        "simple operator without len",
			constraint:  ">5",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "range without len",
			constraint:  "5..10",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "alternative prefix",
			constraint:  "length>5",
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

func TestLengthPluginIntegration(t *testing.T) {
	// Create parser with length plugin
	parser := NewRuleParser(plugins.NewLengthPlugin())

	tests := []struct {
		name     string
		rule     string
		value    string
		expected bool
	}{
		{
			name:     "len greater than in rule",
			rule:     "username=[len>5]",
			value:    "john_doe",
			expected: true,
		},
		{
			name:     "len less than in rule",
			rule:     "password=[len<20]",
			value:    "shortpass",
			expected: true,
		},
		{
			name:     "len range in rule",
			rule:     "code=[len5..10]",
			value:    "123456",
			expected: true,
		},
		{
			name:     "exact length in URL rule",
			rule:     "/api?token=[len=32]",
			value:    "abc123def456ghi789jkl012mno345pq",
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

func TestLengthPluginEdgeCases(t *testing.T) {
	plugin := plugins.NewLengthPlugin()

	edgeCases := []struct {
		name       string
		constraint string
		value      string
		expected   bool
	}{
		{
			name:       "empty string with min length",
			constraint: "len>=1",
			value:      "",
			expected:   false,
		},
		{
			name:       "empty string with zero length",
			constraint: "len=0",
			value:      "",
			expected:   true,
		},
		{
			name:       "very long string",
			constraint: "len<1000",
			value:      "a",
			expected:   true,
		},
		{
			name:       "string with spaces",
			constraint: "len=11",
			value:      "hello world",
			expected:   true,
		},
		{
			name:       "string with special characters",
			constraint: "len=5",
			value:      "a+b=c",
			expected:   true,
		},
	}

	for _, tt := range edgeCases {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := plugin.Parse("test_param", tt.constraint)
			if err != nil {
				t.Fatalf("Parse(%q) failed: %v", tt.constraint, err)
			}

			result := validator(tt.value)
			if result != tt.expected {
				t.Errorf("Validator(%q) for constraint %q = %v, expected %v",
					tt.value, tt.constraint, result, tt.expected)
			}
		})
	}
}
