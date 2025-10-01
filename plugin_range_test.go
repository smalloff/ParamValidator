package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func TestRangePlugin(t *testing.T) {
	plugin := plugins.NewRangePlugin()

	tests := []struct {
		name       string
		constraint string
		value      string
		expectErr  bool
		expected   bool
	}{
		{
			name:       "basic range valid",
			constraint: "range:1-10",
			value:      "5",
			expected:   true,
		},
		{
			name:       "basic range lower bound",
			constraint: "range:1-10",
			value:      "1",
			expected:   true,
		},
		{
			name:       "basic range upper bound",
			constraint: "range:1-10",
			value:      "10",
			expected:   true,
		},
		{
			name:       "basic range below min",
			constraint: "range:1-10",
			value:      "0",
			expected:   false,
		},
		{
			name:       "basic range above max",
			constraint: "range:1-10",
			value:      "11",
			expected:   false,
		},
		{
			name:       "dots range valid",
			constraint: "range:1..10",
			value:      "5",
			expected:   true,
		},
		{
			name:       "dots range lower bound",
			constraint: "range:1..10",
			value:      "1",
			expected:   true,
		},
		{
			name:       "dots range upper bound",
			constraint: "range:1..10",
			value:      "10",
			expected:   true,
		},
		{
			name:       "dots range invalid",
			constraint: "range:1..10",
			value:      "15",
			expected:   false,
		},
		{
			name:       "negative range valid",
			constraint: "range:-10..10",
			value:      "-5",
			expected:   true,
		},
		{
			name:       "negative range lower bound",
			constraint: "range:-10..10",
			value:      "-10",
			expected:   true,
		},
		{
			name:       "negative range upper bound",
			constraint: "range:-10..10",
			value:      "10",
			expected:   true,
		},
		{
			name:       "negative range invalid",
			constraint: "range:-10..10",
			value:      "-15",
			expected:   false,
		},
		{
			name:       "all negative range",
			constraint: "range:-50..-10",
			value:      "-25",
			expected:   true,
		},
		{
			name:       "all negative range lower bound",
			constraint: "range:-50..-10",
			value:      "-50",
			expected:   true,
		},
		{
			name:       "all negative range upper bound",
			constraint: "range:-50..-10",
			value:      "-10",
			expected:   true,
		},
		{
			name:       "all negative range invalid",
			constraint: "range:-50..-10",
			value:      "-5",
			expected:   false,
		},
		{
			name:       "large range valid",
			constraint: "range:1000-9999",
			value:      "5000",
			expected:   true,
		},
		{
			name:       "large range lower bound",
			constraint: "range:1000-9999",
			value:      "1000",
			expected:   true,
		},
		{
			name:       "large range upper bound",
			constraint: "range:1000-9999",
			value:      "9999",
			expected:   true,
		},
		{
			name:       "large range invalid",
			constraint: "range:1000-9999",
			value:      "999",
			expected:   false,
		},
		{
			name:       "single value range valid",
			constraint: "range:42..42",
			value:      "42",
			expected:   true,
		},
		{
			name:       "single value range invalid",
			constraint: "range:42..42",
			value:      "41",
			expected:   false,
		},
		{
			name:       "zero range valid",
			constraint: "range:0..0",
			value:      "0",
			expected:   true,
		},
		{
			name:       "zero to positive range",
			constraint: "range:0..100",
			value:      "50",
			expected:   true,
		},
		{
			name:       "empty constraint",
			constraint: "",
			expectErr:  true,
		},
		{
			name:       "range without numbers",
			constraint: "range:",
			expectErr:  true,
		},
		{
			name:       "single number",
			constraint: "range:5",
			expectErr:  true,
		},
		{
			name:       "triple numbers",
			constraint: "range:1-10-100",
			expectErr:  true,
		},
		{
			name:       "text instead of numbers",
			constraint: "range:a-z",
			expectErr:  true,
		},
		{
			name:       "mixed text numbers",
			constraint: "range:1-abc",
			expectErr:  true,
		},
		{
			name:       "min greater than max",
			constraint: "range:10-1",
			expectErr:  true,
		},
		{
			name:       "empty min value",
			constraint: "range:-10",
			expectErr:  true,
		},
		{
			name:       "empty max value",
			constraint: "range:10-",
			expectErr:  true,
		},
		{
			name:       "dots with empty min",
			constraint: "range:..10",
			expectErr:  true,
		},
		{
			name:       "dots with empty max",
			constraint: "range:10..",
			expectErr:  true,
		},
		{
			name:       "enum format should not parse",
			constraint: "a,b,c",
			expectErr:  true,
		},
		{
			name:       "very large numbers should fail",
			constraint: "range:1..9999999999",
			expectErr:  true,
		},
		{
			name:       "negative with hyphen separator",
			constraint: "range:-10--1",
			value:      "-5",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := plugin.Parse("test_param", tt.constraint)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Parse(%q) should have failed but succeeded", tt.constraint)
				}
				return
			}

			if err != nil {
				t.Errorf("Parse(%q) failed: %v", tt.constraint, err)
				return
			}

			result := validator(tt.value)
			if result != tt.expected {
				t.Errorf("Validator(%q) for constraint %q = %v, expected %v",
					tt.value, tt.constraint, result, tt.expected)
			}
		})
	}
}

