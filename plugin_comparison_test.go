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
		shouldError bool
	}{
		{
			name:        "valid greater than",
			constraint:  "cmp:>100",
			shouldError: false,
		},
		{
			name:        "valid greater than or equal",
			constraint:  "cmp:>=100",
			shouldError: false,
		},
		{
			name:        "valid less than",
			constraint:  "cmp:<100",
			shouldError: false,
		},
		{
			name:        "valid less than or equal",
			constraint:  "cmp:<=100",
			shouldError: false,
		},
		{
			name:        "double greater than should fail",
			constraint:  "cmp:>>100",
			shouldError: true,
		},
		{
			name:        "double less than should fail",
			constraint:  "cmp:<<100",
			shouldError: true,
		},
		{
			name:        "mixed operators should fail",
			constraint:  "cmp:><100",
			shouldError: true,
		},
		{
			name:        "operator with text should fail",
			constraint:  "cmp:>abc",
			shouldError: true,
		},
		{
			name:        "empty after operator should fail",
			constraint:  "cmp:>",
			shouldError: true,
		},
		{
			name:        "operator with equals only should fail",
			constraint:  "cmp:>=",
			shouldError: true,
		},
		{
			name:        "negative number valid",
			constraint:  "cmp:>-100",
			shouldError: false,
		},
		{
			name:        "not comparison format",
			constraint:  "cmp:abc123",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := plugin.Parse("test", tt.constraint)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Parse(%q) should fail but succeeded", tt.constraint)
				}
			} else {
				if err != nil {
					t.Errorf("Parse(%q) failed but should succeed: %v", tt.constraint, err)
				} else if validator == nil {
					t.Errorf("Parse(%q) returned nil validator", tt.constraint)
				}
			}
		})
	}
}

