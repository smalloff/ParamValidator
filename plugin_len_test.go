package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func TestLengthPlugin(t *testing.T) {
	plugin := plugins.NewLengthPlugin()

	tests := []struct {
		name       string
		constraint string
		value      string
		expected   bool
	}{
		{
			name:       "len greater than valid",
			constraint: "len:>5",
			value:      "hello!",
			expected:   true,
		},
		{
			name:       "len greater than invalid",
			constraint: "len:>5",
			value:      "hello",
			expected:   false,
		},
		{
			name:       "len greater than or equal valid",
			constraint: "len:>=5",
			value:      "hello",
			expected:   true,
		},
		{
			name:       "len greater than or equal invalid",
			constraint: "len:>=5",
			value:      "test",
			expected:   false,
		},
		{
			name:       "len less than valid",
			constraint: "len:<10",
			value:      "short",
			expected:   true,
		},
		{
			name:       "len less than invalid",
			constraint: "len:<10",
			value:      "this is too long",
			expected:   false,
		},
		{
			name:       "len less than or equal valid",
			constraint: "len:<=5",
			value:      "hello",
			expected:   true,
		},
		{
			name:       "len less than or equal invalid",
			constraint: "len:<=5",
			value:      "hello!",
			expected:   false,
		},
		{
			name:       "len equal valid",
			constraint: "len:5",
			value:      "hello",
			expected:   true,
		},
		{
			name:       "len equal invalid",
			constraint: "len:5",
			value:      "hi",
			expected:   false,
		},
		{
			name:       "len not equal valid",
			constraint: "len:!=5",
			value:      "hi",
			expected:   true,
		},
		{
			name:       "len not equal invalid",
			constraint: "len:!=5",
			value:      "hello",
			expected:   false,
		},
		{
			name:       "len range valid",
			constraint: "len:5..10",
			value:      "hello!",
			expected:   true,
		},
		{
			name:       "len range invalid",
			constraint: "len:5..10",
			value:      "hi",
			expected:   false,
		},
		{
			name:       "len range exact min",
			constraint: "len:5..10",
			value:      "hello",
			expected:   true,
		},
		{
			name:       "len range exact max",
			constraint: "len:5..10",
			value:      "hello worl",
			expected:   true,
		},
		{
			name:       "unicode string valid",
			constraint: "len:3",
			value:      "при",
			expected:   true,
		},
		{
			name:       "unicode string invalid",
			constraint: "len:3",
			value:      "привет",
			expected:   false,
		},
		{
			name:       "unicode string range valid",
			constraint: "len:2..4",
			value:      "世界",
			expected:   true,
		},
		{
			name:       "unicode string range invalid",
			constraint: "len:2..4",
			value:      "世界你好",
			expected:   true,
		},
		{
			name:       "empty string with min length",
			constraint: "len:>=1",
			value:      "",
			expected:   false,
		},
		{
			name:       "empty string with zero length",
			constraint: "len:0",
			value:      "",
			expected:   true,
		},
		{
			name:       "empty string with range",
			constraint: "len:0..5",
			value:      "",
			expected:   true,
		},
		{
			name:       "very long string",
			constraint: "len:<1000",
			value:      "a",
			expected:   true,
		},
		{
			name:       "string with spaces",
			constraint: "len:11",
			value:      "hello world",
			expected:   true,
		},
		{
			name:       "string with special characters",
			constraint: "len:5",
			value:      "a+b=c",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := plugin.Parse("test_param", tt.constraint)
			if err != nil {
				t.Errorf("Parse(%q) failed: %v", tt.constraint, err)
				return
			}

			if validator != nil {
				result := validator(tt.value)
				if result != tt.expected {
					t.Errorf("Validator(%q) for constraint %q = %v, expected %v",
						tt.value, tt.constraint, result, tt.expected)
				}
			} else {
				t.Errorf("Parse(%q) returned nil validator", tt.constraint)
			}
		})
	}
}