func TestRangePluginIntegration(t *testing.T) {
	rangePlugin := plugins.NewRangePlugin()
	parser := NewRuleParser(rangePlugin)

	tests := []struct {
		name     string
		rule     string
		params   map[string]string
		expected bool
	}{
		{
			name:     "range in param rule",
			rule:     "age=[range:18-65]",
			params:   map[string]string{"age": "25"},
			expected: true,
		},
		{
			name:     "range in param rule too young",
			rule:     "age=[range:18-65]",
			params:   map[string]string{"age": "16"},
			expected: false,
		},
		{
			name:     "range in param rule too old",
			rule:     "age=[range:18-65]",
			params:   map[string]string{"age": "70"},
			expected: false,
		},
		{
			name:     "range in param rule lower bound",
			rule:     "age=[range:18-65]",
			params:   map[string]string{"age": "18"},
			expected: true,
		},
		{
			name:     "range in param rule upper bound",
			rule:     "age=[range:18-65]",
			params:   map[string]string{"age": "65"},
			expected: true,
		},
		{
			name:     "dots range in param rule",
			rule:     "price=[range:100..1000]",
			params:   map[string]string{"price": "500"},
			expected: true,
		},
		{
			name:     "dots range in param rule too low",
			rule:     "price=[range:100..1000]",
			params:   map[string]string{"price": "50"},
			expected: false,
		},
		{
			name:     "dots range in param rule too high",
			rule:     "price=[range:100..1000]",
			params:   map[string]string{"price": "1500"},
			expected: false,
		},
		{
			name:     "negative range in param rule",
			rule:     "temperature=[range:-20..40]",
			params:   map[string]string{"temperature": "25"},
			expected: true,
		},
		{
			name:     "negative range in param rule negative value",
			rule:     "temperature=[range:-20..40]",
			params:   map[string]string{"temperature": "-10"},
			expected: true,
		},
		{
			name:     "negative range in param rule too low",
			rule:     "temperature=[range:-20..40]",
			params:   map[string]string{"temperature": "-25"},
			expected: false,
		},
		{
			name:     "all negative range in param rule",
			rule:     "score=[range:-100..-50]",
			params:   map[string]string{"score": "-75"},
			expected: true,
		},
		{
			name:     "single value range in param rule",
			rule:     "version=[range:5..5]",
			params:   map[string]string{"version": "5"},
			expected: true,
		},
		{
			name:     "single value range in param rule invalid",
			rule:     "version=[range:5..5]",
			params:   map[string]string{"version": "4"},
			expected: false,
		},
		{
			name:     "mixed range types in same rule - age valid",
			rule:     "age=[range:18-65]&price=[range:100..1000]",
			params:   map[string]string{"age": "25", "price": "500"},
			expected: true,
		},
		{
			name:     "mixed range types in same rule - age invalid",
			rule:     "age=[range:18-65]&price=[range:100..1000]",
			params:   map[string]string{"age": "16", "price": "500"},
			expected: false,
		},
		{
			name:     "mixed range types in same rule - price invalid",
			rule:     "age=[range:18-65]&price=[range:100..1000]",
			params:   map[string]string{"age": "25", "price": "50"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalParams, urlRules, err := parser.parseRulesUnsafe(tt.rule)
			if err != nil {
				t.Fatalf("Failed to parse rule %q: %v", tt.rule, err)
			}

			validators := make(map[string]func(string) bool)

			for _, rule := range globalParams {
				if rule != nil && rule.CustomValidator != nil {
					validators[rule.Name] = rule.CustomValidator
				}
			}

			for _, urlRule := range urlRules {
				for _, rule := range urlRule.Params {
					if rule != nil && rule.CustomValidator != nil {
						validators[rule.Name] = rule.CustomValidator
					}
				}
			}

			if len(validators) == 0 {
				t.Fatalf("No validators found for rule %q", tt.rule)
			}

			allValid := true
			for paramName, value := range tt.params {
				validator, exists := validators[paramName]
				if !exists {
					t.Errorf("No validator found for parameter %q", paramName)
					allValid = false
					continue
				}

				result := validator(value)
				if !result {
					allValid = false
				}
			}

			if allValid != tt.expected {
				t.Errorf("Overall validation result = %v, expected %v for params %v",
					allValid, tt.expected, tt.params)
			}
		})
	}
}