func TestComparisonPlugin(t *testing.T) {
	plugin := plugins.NewComparisonPlugin()

	tests := []struct {
		name       string
		constraint string
		value      string
		expected   bool
	}{
		{
			name:       "greater than valid",
			constraint: "cmp:>5",
			value:      "6",
			expected:   true,
		},
		{
			name:       "greater than invalid",
			constraint: "cmp:>5",
			value:      "5",
			expected:   false,
		},
		{
			name:       "greater than equal invalid",
			constraint: "cmp:>5",
			value:      "4",
			expected:   false,
		},
		{
			name:       "greater than or equal valid",
			constraint: "cmp:>=5",
			value:      "5",
			expected:   true,
		},
		{
			name:       "greater than or equal valid higher",
			constraint: "cmp:>=5",
			value:      "6",
			expected:   true,
		},
		{
			name:       "greater than or equal invalid",
			constraint: "cmp:>=5",
			value:      "4",
			expected:   false,
		},
		{
			name:       "less than valid",
			constraint: "cmp:<10",
			value:      "9",
			expected:   true,
		},
		{
			name:       "less than invalid",
			constraint: "cmp:<10",
			value:      "10",
			expected:   false,
		},
		{
			name:       "less than equal invalid",
			constraint: "cmp:<10",
			value:      "11",
			expected:   false,
		},
		{
			name:       "less than or equal valid",
			constraint: "cmp:<=10",
			value:      "10",
			expected:   true,
		},
		{
			name:       "less than or equal valid lower",
			constraint: "cmp:<=10",
			value:      "9",
			expected:   true,
		},
		{
			name:       "less than or equal invalid",
			constraint: "cmp:<=10",
			value:      "11",
			expected:   false,
		},
		{
			name:       "negative numbers valid",
			constraint: "cmp:>-5",
			value:      "-4",
			expected:   true,
		},
		{
			name:       "negative numbers invalid",
			constraint: "cmp:>-5",
			value:      "-6",
			expected:   false,
		},
		{
			name:       "negative range valid",
			constraint: "cmp:>=-100",
			value:      "-50",
			expected:   true,
		},
		{
			name:       "large numbers valid",
			constraint: "cmp:>100",
			value:      "150",
			expected:   true,
		},
		{
			name:       "large numbers invalid",
			constraint: "cmp:<1000000",
			value:      "1000001",
			expected:   false,
		},
		{
			name:       "equal boundary valid",
			constraint: "cmp:>=0",
			value:      "0",
			expected:   true,
		},
		{
			name:       "equal boundary invalid",
			constraint: "cmp:>=0",
			value:      "-1",
			expected:   false,
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

func TestComparisonPluginIntegration(t *testing.T) {
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
			rule:     "age=[cmp:>18]",
			value:    "25",
			expected: true,
		},
		{
			name:     "greater than in param rule too young",
			rule:     "age=[cmp:>18]",
			value:    "16",
			expected: false,
		},
		{
			name:     "less than in param rule",
			rule:     "price=[cmp:<1000]",
			value:    "500",
			expected: true,
		},
		{
			name:     "less than in param rule too expensive",
			rule:     "price=[cmp:<1000]",
			value:    "1500",
			expected: false,
		},
		{
			name:     "greater or equal in param rule",
			rule:     "score=[cmp:>=50]",
			value:    "50",
			expected: true,
		},
		{
			name:     "greater or equal in param rule below",
			rule:     "score=[cmp:>=50]",
			value:    "49",
			expected: false,
		},
		{
			name:     "less or equal in param rule",
			rule:     "quantity=[cmp:<=10]",
			value:    "10",
			expected: true,
		},
		{
			name:     "less or equal in param rule above",
			rule:     "quantity=[cmp:<=10]",
			value:    "11",
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

func TestComparisonPluginWithValidateURL(t *testing.T) {
	comparisonPlugin := plugins.NewComparisonPlugin()

	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "validate URL with greater than",
			rules:    "/api?age=[cmp:>18]",
			url:      "/api?age=25",
			expected: true,
		},
		{
			name:     "validate URL with greater than too low",
			rules:    "/api?age=[cmp:>18]",
			url:      "/api?age=16",
			expected: false,
		},
		{
			name:     "validate URL with less than",
			rules:    "/api?price=[cmp:<1000]",
			url:      "/api?price=500",
			expected: true,
		},
		{
			name:     "validate URL with less than too high",
			rules:    "/api?price=[cmp:<1000]",
			url:      "/api?price=1500",
			expected: false,
		},
		{
			name:     "validate URL with greater or equal",
			rules:    "/api?score=[cmp:>=50]",
			url:      "/api?score=50",
			expected: true,
		},
		{
			name:     "validate URL with greater or equal below",
			rules:    "/api?score=[cmp:>=50]",
			url:      "/api?score=49",
			expected: false,
		},
		{
			name:     "validate URL with less or equal",
			rules:    "/api?quantity=[cmp:<=10]",
			url:      "/api?quantity=10",
			expected: true,
		},
		{
			name:     "validate URL with less or equal above",
			rules:    "/api?quantity=[cmp:<=10]",
			url:      "/api?quantity=11",
			expected: false,
		},
		{
			name:     "validate URL with multiple comparison constraints",
			rules:    "/api?age=[cmp:>18]&price=[cmp:<1000]",
			url:      "/api?age=25&price=500",
			expected: true,
		},
		{
			name:     "validate URL with one invalid comparison constraint",
			rules:    "/api?age=[cmp:>18]&price=[cmp:<1000]",
			url:      "/api?age=16&price=500",
			expected: false,
		},
		{
			name:     "validate URL with negative numbers",
			rules:    "/api?temp=[cmp:>-10]",
			url:      "/api?temp=-5",
			expected: true,
		},
		{
			name:     "validate URL with negative numbers invalid",
			rules:    "/api?temp=[cmp:>-10]",
			url:      "/api?temp=-15",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rules, WithPlugins(comparisonPlugin))
			if err != nil {
				t.Fatalf("Failed to create validator: %v", err)
			}

			result := pv.ValidateURL(tt.url)
			if result != tt.expected {
				t.Errorf("ValidateURL(%q) with rules %q = %v, expected %v",
					tt.url, tt.rules, result, tt.expected)
			}
		})
	}
}

