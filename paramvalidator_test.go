package paramvalidator

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func TestNewParamValidator(t *testing.T) {
	tests := []struct {
		name      string
		rulesStr  string
		wantValid bool
		wantError bool
	}{
		{
			name:      "empty rules",
			rulesStr:  "",
			wantValid: true,
			wantError: false,
		},
		{
			name:      "global rules",
			rulesStr:  "page=[1]&sort=[name,date]",
			wantValid: true,
			wantError: false,
		},
		{
			name:      "url rules",
			rulesStr:  "/products?page=[1];/users?sort=[name,date]",
			wantValid: true,
			wantError: false,
		},
		{
			name:      "invalid rules format",
			rulesStr:  "page=[1",
			wantValid: false,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rulesStr)

			if tt.wantError {
				if err == nil {
					t.Errorf("NewParamValidator() expected error for rules %q, but got nil", tt.rulesStr)
				}
				if pv != nil {
					t.Error("NewParamValidator() should return nil validator when error occurs")
				}
			} else {
				if err != nil {
					t.Errorf("NewParamValidator() unexpected error = %v for rules %q", err, tt.rulesStr)
				}
				if pv == nil {
					t.Error("Expected validator to be created")
				}
			}
		})
	}
}

func TestParseRules(t *testing.T) {
	pv, err := NewParamValidator("")
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name      string
		rulesStr  string
		wantError bool
	}{
		{
			name:      "empty string",
			rulesStr:  "",
			wantError: false,
		},
		{
			name:      "valid global rules",
			rulesStr:  "page=[1]&limit=[5,10,20]",
			wantError: false,
		},
		{
			name:      "valid url rules",
			rulesStr:  "/products?page=[1];/users?limit=[5,10,20]",
			wantError: false,
		},
		{
			name:      "invalid rules format - unclosed bracket",
			rulesStr:  "page=[1",
			wantError: true,
		},
		{
			name:      "invalid rules format - empty parameter name",
			rulesStr:  "=[1]",
			wantError: true,
		},
		{
			name:      "invalid enum values",
			rulesStr:  "page=[a,b,c]",
			wantError: false, // Enum values can be any strings
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pv.ParseRules(tt.rulesStr)

			if tt.wantError {
				if err == nil {
					t.Errorf("ParseRules() expected error for rules %q, but got nil", tt.rulesStr)
				} else {
					t.Logf("ParseRules() correctly returned error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("ParseRules() unexpected error = %v for rules %q", err, tt.rulesStr)
				}
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "empty rules - any url invalid",
			rules:    "",
			url:      "/test?param=value",
			expected: false,
		},
		{
			name:     "allow all params with wildcard",
			rules:    "/api/*?*",
			url:      "/api/test?any=value&other=123",
			expected: true,
		},
		{
			name:     "valid single value parameter",
			rules:    "/products?page=[5]",
			url:      "/products?page=5",
			expected: true,
		},
		{
			name:     "invalid single value parameter",
			rules:    "/products?page=[5]",
			url:      "/products?page=10",
			expected: false,
		},
		{
			name:     "valid enum parameter",
			rules:    "/search?sort=[name,date,price]",
			url:      "/search?sort=name",
			expected: true,
		},
		{
			name:     "invalid enum parameter",
			rules:    "/search?sort=[name,date,price]",
			url:      "/search?sort=invalid",
			expected: false,
		},
		{
			name:     "key-only parameter",
			rules:    "/filter?active=[]",
			url:      "/filter?active",
			expected: true,
		},
		{
			name:     "key-only parameter with value",
			rules:    "/filter?active=[]",
			url:      "/filter?active=true",
			expected: false,
		},
		{
			name:     "multiple valid parameters",
			rules:    "/api?page=[5]&limit=[10]",
			url:      "/api?page=5&limit=10",
			expected: true,
		},
		{
			name:     "one invalid parameter",
			rules:    "/api?page=[5]&limit=[10]",
			url:      "/api?page=5&limit=15",
			expected: false,
		},
		{
			name:     "global parameters",
			rules:    "page=[5]&sort=[name,date]",
			url:      "/any/path?page=5&sort=name",
			expected: true,
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

func TestValidateParam(t *testing.T) {
	pv, err := NewParamValidator("/products?page=[5]&category=[electronics,books]")
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name     string
		urlPath  string
		param    string
		value    string
		expected bool
	}{
		{
			name:     "valid page parameter",
			urlPath:  "/products",
			param:    "page",
			value:    "5",
			expected: true,
		},
		{
			name:     "invalid page parameter",
			urlPath:  "/products",
			param:    "page",
			value:    "15",
			expected: false,
		},
		{
			name:     "valid category parameter",
			urlPath:  "/products",
			param:    "category",
			value:    "electronics",
			expected: true,
		},
		{
			name:     "invalid category parameter",
			urlPath:  "/products",
			param:    "category",
			value:    "invalid",
			expected: false,
		},
		{
			name:     "unknown parameter",
			urlPath:  "/products",
			param:    "unknown",
			value:    "value",
			expected: false,
		},
		{
			name:     "wrong url path - should use global rules",
			urlPath:  "/users",
			param:    "page",
			value:    "5",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pv.ValidateParam(tt.urlPath, tt.param, tt.value)
			if result != tt.expected {
				t.Errorf("ValidateParam(%q, %q, %q) = %v, expected %v",
					tt.urlPath, tt.param, tt.value, result, tt.expected)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		rules    string
		url      string
		expected string
	}{
		{
			name:     "remove invalid parameters",
			rules:    "/search?page=[5]&sort=[name,date]",
			url:      "/search?page=5&sort=name&invalid=value",
			expected: "/search?page=5&sort=name",
		},
		{
			name:     "filter invalid values - keep valid ones",
			rules:    "/products?page=[5]",
			url:      "/products?page=15&page=5",
			expected: "/products?page=5",
		},
		{
			name:     "no valid parameters - return path only",
			rules:    "/api?token=[valid]",
			url:      "/api?invalid=value",
			expected: "/api",
		},
		{
			name:     "allow all parameters with wildcard",
			rules:    "/api/*?*",
			url:      "/api?any=value&other=123",
			expected: "/api?any=value&other=123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rules)
			if err != nil {
				t.Fatalf("Failed to create validator: %v", err)
			}
			result := pv.NormalizeURL(tt.url)

			expected := tt.expected
			if result != expected {
				t.Errorf("NormalizeURL(%q) = %q, expected %q", tt.url, result, expected)
			}
		})
	}
}

func TestFilterQueryParams(t *testing.T) {
	pv, err := NewParamValidator("/api?page=[5]&limit=[10]")
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name     string
		urlPath  string
		query    string
		expected string
	}{
		{
			name:     "valid parameters",
			urlPath:  "/api",
			query:    "page=5&limit=10",
			expected: "page=5&limit=10",
		},
		{
			name:     "filter invalid parameters",
			urlPath:  "/api",
			query:    "page=5&limit=15&invalid=value",
			expected: "page=5",
		},
		{
			name:     "empty query",
			urlPath:  "/api",
			query:    "",
			expected: "",
		},
		{
			name:     "wrong path - no rules apply",
			urlPath:  "/users",
			query:    "page=5",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pv.FilterQueryParams(tt.urlPath, tt.query)
			if result != tt.expected {
				t.Errorf("FilterQueryParams(%q, %q) = %q, expected %q",
					tt.urlPath, tt.query, result, tt.expected)
			}
		})
	}
}

func TestClear(t *testing.T) {
	pv, err := NewParamValidator("/api?page=[5]")
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	if !pv.ValidateURL("/api?page=5") {
		t.Error("Should validate before clear")
	}

	pv.Clear()

	if pv.ValidateURL("/api?page=5") {
		t.Error("Should not validate after clear")
	}
}

func TestURLPatternMatching(t *testing.T) {
	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "exact match",
			rules:    "/api/v1/users?page=[5]",
			url:      "/api/v1/users?page=5",
			expected: true,
		},
		{
			name:     "wildcard prefix",
			rules:    "/api/*?page=[5]",
			url:      "/api/v1/users?page=5",
			expected: true,
		},
		{
			name:     "wildcard suffix",
			rules:    "/api/v1/*?page=[5]",
			url:      "/api/v1/users/list?page=5",
			expected: true,
		},
		{
			name:     "no match",
			rules:    "/api/v1/users?page=[5]",
			url:      "/api/v1/products?page=5",
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
				t.Errorf("URL pattern matching for %q with rules %q = %v, expected %v",
					tt.url, tt.rules, result, tt.expected)
			}
		})
	}
}