func TestLengthPluginIntegration(t *testing.T) {
	lengthPlugin := plugins.NewLengthPlugin()
	parser := NewRuleParser(lengthPlugin)

	tests := []struct {
		name     string
		rule     string
		value    string
		expected bool
	}{
		{
			name:     "len greater than in rule",
			rule:     "username=[len:>5]",
			value:    "john_doe",
			expected: true,
		},
		{
			name:     "len greater than in rule too short",
			rule:     "username=[len:>5]",
			value:    "john",
			expected: false,
		},
		{
			name:     "len less than in rule",
			rule:     "password=[len:<20]",
			value:    "shortpass",
			expected: true,
		},
		{
			name:     "len less than in rule too long",
			rule:     "password=[len:<20]",
			value:    "this_is_a_very_long_password",
			expected: false,
		},
		{
			name:     "len range in rule",
			rule:     "code=[len:5..10]",
			value:    "123456",
			expected: true,
		},
		{
			name:     "len range in rule too short",
			rule:     "code=[len:5..10]",
			value:    "123",
			expected: false,
		},
		{
			name:     "len range in rule too long",
			rule:     "code=[len:5..10]",
			value:    "12345678901",
			expected: false,
		},
		{
			name:     "exact length in URL rule",
			rule:     "/api?token=[len:32]",
			value:    "abc123def456ghi789jkl012mno345pq",
			expected: true,
		},
		{
			name:     "exact length in URL rule wrong length",
			rule:     "/api?token=[len:32]",
			value:    "short",
			expected: false,
		},
		{
			name:     "not equal length in rule",
			rule:     "id=[len:!=0]",
			value:    "123",
			expected: true,
		},
		{
			name:     "not equal length in rule empty",
			rule:     "id=[len:!=0]",
			value:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalParams, urlRules, err := parser.parseRulesUnsafe(tt.rule)
			if err != nil {
				t.Fatalf("Failed to parse rule %q: %v", tt.rule, err)
			}

			var paramRule *ParamRule
			for _, rule := range globalParams {
				if rule != nil {
					paramRule = rule
					break
				}
			}

			if paramRule == nil {
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

func TestLengthEdgeCases(t *testing.T) {
	plugin := plugins.NewLengthPlugin()

	tests := []struct {
		name       string
		constraint string
		shouldFail bool
	}{
		{
			name:       "invalid format no number",
			constraint: "len:>",
			shouldFail: true,
		},
		{
			name:       "invalid format text",
			constraint: "len:>abc",
			shouldFail: true,
		},
		{
			name:       "invalid range format",
			constraint: "len:5..",
			shouldFail: true,
		},
		{
			name:       "invalid range min greater than max",
			constraint: "len:10..5",
			shouldFail: true,
		},
		{
			name:       "negative length",
			constraint: "len:>-5",
			shouldFail: true,
		},
		{
			name:       "empty constraint",
			constraint: "",
			shouldFail: true,
		},
		{
			name:       "unsupported prefix",
			constraint: "width>5",
			shouldFail: true,
		},
		{
			name:       "simple operator without len",
			constraint: ">5",
			shouldFail: true,
		},
		{
			name:       "range without len",
			constraint: "5..10",
			shouldFail: true,
		},
		{
			name:       "alternative prefix",
			constraint: "len:gth>5",
			shouldFail: true,
		},
		{
			name:       "double operator",
			constraint: "len:>>5",
			shouldFail: true,
		},
		{
			name:       "very large number",
			constraint: "len:>9999999999",
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := plugin.Parse("test", tt.constraint)

			if tt.shouldFail {
				if err == nil {
					t.Errorf("Parse(%q) should have failed but succeeded", tt.constraint)
				}
			} else {
				if err != nil {
					t.Errorf("Parse(%q) failed: %v", tt.constraint, err)
				}
			}
		})
	}
}

func BenchmarkLengthPlugin(b *testing.B) {
	plugin := plugins.NewLengthPlugin()
	validator, err := plugin.Parse("test", "len:>5")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator("hello!")
	}
}

func BenchmarkLengthPluginParse(b *testing.B) {
	plugin := plugins.NewLengthPlugin()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.Parse("test", "len:>5")
	}
}

func BenchmarkLengthPluginNormalization(b *testing.B) {
	lengthPlugin := plugins.NewLengthPlugin()
	pv, err := NewParamValidator("", WithPlugins(lengthPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?username=[len:>5]&code=[len:5..10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterURL("/api?username=john_doe&code=123456&invalid=value")
	}
}

func BenchmarkLengthPluginFilterQuery(b *testing.B) {
	lengthPlugin := plugins.NewLengthPlugin()
	pv, err := NewParamValidator("", WithPlugins(lengthPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?username=[len:>5]&code=[len:5..10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterQuery("/api", "username=john_doe&code=123456&invalid=value")
	}
}

func BenchmarkLengthPluginValidateQuery(b *testing.B) {
	lengthPlugin := plugins.NewLengthPlugin()
	pv, err := NewParamValidator("", WithPlugins(lengthPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?username=[len:>5]&code=[len:5..10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateQuery("/api", "username=john_doe&code=123456&invalid=value")
	}
}