func TestComparisonPluginWithFilterURL(t *testing.T) {
	comparisonPlugin := plugins.NewComparisonPlugin()

	tests := []struct {
		name     string
		rules    string
		url      string
		expected string
	}{
		{
			name:     "filter URL with greater than",
			rules:    "/api?age=[cmp:>18]",
			url:      "/api?age=25&age=16",
			expected: "/api?age=25",
		},
		{
			name:     "filter URL with less than",
			rules:    "/api?price=[cmp:<1000]",
			url:      "/api?price=500&price=1500",
			expected: "/api?price=500",
		},
		{
			name:     "filter URL with greater or equal",
			rules:    "/api?score=[cmp:>=50]",
			url:      "/api?score=50&score=49",
			expected: "/api?score=50",
		},
		{
			name:     "filter URL with less or equal",
			rules:    "/api?quantity=[cmp:<=10]",
			url:      "/api?quantity=10&quantity=11",
			expected: "/api?quantity=10",
		},
		{
			name:     "filter URL with multiple comparison constraints",
			rules:    "/api?age=[cmp:>18]&price=[cmp:<1000]",
			url:      "/api?age=25&price=500&invalid=value",
			expected: "/api?age=25&price=500",
		},
		{
			name:     "filter URL remove all invalid parameters",
			rules:    "/api?age=[cmp:>18]&price=[cmp:<1000]",
			url:      "/api?age=16&price=1500&invalid=value",
			expected: "/api",
		},
		{
			name:     "filter URL with mixed valid and invalid values",
			rules:    "/api?score=[cmp:>=50]&quantity=[cmp:<=10]",
			url:      "/api?score=75&score=25&quantity=5&quantity=15",
			expected: "/api?score=75&quantity=5",
		},
		{
			name:     "filter URL with negative numbers",
			rules:    "/api?temp=[cmp:>-10]",
			url:      "/api?temp=-5&temp=-15",
			expected: "/api?temp=-5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rules, WithPlugins(comparisonPlugin))
			if err != nil {
				t.Fatalf("Failed to create validator: %v", err)
			}

			result := pv.FilterURL(tt.url)
			if result != tt.expected {
				t.Errorf("FilterURL(%q) with rules %q = %q, expected %q",
					tt.url, tt.rules, result, tt.expected)
			}
		})
	}
}

func BenchmarkComparisonPlugin(b *testing.B) {
	plugin := plugins.NewComparisonPlugin()
	validator, err := plugin.Parse("test", "cmp:>100")
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
		plugin.Parse("test", "cmp:>100")
	}
}

func BenchmarkComparisonPluginNormalization(b *testing.B) {
	comparisonPlugin := plugins.NewComparisonPlugin()
	pv, err := NewParamValidator("", WithPlugins(comparisonPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?score=[cmp:>50]&quantity=[cmp:<=10]")
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
	err = pv.ParseRules("/api?score=[cmp:>50]&quantity=[cmp:<=10]")
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
	err = pv.ParseRules("/api?score=[cmp:>50]&quantity=[cmp:<=10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateQuery("/api", "score=75&quantity=5&invalid=value")
	}
}

func BenchmarkComparisonPluginValidateURL(b *testing.B) {
	comparisonPlugin := plugins.NewComparisonPlugin()
	pv, err := NewParamValidator("/api?age=[cmp:>18]&price=[cmp:<1000]", WithPlugins(comparisonPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateURL("/api?age=25&price=500")
	}
}

func BenchmarkComparisonPluginFilterURL(b *testing.B) {
	comparisonPlugin := plugins.NewComparisonPlugin()
	pv, err := NewParamValidator("/api?age=[cmp:>18]&price=[cmp:<1000]", WithPlugins(comparisonPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterURL("/api?age=25&price=500&invalid=value")
	}
}