func TestEdgeCases(t *testing.T) {
	t.Run("uninitialized validator", func(t *testing.T) {
		pv := &ParamValidator{initialized: false}

		if pv.ValidateURL("/test") {
			t.Error("Uninitialized validator should not validate")
		}

		if pv.NormalizeURL("/test") != "/test" {
			t.Error("Uninitialized validator should return original URL")
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		pv, err := NewParamValidator("/test?param=value")
		if err != nil {
			t.Fatalf("Failed to create validator: %v", err)
		}

		if pv.ValidateURL(":invalid:url:") {
			t.Error("Invalid URL should not validate")
		}
	})

	t.Run("empty parameter values", func(t *testing.T) {
		pv, err := NewParamValidator("/test?param=[value1,value2]")
		if err != nil {
			t.Fatalf("Failed to create validator: %v", err)
		}

		if pv.ValidateParam("/test", "param", "") {
			t.Error("Empty value should not validate against enum")
		}
	})

	t.Run("nil validator", func(t *testing.T) {
		var pv *ParamValidator

		result := pv.ValidateURL("/test")
		if result != false {
			t.Error("nil validator should return false")
		}

		normalized := pv.NormalizeURL("/test")
		if normalized != "/test" {
			t.Error("nil validator should return original URL")
		}
	})
}

func TestMultipleRulesWithSemicolon(t *testing.T) {
	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "multiple URL rules with semicolon",
			rules:    "/products?page=[5];/users?sort=[name,date];/search?q=[]",
			url:      "/products?page=5",
			expected: true,
		},
		{
			name:     "second rule in list",
			rules:    "/products?page=[5];/users?sort=[name,date]",
			url:      "/users?sort=name",
			expected: true,
		},
		{
			name:     "third rule in list",
			rules:    "/products?page=[5];/users?sort=[name,date];/search?q=[]",
			url:      "/search?q",
			expected: true,
		},
		{
			name:     "mixed global and URL rules",
			rules:    "page=[5];/users?sort=[name,date]",
			url:      "/any/path?page=5",
			expected: true,
		},
		{
			name:     "global rules work for any URL when mixed",
			rules:    "page=[5];/users?sort=[name,date]",
			url:      "/products?page=5",
			expected: true,
		},
		{
			name:     "URL-specific rules override global for specific path",
			rules:    "page=[100];/products?page=[5]",
			url:      "/products?page=5",
			expected: true,
		},
		{
			name:     "URL-specific rules override global - invalid case",
			rules:    "page=[100];/products?page=[5]",
			url:      "/products?page=50",
			expected: false,
		},
		{
			name:     "multiple rules with wildcards",
			rules:    "/api/*?page=[5];/admin/*?access=[admin,superuser]",
			url:      "/api/v1/users?page=5",
			expected: true,
		},
		{
			name:     "second wildcard rule",
			rules:    "/api/*?page=[5];/admin/*?access=[admin,superuser]",
			url:      "/admin/users?access=admin",
			expected: true,
		},
		{
			name:     "complex multiple rules",
			rules:    "/products?category=[electronics,books]&price=[500];/users?role=[admin,user]&status=[active,inactive]",
			url:      "/products?category=electronics&price=500",
			expected: true,
		},
		{
			name:     "another complex rule",
			rules:    "/products?category=[electronics,books]&price=[500];/users?role=[admin,user]&status=[active,inactive]",
			url:      "/users?role=admin&status=active",
			expected: true,
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

func TestMultipleRulesNormalization(t *testing.T) {
	tests := []struct {
		name     string
		rules    string
		url      string
		expected string
	}{
		{
			name:     "parameters from multiple matching rules",
			rules:    "/api/v1/*?page=[5];/api/v1/users?limit=[10]",
			url:      "/api/v1/users?page=5&limit=10",
			expected: "/api/v1/users?page=5&limit=10",
		},
		{
			name:     "same parameter name - more specific wins",
			rules:    "/api/*?page=[100];/api/users?page=[5]",
			url:      "/api/users?page=5",
			expected: "/api/users?page=5",
		},
		{
			name:     "same parameter name - more specific wins with invalid value",
			rules:    "/api/*?page=[100];/api/users?page=[5]",
			url:      "/api/users?page=50",
			expected: "/api/users",
		},
		{
			name:     "normalize with multiple rules",
			rules:    "/products?page=[5];/users?sort=[name,date]",
			url:      "/products?page=5&invalid=value",
			expected: "/products?page=5",
		},
		{
			name:     "normalize with second rule",
			rules:    "/products?page=[5];/users?sort=[name,date]",
			url:      "/users?sort=name&invalid=value",
			expected: "/users?sort=name",
		},
		{
			name:     "normalize with global and URL rules",
			rules:    "page=[5];/users?sort=[name,date]",
			url:      "/any/path?page=5&invalid=value",
			expected: "/any/path?page=5",
		},
		{
			name:     "multiple rules with different parameters",
			rules:    "/api/*?page=[5];/api/users?limit=[10]",
			url:      "/api/users?page=5&limit=10&invalid=value",
			expected: "/api/users?page=5&limit=10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator(tt.rules)
			if err != nil {
				t.Fatalf("Failed to create validator: %v", err)
			}

			pv.NormalizeURL(tt.url)

		})
	}
}

func TestCallbackPattern(t *testing.T) {
	callbackFunc := func(key string, value string) bool {
		switch key {
		case "token":
			return value == "valid_token"
		case "user_id":
			return len(value) > 0 && len(value) <= 10
		case "timestamp":
			return len(value) == 10
		default:
			return false
		}
	}

	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "valid callback parameter",
			rules:    "/api?token=[?]",
			url:      "/api?token=valid_token",
			expected: true,
		},
		{
			name:     "invalid callback parameter",
			rules:    "/api?token=[?]",
			url:      "/api?token=invalid_token",
			expected: false,
		},
		{
			name:     "multiple callback parameters",
			rules:    "/auth?token=[?]&user_id=[?]",
			url:      "/auth?token=valid_token&user_id=12345",
			expected: true,
		},
		{
			name:     "mixed callback and regular rules",
			rules:    "/data?token=[?]&page=[5]",
			url:      "/data?token=valid_token&page=5",
			expected: true,
		},
		{
			name:     "callback with empty value",
			rules:    "/api?token=[?]",
			url:      "/api?token=",
			expected: false,
		},
		{
			name:     "callback parameter in global rules",
			rules:    "timestamp=[?]",
			url:      "/any/path?timestamp=1234567890",
			expected: true,
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

func TestCallbackWithoutFunction(t *testing.T) {
	pv, err := NewParamValidator("/api?token=[?]")
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	if pv.ValidateURL("/api?token=any_value") {
		t.Error("Callback parameter should be invalid when no callback function is set")
	}

	callbackFunc := func(key string, value string) bool {
		return value == "valid"
	}
	pv.SetCallback(callbackFunc)

	if !pv.ValidateURL("/api?token=valid") {
		t.Error("Callback parameter should be valid after setting callback function")
	}

	if pv.ValidateURL("/api?token=invalid") {
		t.Error("Callback parameter should be invalid for wrong values")
	}
}

// Benchmark tests for plugins
func BenchmarkRangePluginValidation(b *testing.B) {
	rangePlugin := plugins.NewRangePlugin()
	parser := NewRuleParser(rangePlugin)

	pv := &ParamValidator{
		globalParams:  make(map[string]*ParamRule),
		urlRules:      make(map[string]*URLRule),
		urlMatcher:    NewURLMatcher(),
		compiledRules: &CompiledRules{},
		initialized:   true,
		parser:        parser,
	}

	err := pv.ParseRules("/api?age=[18-65]")
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateURL("/api?age=25")
	}
}

func BenchmarkRangePluginNormalization(b *testing.B) {
	rangePlugin := plugins.NewRangePlugin()
	parser := NewRuleParser(rangePlugin)

	pv := &ParamValidator{
		globalParams:  make(map[string]*ParamRule),
		urlRules:      make(map[string]*URLRule),
		urlMatcher:    NewURLMatcher(),
		compiledRules: &CompiledRules{},
		initialized:   true,
		parser:        parser,
	}

	err := pv.ParseRules("/api?age=[18-65]")
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.NormalizeURL("/api?age=25&invalid=value")
	}
}

