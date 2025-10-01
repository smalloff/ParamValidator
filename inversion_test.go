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

func TestInvertedKeyOnlyRules(t *testing.T) {
	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "inverted key-only - allow only with value",
			rules:    "/api?flag=![]",
			url:      "/api?flag=true",
			expected: true,
		},
		{
			name:     "inverted key-only - block without value",
			rules:    "/api?flag=![]",
			url:      "/api?flag",
			expected: false,
		},
		{
			name:     "inverted key-only - block empty value",
			rules:    "/api?flag=![]",
			url:      "/api?flag=",
			expected: false,
		},
		{
			name:     "normal key-only vs inverted key-only",
			rules:    "/api?optional=[]&required=![]",
			url:      "/api?optional&required=yes",
			expected: true,
		},
		{
			name:     "normal key-only vs inverted key-only - fails",
			rules:    "/api?optional=[]&required=![]",
			url:      "/api?optional&required",
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

func TestInvertedAnyRules(t *testing.T) {
	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "inverted any - block all values",
			rules:    "/api?blocked=![*]",
			url:      "/api?blocked=anyvalue",
			expected: false,
		},
		{
			name:     "inverted any - block parameter existence",
			rules:    "/api?blocked=![*]",
			url:      "/api?other=value",
			expected: false,
		},
		{
			name:     "inverted any - block empty value",
			rules:    "/api?blocked=![*]",
			url:      "/api?blocked=",
			expected: true,
		},
		{
			name:     "inverted any - block key-only",
			rules:    "/api?blocked=![*]",
			url:      "/api?blocked",
			expected: true,
		},
		{
			name:     "mixed inverted any and normal any",
			rules:    "/api?allowed=[*]&blocked=![*]",
			url:      "/api?allowed=anything",
			expected: true,
		},
		{
			name:     "mixed inverted any and normal any - blocked",
			rules:    "/api?allowed=[*]&blocked=![*]",
			url:      "/api?allowed=anything&blocked=value",
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
		{
			name:     "inverted key-only - remove key-only parameter",
			rules:    "/api?flag=![]&value[*]",
			url:      "/api?flag&value=1",
			expected: "/api?value=1",
		},
		{
			name:     "inverted any - remove all values of parameter",
			rules:    "/api?blocked=![*]",
			url:      "/api?blocked&blocked=value1&blocked=value2&allowed=ok",
			expected: "/api?blocked",
		},
		{
			name:     "inverted any with key-only - remove parameter completely",
			rules:    "/api?blocked=![*]",
			url:      "/api?blocked&allowed=ok",
			expected: "/api?blocked",
		},
		{
			name:     "multiple inverted rules - complex filtering",
			rules:    "/api?category=![electronics]&status=![inactive]&mode=![]",
			url:      "/api?category=electronics&category=books&status=active&status=inactive&mode&mode=auto",
			expected: "/api?category=books&status=active&mode=auto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rules)
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

func TestInvertedCallbackRules(t *testing.T) {
	callbackFunc := func(key string, value string) bool {
		return value == "valid" || value == "approved"
	}

	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "inverted callback - block valid values",
			rules:    "/api?token=![?]",
			url:      "/api?token=valid",
			expected: false,
		},
		{
			name:     "inverted callback - allow invalid values",
			rules:    "/api?token=![?]",
			url:      "/api?token=invalid",
			expected: true,
		},
		{
			name:     "inverted callback - mixed values",
			rules:    "/api?token=![?]",
			url:      "/api?token=valid&token=invalid",
			expected: false,
		},
		{
			name:     "normal callback vs inverted callback",
			rules:    "/api?allowed=[?]&blocked=![?]",
			url:      "/api?allowed=valid&blocked=invalid",
			expected: true,
		},
		{
			name:     "normal callback vs inverted callback - fails",
			rules:    "/api?allowed=[?]&blocked=![?]",
			url:      "/api?allowed=valid&blocked=valid",
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

func TestInvertedGlobalRules(t *testing.T) {
	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "global inverted enum",
			rules:    "category=![electronics]",
			url:      "/api?category=books",
			expected: true,
		},
		{
			name:     "global inverted enum - blocked",
			rules:    "category=![electronics]",
			url:      "/api?category=electronics",
			expected: false,
		},
		{
			name:     "global inverted any",
			rules:    "blocked=![*]&allowed[value]",
			url:      "/any/path?blocked&allowed=value",
			expected: true,
		},
		{
			name:     "global inverted any - blocked",
			rules:    "blocked=![*]",
			url:      "/any/path?blocked=value",
			expected: false,
		},
		{
			name:     "mixed global and url-specific inverted rules",
			rules:    "global_blocked=![*];/api?local_blocked=![*];other=[value]",
			url:      "/api?other=value&global_blocked&local_blocked",
			expected: true,
		},
		{
			name:     "mixed global and url-specific inverted rules - global blocked",
			rules:    "global_blocked=![*];/api?local_blocked=![*]",
			url:      "/api?global_blocked=value",
			expected: false,
		},
		{
			name:     "mixed global and url-specific inverted rules - local blocked",
			rules:    "global_blocked=![*];/api?local_blocked=![*]",
			url:      "/api?local_blocked=value",
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

func TestInvertedRulesEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "empty inverted constraint - should behave like inverted any",
			rules:    "/api?param=![]",
			url:      "/api?param=value",
			expected: true,
		},
		{
			name:     "inverted with special characters",
			rules:    "/api?email=![test@example.com,admin@site.org]",
			url:      "/api?email=user@domain.com",
			expected: true,
		},
		{
			name:     "inverted with special characters - blocked",
			rules:    "/api?email=![test@example.com,admin@site.org]",
			url:      "/api?email=test@example.com",
			expected: false,
		},
		{
			name:     "multiple values with inverted enum",
			rules:    "/api?codes=![404,500]",
			url:      "/api?codes=200&codes=301",
			expected: true,
		},
		{
			name:     "multiple values with inverted enum - one blocked",
			rules:    "/api?codes=![404,500]",
			url:      "/api?codes=200&codes=404",
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
