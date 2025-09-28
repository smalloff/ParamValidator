package paramvalidator

import (
	"testing"
)

func TestInvertedRules(t *testing.T) {
	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "inverted enum - allow all except specific values",
			rules:    "/api?category=![electronics,books]",
			url:      "/api?category=clothing",
			expected: true,
		},
		{
			name:     "inverted enum - block specific values",
			rules:    "/api?category=![electronics,books]",
			url:      "/api?category=electronics",
			expected: false,
		},
		{
			name:     "inverted single value - allow all except one",
			rules:    "/search?q=![test]",
			url:      "/search?q=hello",
			expected: true,
		},
		{
			name:     "inverted single value - block specific value",
			rules:    "/search?q=![test]",
			url:      "/search?q=test",
			expected: false,
		},
		{
			name:     "mixed normal and inverted rules",
			rules:    "/api?page=[5]&sort=![name]",
			url:      "/api?page=5&sort=date",
			expected: true,
		},
		{
			name:     "mixed normal and inverted rules - blocked by inverted",
			rules:    "/api?page=[5]&sort=![name]",
			url:      "/api?page=5&sort=name",
			expected: false,
		},
		{
			name:     "multiple inverted parameters",
			rules:    "/api?category=![electronics]&status=![inactive]",
			url:      "/api?category=books&status=active",
			expected: true,
		},
		{
			name:     "multiple inverted parameters - one fails",
			rules:    "/api?category=![electronics]&status=![inactive]",
			url:      "/api?category=books&status=inactive",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rules)
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

func TestInvertedRulesNormalization(t *testing.T) {
	tests := []struct {
		name     string
		rules    string
		url      string
		expected string
	}{
		{
			name:     "inverted enum - remove blocked values",
			rules:    "/api?category=![electronics,books]",
			url:      "/api?category=electronics&category=clothing",
			expected: "/api?category=clothing",
		},
		{
			name:     "inverted single value - remove blocked value",
			rules:    "/search?q=![test]",
			url:      "/search?q=test&q=hello",
			expected: "/search?q=hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rules)
			if err != nil {
				t.Fatalf("Failed to create validator: %v", err)
			}

			result := pv.NormalizeURL(tt.url)
			if result != tt.expected {
				t.Errorf("NormalizeURL(%q) with rules %q = %q, expected %q",
					tt.url, tt.rules, result, tt.expected)
			}
		})
	}
}

func TestInvertedCallbackRules(t *testing.T) {
	callbackFunc := func(key string, value string) bool {
		return value == "valid"
	}

	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "inverted callback - allow invalid values",
			rules:    "/api?token=![?]",
			url:      "/api?token=valid",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rules, WithCallback(callbackFunc))
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