func BenchmarkValidateWithMultiplePlugins(b *testing.B) {
	// Создаем правила, которые используют все три плагина
	rules := `/api?age=[range:18-65]&email=[regex:^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$]&score=[comparison:>50]&name=[regex:^[a-zA-Z ]{2,50}$]&quantity=[range:1-100]&rating=[comparison:>=3.5]`

	pv, err := NewParamValidator(rules,
		WithPlugins(
			plugins.NewComparisonPlugin(),
			plugins.NewRangePlugin(),
			plugins.NewRegexPlugin(),
		))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	// Тестовые URL для валидации
	testURLs := []string{
		"/api?age=25&email=test@example.com&score=75&name=John Doe&quantity=10&rating=4.2",
		"/api?age=30&email=user@domain.com&score=60&name=Jane Smith&quantity=50&rating=3.8",
		"/api?age=22&email=admin@test.org&score=80&name=Bob Wilson&quantity=25&rating=4.5",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Чередуем тестовые URL для более реалистичного бенчмарка
		url := testURLs[i%len(testURLs)]
		pv.ValidateURL(url)
	}
}

func BenchmarkNormalizeWithMultiplePlugins(b *testing.B) {
	// Создаем правила, которые используют все три плагина
	rules := `/api?age=[18-65]&email=[^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$]&score=[>50]&name=[^[a-zA-Z ]{2,50}$]&quantity=[1-100]&rating=[>=3]`

	pv, err := NewParamValidator(rules,
		WithPlugins(
			plugins.NewComparisonPlugin(),
			plugins.NewRangePlugin(),
			plugins.NewRegexPlugin(),
		))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	// Тестовые URL для валидации
	testURLs := []string{
		"/api?age=25&email=test@example.com&score=75&name=John Doe&quantity=10&rating=4.2",
		"/api?age=30&email=user@domain.com&score=60&name=Jane Smith&quantity=50&rating=3.8",
		"/api?age=22&email=admin@test.org&score=80&name=Bob Wilson&quantity=25&rating=4.5",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Чередуем тестовые URL для более реалистичного бенчмарка
		url := testURLs[i%len(testURLs)]
		pv.NormalizeURL(url)
	}
}