func TestRangeEdgeCases(t *testing.T) {
	plugin := plugins.NewRangePlugin()

	tests := []struct {
		name       string
		constraint string
		expectErr  bool
	}{
		{
			name:       "valid range with hyphen",
			constraint: "range:1-10",
		},
		{
			name:       "valid range with dots",
			constraint: "range:1..10",
		},
		{
			name:       "negative range valid",
			constraint: "range:-10..10",
		},
		{
			name:       "all negative range valid",
			constraint: "range:-50..-10",
		},
		{
			name:       "min greater than max should fail",
			constraint: "range:10..1",
			expectErr:  true,
		},
		{
			name:       "empty min value should fail",
			constraint: "range:..10",
			expectErr:  true,
		},
		{
			name:       "empty max value should fail",
			constraint: "range:10..",
			expectErr:  true,
		},
		{
			name:       "text instead of numbers should fail",
			constraint: "range:a..z",
			expectErr:  true,
		},
		{
			name:       "very large numbers should fail",
			constraint: "range:1..9999999999",
			expectErr:  true,
		},
		{
			name:       "triple numbers should fail",
			constraint: "range:1-10-100",
			expectErr:  true,
		},
		{
			name:       "range prefix only",
			constraint: "range:",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := plugin.Parse("test", tt.constraint)

			if tt.expectErr {
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

func TestRangePluginBoundaryValues(t *testing.T) {
	plugin := plugins.NewRangePlugin()

	tests := []struct {
		constraint string
		value      string
		expected   bool
	}{
		{"range:0-100", "0", true},
		{"range:0-100", "100", true},
		{"range:0-100", "-1", false},
		{"range:0-100", "101", false},
		{"range:-100-0", "-100", true},
		{"range:-100-0", "0", true},
		{"range:-100-0", "-101", false},
		{"range:-100-0", "1", false},
		{"range:-100--50", "-100", true},
		{"range:-100--50", "-50", true},
		{"range:-100--50", "-75", true},
		{"range:-100--50", "-49", false},
		{"range:-100--50", "-101", false},
		{"range:999990-1000000", "999990", true},
		{"range:999990-1000000", "1000000", true},
		{"range:999990-1000000", "999989", false},
		{"range:999990-1000000", "1000001", false},
	}

	for _, tt := range tests {
		t.Run(tt.constraint+"_"+tt.value, func(t *testing.T) {
			validator, err := plugin.Parse("test", tt.constraint)
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

func BenchmarkRangePlugin(b *testing.B) {
	plugin := plugins.NewRangePlugin()
	validator, err := plugin.Parse("test", "range:1-100")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator("50")
	}
}

func BenchmarkRangePluginParse(b *testing.B) {
	plugin := plugins.NewRangePlugin()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.Parse("test", "range:1-100")
	}
}

func BenchmarkRangePluginNormalization(b *testing.B) {
	rangePlugin := plugins.NewRangePlugin()
	pv, err := NewParamValidator("", WithPlugins(rangePlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	pv.initialized.Store(true)
	err = pv.ParseRules("/api?age=[range:18-65]&price=[range:100..1000]")
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterURL("/api?age=25&price=500&invalid=value")
	}
}

func BenchmarkRangePluginFilterQuery(b *testing.B) {
	rangePlugin := plugins.NewRangePlugin()
	pv, err := NewParamValidator("", WithPlugins(rangePlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?age=[range:18-65]&price=[range:100..1000]")
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterQuery("/api", "age=25&price=500&invalid=value")
	}
}

func BenchmarkRangePluginValidateQuery(b *testing.B) {
	rangePlugin := plugins.NewRangePlugin()
	pv, err := NewParamValidator("", WithPlugins(rangePlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?age=[range:18-65]&price=[range:100..1000]")
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateQuery("/api", "age=25&price=500&invalid=value")
	}
}

func BenchmarkRangePluginMultipleValidators(b *testing.B) {
	plugin := plugins.NewRangePlugin()

	validators := []func(string) bool{}
	constraints := []string{
		"range:1-10",
		"range:100-1000",
		"range:-50-50",
		"range:0-1000",
		"range:999-9999",
	}

	for _, constraint := range constraints {
		validator, err := plugin.Parse("test", constraint)
		if err != nil {
			b.Fatalf("Failed to create validator for %s: %v", constraint, err)
		}
		validators = append(validators, validator)
	}

	values := []string{"5", "500", "0", "750", "5000"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j, validator := range validators {
			validator(values[j])
		}
	}
}
