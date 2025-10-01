package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func TestPatternPlugin(t *testing.T) {
	plugin := plugins.NewPatternPlugin()

	tests := []struct {
		name       string
		constraint string
		value      string
		expectErr  bool
		expected   bool
	}{
		{
			name:       "prefix pattern match",
			constraint: "in:start*",
			value:      "start_value",
			expected:   true,
		},
		{
			name:       "prefix pattern exact match",
			constraint: "in:start*",
			value:      "start",
			expected:   true,
		},
		{
			name:       "prefix pattern no match",
			constraint: "in:start*",
			value:      "wrong_start",
			expected:   false,
		},
		{
			name:       "prefix pattern empty value",
			constraint: "in:start*",
			value:      "",
			expected:   false,
		},
		{
			name:       "suffix pattern match",
			constraint: "in:*end",
			value:      "value_end",
			expected:   true,
		},
		{
			name:       "suffix pattern exact match",
			constraint: "in:*end",
			value:      "end",
			expected:   true,
		},
		{
			name:       "suffix pattern no match",
			constraint: "in:*end",
			value:      "end_wrong",
			expected:   false,
		},
		{
			name:       "contains pattern match",
			constraint: "in:*val*",
			value:      "some_val_here",
			expected:   true,
		},
		{
			name:       "contains pattern exact match",
			constraint: "in:*val*",
			value:      "val",
			expected:   true,
		},
		{
			name:       "contains pattern no match",
			constraint: "in:*val*",
			value:      "nothing",
			expected:   false,
		},
		{
			name:       "multiple parts match",
			constraint: "in:*one*two*three*",
			value:      "blablaoneblablatwoblathreeblabla",
			expected:   true,
		},
		{
			name:       "multiple parts exact match",
			constraint: "in:*one*two*three*",
			value:      "onetwothree",
			expected:   true,
		},
		{
			name:       "multiple parts partial match",
			constraint: "in:*one*two*three*",
			value:      "one_two",
			expected:   false,
		},
		{
			name:       "any string match",
			constraint: "in:*",
			value:      "any_value",
			expected:   true,
		},
		{
			name:       "any string empty",
			constraint: "in:*",
			value:      "",
			expected:   true,
		},
		{
			name:       "complex pattern match",
			constraint: "in:pre*mid*post",
			value:      "pre123mid456post",
			expected:   true,
		},
		{
			name:       "complex pattern no match",
			constraint: "in:pre*mid*post",
			value:      "pre123mid456",
			expected:   false,
		},
		{
			name:       "empty constraint",
			constraint: "",
			expectErr:  true,
		},
		{
			name:       "only prefix without pattern",
			constraint: "in:",
			expectErr:  true,
		},
		{
			name:       "only wildcard",
			constraint: "in:*",
			expected:   true,
		},
		{
			name:       "multiple wildcards only",
			constraint: "in:**",
			expected:   true,
		},
		{
			name:       "too long pattern",
			constraint: "in:a*" + string(make([]byte, 1001)),
			expectErr:  true,
		},
		{
			name:       "pattern without wildcard",
			constraint: "in:nowildcard",
			expectErr:  true,
		},
		{
			name:       "wrong prefix",
			constraint: "len:*",
			expectErr:  true,
		},
		{
			name:       "wrong prefix range",
			constraint: "range*",
			expectErr:  true,
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

			if validator == nil {
				t.Errorf("Parse(%q) returned nil validator", tt.constraint)
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

func TestPatternPluginIntegration(t *testing.T) {
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

func TestPatternPluginWithValidateURL(t *testing.T) {
	patternPlugin := plugins.NewPatternPlugin()

	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "validate URL with prefix pattern",
			rules:    "/api?file=[in:img_*]",
			url:      "/api?file=img_photo.jpg",
			expected: true,
		},
		{
			name:     "validate URL with prefix pattern no match",
			rules:    "/api?file=[in:img_*]",
			url:      "/api?file=doc_file.pdf",
			expected: false,
		},
		{
			name:     "validate URL with suffix pattern",
			rules:    "/api?file=[in:*.jpg]",
			url:      "/api?file=photo.jpg",
			expected: true,
		},
		{
			name:     "validate URL with suffix pattern no match",
			rules:    "/api?file=[in:*.jpg]",
			url:      "/api?file=document.pdf",
			expected: false,
		},
		{
			name:     "validate URL with contains pattern",
			rules:    "/api?id=[in:*user*]",
			url:      "/api?id=new_user_123",
			expected: true,
		},
		{
			name:     "validate URL with contains pattern no match",
			rules:    "/api?id=[in:*user*]",
			url:      "/api?id=admin_123",
			expected: false,
		},
		{
			name:     "validate URL with complex pattern",
			rules:    "/api?key=[in:prefix_*_suffix]",
			url:      "/api?key=prefix_value_suffix",
			expected: true,
		},
		{
			name:     "validate URL with complex pattern no match",
			rules:    "/api?key=[in:prefix_*_suffix]",
			url:      "/api?key=prefix_value",
			expected: false,
		},
		{
			name:     "validate URL with multiple pattern constraints",
			rules:    "/api?file=[in:img_*]&id=[in:*user*]",
			url:      "/api?file=img_photo.jpg&id=new_user_123",
			expected: true,
		},
		{
			name:     "validate URL with one invalid pattern constraint",
			rules:    "/api?file=[in:img_*]&id=[in:*user*]",
			url:      "/api?file=img_photo.jpg&id=admin_123",
			expected: false,
		},
		{
			name:     "validate URL with wildcard pattern",
			rules:    "/api?any=[in:*]",
			url:      "/api?any=anything",
			expected: true,
		},
		{
			name:     "validate URL with wildcard pattern empty",
			rules:    "/api?any=[in:*]",
			url:      "/api?any=",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rules, WithPlugins(patternPlugin))
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

func TestPatternPluginWithFilterURL(t *testing.T) {
	patternPlugin := plugins.NewPatternPlugin()

	tests := []struct {
		name     string
		rules    string
		url      string
		expected string
	}{
		{
			name:     "filter URL with prefix pattern",
			rules:    "/api?file=[in:img_*]",
			url:      "/api?file=img_photo.jpg&file=doc_file.pdf",
			expected: "/api?file=img_photo.jpg",
		},
		{
			name:     "filter URL with suffix pattern",
			rules:    "/api?file=[in:*.jpg]",
			url:      "/api?file=photo.jpg&file=document.pdf",
			expected: "/api?file=photo.jpg",
		},
		{
			name:     "filter URL with contains pattern",
			rules:    "/api?id=[in:*user*]",
			url:      "/api?id=new_user_123&id=admin_123",
			expected: "/api?id=new_user_123",
		},
		{
			name:     "filter URL with complex pattern",
			rules:    "/api?key=[in:prefix_*_suffix]",
			url:      "/api?key=prefix_value_suffix&key=prefix_value",
			expected: "/api?key=prefix_value_suffix",
		},
		{
			name:     "filter URL with multiple pattern constraints",
			rules:    "/api?file=[in:img_*]&id=[in:*user*]",
			url:      "/api?file=img_photo.jpg&id=new_user_123&invalid=value",
			expected: "/api?file=img_photo.jpg&id=new_user_123",
		},
		{
			name:     "filter URL remove all invalid parameters",
			rules:    "/api?file=[in:img_*]&id=[in:*user*]",
			url:      "/api?file=doc_file.pdf&id=admin_123&invalid=value",
			expected: "/api",
		},
		{
			name:     "filter URL with mixed valid and invalid values",
			rules:    "/api?file=[in:img_*]&id=[in:*user*]",
			url:      "/api?file=img_photo.jpg&file=doc.pdf&id=new_user_123&id=admin_456",
			expected: "/api?file=img_photo.jpg&id=new_user_123",
		},
		{
			name:     "filter URL with wildcard pattern",
			rules:    "/api?any=[in:*]",
			url:      "/api?any=anything&any=nothing",
			expected: "/api?any=anything&any=nothing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rules, WithPlugins(patternPlugin))
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

func TestPatternEdgeCases(t *testing.T) {
	plugin := plugins.NewPatternPlugin()

	tests := []struct {
		name       string
		constraint string
		expectErr  bool
	}{
		{
			name:       "valid prefix pattern",
			constraint: "in:start*",
		},
		{
			name:       "valid suffix pattern",
			constraint: "in:*end",
		},
		{
			name:       "valid contains pattern",
			constraint: "in:*val*",
		},
		{
			name:       "any string pattern",
			constraint: "in:*",
		},
		{
			name:       "multiple wildcards only",
			constraint: "in:**",
		},
		{
			name:       "empty constraint should not parse",
			constraint: "",
			expectErr:  true,
		},
		{
			name:       "only prefix should not parse",
			constraint: "in:",
			expectErr:  true,
		},
		{
			name:       "complex multiple parts",
			constraint: "in:*one*two*three*",
		},
		{
			name:       "pattern with special characters",
			constraint: "in:*.*+?[](){}|^$\\*",
		},
		{
			name:       "pattern without wildcard should not parse",
			constraint: "in:nowildcard",
			expectErr:  true,
		},
		{
			name:       "too long pattern should not parse",
			constraint: "in:a*" + string(make([]byte, 1001)),
			expectErr:  true,
		},
		{
			name:       "wrong prefix should not parse",
			constraint: "len:*",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := plugin.Parse("test", tt.constraint)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Parse(%q) should have failed but succeeded", tt.constraint)
				}
			} else {
				if err != nil {
					t.Errorf("Parse(%q) failed: %v", tt.constraint, err)
				} else if validator == nil {
					t.Errorf("Parse(%q) returned nil validator", tt.constraint)
				}
			}
		})
	}
}

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
		b.Fatalf("Failed to parse rules: %v", err)
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

func BenchmarkPatternPluginValidateURL(b *testing.B) {
	patternPlugin := plugins.NewPatternPlugin()
	pv, err := NewParamValidator("/api?file=[in:img_*]&id=[in:*user*]", WithPlugins(patternPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateURL("/api?file=img_photo.jpg&id=new_user_123")
	}
}

func BenchmarkPatternPluginFilterURL(b *testing.B) {
	patternPlugin := plugins.NewPatternPlugin()
	pv, err := NewParamValidator("/api?file=[in:img_*]&id=[in:*user*]", WithPlugins(patternPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterURL("/api?file=img_photo.jpg&id=new_user_123&invalid=value")
	}
}