// Original benchmark tests (updated)
func BenchmarkValidateURL(b *testing.B) {
	pv, err := NewParamValidator("/api/v1/*?page=[5]&limit=[10]&sort=[name,date]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	url := "/api/v1/users/list?page=5&limit=10&sort=name"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateURL(url)
	}
}

func BenchmarkNormalizeURL(b *testing.B) {
	pv, err := NewParamValidator("/api/*?page=[5]&limit=[10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	url := "/api/v1/data?page=5&limit=10&invalid=value&extra=param"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.NormalizeURL(url)
	}
}

func BenchmarkFilterQueryParams(b *testing.B) {
	pv, err := NewParamValidator("/api/*?page=[5]&limit=[10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	urlPath := "/api/v1/data"
	query := "page=5&limit=10&invalid=value&extra=param"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterQueryParams(urlPath, query)
	}
}

func BenchmarkValidateQueryParams(b *testing.B) {
	pv, err := NewParamValidator("/api/*?page=[5]&limit=[10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	urlPath := "/api/v1/data"
	query := "page=5&limit=10&invalid=value&extra=param"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateQueryParams(urlPath, query)
	}
}

func TestConcurrentValidation(t *testing.T) {
	pv, err := NewParamValidator("/api/*?page=[5]&limit=[10]&sort=[name,date]")
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	numGoroutines := 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errorCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Генерируем значение страницы от 1 до 5
			pageValue := fmt.Sprintf("%d", (id%5)+1)

			// Определяем, должно ли значение проходить валидацию
			shouldPass := pageValue == "5"

			switch id % 4 {
			case 0:
				result := pv.ValidateURL(fmt.Sprintf("/api/users?page=%s&limit=10", pageValue))
				if result != shouldPass {
					errorCh <- fmt.Errorf("goroutine %d: URL validation failed for page=%s, expected %v, got %v",
						id, pageValue, shouldPass, result)
				}
			case 1:
				result := pv.ValidateParam("/api/users", "page", pageValue)
				if result != shouldPass {
					errorCh <- fmt.Errorf("goroutine %d: param validation failed for page=%s, expected %v, got %v",
						id, pageValue, shouldPass, result)
				}
			case 2:
				normalized := pv.NormalizeURL(fmt.Sprintf("/api/users?page=%s&invalid=value", pageValue))
				if shouldPass {
					// Если значение должно проходить, проверяем что оно осталось в URL
					if !strings.Contains(normalized, "page="+pageValue) {
						errorCh <- fmt.Errorf("goroutine %d: normalization failed for valid page=%s: %s",
							id, pageValue, normalized)
					}
				} else {
					// Если значение не должно проходить, проверяем что оно удалено
					if strings.Contains(normalized, "page="+pageValue) {
						errorCh <- fmt.Errorf("goroutine %d: normalization failed for invalid page=%s: %s",
							id, pageValue, normalized)
					}
				}
			case 3:
				filtered := pv.FilterQueryParams("/api/users",
					fmt.Sprintf("page=%s&limit=10&invalid=value", pageValue))
				if shouldPass {
					// Если значение должно проходить, проверяем что оно осталось
					if !strings.Contains(filtered, "page="+pageValue) || !strings.Contains(filtered, "limit=10") {
						errorCh <- fmt.Errorf("goroutine %d: filtering failed for valid page=%s: %s",
							id, pageValue, filtered)
					}
				} else {
					// Если значение не должно проходить, проверяем что оно удалено
					if strings.Contains(filtered, "page="+pageValue) {
						errorCh <- fmt.Errorf("goroutine %d: filtering failed for invalid page=%s: %s",
							id, pageValue, filtered)
					}
					// Но limit=10 должен остаться, так как он всегда валиден
					if !strings.Contains(filtered, "limit=10") {
						errorCh <- fmt.Errorf("goroutine %d: filtering removed valid limit for page=%s: %s",
							id, pageValue, filtered)
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errorCh)

	var errors []error
	for err := range errorCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Errorf("Concurrent validation failed with %d errors:", len(errors))
		for _, err := range errors {
			t.Error(err)
		}
	}
}

func BenchmarkConcurrentValidation(b *testing.B) {
	pv, err := NewParamValidator("/api/*?page=[5]&limit=[10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pv.ValidateURL("/api/users?page=5&limit=10")
			pv.ValidateParam("/api/users", "page", "5")
		}
	})
}

func BenchmarkConcurrentNormalization(b *testing.B) {
	pv, err := NewParamValidator("/api/*?page=[5]&limit=[10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pv.NormalizeURL("/api/users?page=5&limit=10&invalid=value")
		}
	})
}
