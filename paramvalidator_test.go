package paramvalidator

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewParamValidator(t *testing.T) {
	tests := []struct {
		name      string
		rulesStr  string
		wantValid bool
	}{
		{
			name:      "empty rules",
			rulesStr:  "",
			wantValid: true,
		},
		{
			name:      "global rules",
			rulesStr:  "page=[1-10]&sort=[name,date]",
			wantValid: true,
		},
		{
			name:      "url rules",
			rulesStr:  "/products?page=[1-10];/users?sort=[name,date]",
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv := NewParamValidator(tt.rulesStr)
			if pv == nil {
				t.Fatal("Expected validator to be created")
			}
		})
	}
}

func TestParseRules(t *testing.T) {
	pv := NewParamValidator("")

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
			rulesStr:  "page=[1-10]&limit=[5,10,20]",
			wantError: false,
		},
		{
			name:      "valid url rules",
			rulesStr:  "/products?page=[1-10];/users?limit=[5,10,20]",
			wantError: false,
		},
		{
			name:      "invalid rules format - unclosed bracket",
			rulesStr:  "page=[1-10",
			wantError: true,
		},
		{
			name:      "invalid rules format - empty parameter name",
			rulesStr:  "=[1-10]",
			wantError: true,
		},
		{
			name:      "invalid range format",
			rulesStr:  "page=[1-10-20]",
			wantError: true,
		},
		{
			name:      "invalid range values",
			rulesStr:  "page=[a-z]",
			wantError: true,
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
			name:     "valid range parameter",
			rules:    "/products?page=[1-10]",
			url:      "/products?page=5",
			expected: true,
		},
		{
			name:     "invalid range parameter",
			rules:    "/products?page=[1-10]",
			url:      "/products?page=15",
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
			rules:    "/api?page=[1-10]&limit=[5,10,20]",
			url:      "/api?page=5&limit=10",
			expected: true,
		},
		{
			name:     "one invalid parameter",
			rules:    "/api?page=[1-10]&limit=[5,10,20]",
			url:      "/api?page=5&limit=15",
			expected: false,
		},
		{
			name:     "global parameters",
			rules:    "page=[1-10]&sort=[name,date]",
			url:      "/any/path?page=5&sort=name",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv := NewParamValidator(tt.rules)
			result := pv.ValidateURL(tt.url)
			if result != tt.expected {
				t.Errorf("ValidateURL(%q) with rules %q = %v, expected %v",
					tt.url, tt.rules, result, tt.expected)
			}
		})
	}
}

func TestValidateParam(t *testing.T) {
	pv := NewParamValidator("/products?page=[1-10]&category=[electronics,books]")

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
			rules:    "/search?page=[1-10]&sort=[name,date]",
			url:      "/search?page=5&sort=name&invalid=value",
			expected: "/search?page=5&sort=name",
		},
		{
			name:     "filter invalid values - keep valid ones",
			rules:    "/products?page=[1-10]",
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
			pv := NewParamValidator(tt.rules)
			result := pv.NormalizeURL(tt.url)

			expected := tt.expected
			if result != expected {
				t.Errorf("NormalizeURL(%q) = %q, expected %q", tt.url, result, expected)
			}
		})
	}
}

func TestFilterQueryParams(t *testing.T) {
	pv := NewParamValidator("/api?page=[1-10]&limit=[5,10]")

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

func TestAddURLRule(t *testing.T) {
	pv := NewParamValidator("")

	rule := &ParamRule{
		Name:    "page",
		Pattern: PatternRange,
		Min:     1,
		Max:     10,
	}
	params := map[string]*ParamRule{"page": rule}
	pv.AddURLRule("/test", params)

	if !pv.ValidateURL("/test?page=5") {
		t.Error("Added URL rule should validate correctly")
	}

	if pv.ValidateURL("/test?page=15") {
		t.Error("Added URL rule should reject invalid values")
	}
}

func TestClear(t *testing.T) {
	pv := NewParamValidator("/api?page=[1-10]")

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
			rules:    "/api/v1/users?page=[1-10]",
			url:      "/api/v1/users?page=5",
			expected: true,
		},
		{
			name:     "wildcard prefix",
			rules:    "/api/*?page=[1-10]",
			url:      "/api/v1/users?page=5",
			expected: true,
		},
		{
			name:     "wildcard suffix",
			rules:    "/api/v1/*?page=[1-10]",
			url:      "/api/v1/users/list?page=5",
			expected: true,
		},
		{
			name:     "no match",
			rules:    "/api/v1/users?page=[1-10]",
			url:      "/api/v1/products?page=5",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv := NewParamValidator(tt.rules)
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
		pv := NewParamValidator("/test?param=value")

		if pv.ValidateURL(":invalid:url:") {
			t.Error("Invalid URL should not validate")
		}
	})

	t.Run("empty parameter values", func(t *testing.T) {
		pv := NewParamValidator("/test?param=[value1,value2]")

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

func BenchmarkValidateURL(b *testing.B) {
	pv := NewParamValidator("/api/v1/*?page=[1-100]&limit=[10,20,50]&sort=[name,date]")
	url := "/api/v1/users/list?page=50&limit=20&sort=name"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateURL(url)
	}
}

func BenchmarkNormalizeURL(b *testing.B) {
	pv := NewParamValidator("/api/*?page=[1-100]&limit=[10,20,50]")
	url := "/api/v1/data?page=50&limit=20&invalid=value&extra=param"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.NormalizeURL(url)
	}
}

func BenchmarkFilterQueryParamsParallel(b *testing.B) {
	pv := NewParamValidator("/api/*?page=[1-100]&limit=[10,20,50]")
	urlPath := "/api/v1/data"
	query := "page=50&limit=20&invalid=value&extra=param"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pv.FilterQueryParams(urlPath, query)
		}
	})
}

func BenchmarkFilterQueryParams(b *testing.B) {
	pv := NewParamValidator("/api/*?page=[1-100]&limit=[10,20,50]")
	urlPath := "/api/v1/data"
	query := "page=50&limit=20&invalid=value&extra=param"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterQueryParams(urlPath, query)
	}
}

func TestConcurrentValidation(t *testing.T) {
	pv := NewParamValidator("/api/*?page=[1-100]&limit=[10,20,50]&sort=[name,date]")

	numGoroutines := 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errorCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			switch id % 4 {
			case 0:
				if !pv.ValidateURL(fmt.Sprintf("/api/users?page=%d&limit=10", id%100+1)) {
					errorCh <- fmt.Errorf("goroutine %d: URL validation failed", id)
				}
			case 1:
				if !pv.ValidateParam("/api/users", "page", fmt.Sprintf("%d", id%100+1)) {
					errorCh <- fmt.Errorf("goroutine %d: param validation failed", id)
				}
			case 2:
				normalized := pv.NormalizeURL(fmt.Sprintf("/api/users?page=%d&invalid=value", id%100+1))
				if !strings.Contains(normalized, "page=") {
					errorCh <- fmt.Errorf("goroutine %d: normalization failed: %s", id, normalized)
				}
			case 3:
				filtered := pv.FilterQueryParams("/api/users",
					fmt.Sprintf("page=%d&limit=10&invalid=value", id%100+1))
				if !strings.Contains(filtered, "page=") {
					errorCh <- fmt.Errorf("goroutine %d: filtering failed: %s", id, filtered)
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

func TestConcurrentRuleUpdates(t *testing.T) {
	pv := NewParamValidator("")

	numReaders := 50
	numWriters := 10
	var wg sync.WaitGroup
	wg.Add(numReaders + numWriters)

	errorCh := make(chan error, numReaders+numWriters)
	stopCh := make(chan struct{})

	for i := 0; i < numReaders; i++ {
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					pv.ValidateURL(fmt.Sprintf("/test%d?param=value", id))
					pv.ValidateParam("/test", "param", "value")
					pv.NormalizeURL(fmt.Sprintf("/test%d?param=value", id))
					time.Sleep(time.Microsecond * 10)
				}
			}
		}(i)
	}

	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				select {
				case <-stopCh:
					return
				default:
					urlParams := map[string]*ParamRule{
						"page": {
							Name:    "page",
							Pattern: PatternRange,
							Min:     int64(id * 10),
							Max:     int64(id*10 + 5),
						},
					}
					pv.AddURLRule(fmt.Sprintf("/api%d", id), urlParams)

					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}

	time.Sleep(time.Second)
	close(stopCh)
	wg.Wait()
	close(errorCh)

	if len(errorCh) > 0 {
		t.Errorf("Concurrent rule updates failed with %d errors", len(errorCh))
	}
}

func TestConcurrentParseRules(t *testing.T) {
	pv := NewParamValidator("")

	numGoroutines := 20
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			var rules string
			if id%2 == 0 {
				rules = fmt.Sprintf("/api%d?page=[1-10]&sort=[name,date]", id)
			} else {
				rules = fmt.Sprintf("page=[%d-%d]&limit=[5,10,20]", id, id+10)
			}

			if err := pv.ParseRules(rules); err != nil {
				t.Logf("Goroutine %d: ParseRules error: %v", id, err)
			}

			pv.ValidateURL(fmt.Sprintf("/api%d?page=5", id))
		}(i)
	}

	wg.Wait()

	if !pv.ValidateURL("/api0?page=5") && !pv.ValidateURL("/any?page=5") {
		t.Log("Validator state is consistent after concurrent updates")
	}
}

func TestRaceConditionDetection(t *testing.T) {
	pv := NewParamValidator("/initial?param=[value1,value2]")

	done := make(chan bool)

	go func() {
		for i := 0; i < 1000; i++ {
			pv.ValidateURL("/initial?param=value1")
			pv.NormalizeURL("/initial?param=value1&extra=value")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			pv.ParseRules(fmt.Sprintf("/updated%d?newparam=[1-10]", i))
		}
		done <- true
	}()

	<-done
	<-done
}

func TestConcurrentAccessAfterClear(t *testing.T) {
	pv := NewParamValidator("/api?page=[1-10]")

	// Уменьшаем количество итераций для теста
	var wg sync.WaitGroup
	wg.Add(3)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Горутина для очистки
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ { // Уменьшаем количество очисток
			select {
			case <-ctx.Done():
				return
			default:
				pv.Clear()
				time.Sleep(time.Millisecond * 50) // Увеличиваем задержку
			}
		}
	}()

	// Горутина для валидации
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ { // Уменьшаем количество итераций
			select {
			case <-ctx.Done():
				return
			default:
				pv.ValidateURL(fmt.Sprintf("/api?param%d=a", i%10))
				time.Sleep(time.Millisecond * 10)
			}
		}
	}()

	// Горутина для нормализации
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				pv.NormalizeURL(fmt.Sprintf("/api?param%d=a", i%10))
				time.Sleep(time.Millisecond * 10)
			}
		}
	}()

	// Ожидаем завершения с таймаутом
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("Concurrent operations completed successfully")
	case <-ctx.Done():
		t.Error("Test timed out - possible deadlock")
	}
}

func BenchmarkConcurrentValidation(b *testing.B) {
	pv := NewParamValidator("/api/*?page=[1-100]&limit=[10,20,50]")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pv.ValidateURL("/api/users?page=50&limit=10")
			pv.ValidateParam("/api/users", "page", "50")
		}
	})
}

func BenchmarkConcurrentNormalization(b *testing.B) {
	pv := NewParamValidator("/api/*?page=[1-100]&limit=[10,20,50]")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pv.NormalizeURL("/api/users?page=50&limit=10&invalid=value")
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
			rules:    "/products?page=[1-10];/users?sort=[name,date];/search?q=[]",
			url:      "/products?page=5",
			expected: true,
		},
		{
			name:     "second rule in list",
			rules:    "/products?page=[1-10];/users?sort=[name,date]",
			url:      "/users?sort=name",
			expected: true,
		},
		{
			name:     "third rule in list",
			rules:    "/products?page=[1-10];/users?sort=[name,date];/search?q=[]",
			url:      "/search?q",
			expected: true,
		},
		{
			name:     "mixed global and URL rules",
			rules:    "page=[1-10];/users?sort=[name,date]",
			url:      "/any/path?page=5",
			expected: true,
		},
		{
			name:     "global rules work for any URL when mixed",
			rules:    "page=[1-10];/users?sort=[name,date]",
			url:      "/products?page=5",
			expected: true,
		},
		{
			name:     "URL-specific rules override global for specific path",
			rules:    "page=[1-100];/products?page=[1-10]",
			url:      "/products?page=5",
			expected: true,
		},
		{
			name:     "URL-specific rules override global - invalid case",
			rules:    "page=[1-100];/products?page=[1-10]",
			url:      "/products?page=50",
			expected: false,
		},
		{
			name:     "multiple rules with wildcards",
			rules:    "/api/*?page=[1-10];/admin/*?access=[admin,superuser]",
			url:      "/api/v1/users?page=5",
			expected: true,
		},
		{
			name:     "second wildcard rule",
			rules:    "/api/*?page=[1-10];/admin/*?access=[admin,superuser]",
			url:      "/admin/users?access=admin",
			expected: true,
		},
		{
			name:     "complex multiple rules",
			rules:    "/products?category=[electronics,books]&price=[10-1000];/users?role=[admin,user]&status=[active,inactive]",
			url:      "/products?category=electronics&price=500",
			expected: true,
		},
		{
			name:     "another complex rule",
			rules:    "/products?category=[electronics,books]&price=[10-1000];/users?role=[admin,user]&status=[active,inactive]",
			url:      "/users?role=admin&status=active",
			expected: true,
		},
		{
			name:     "empty rules between semicolons",
			rules:    "/products?page=[1-10];;/users?sort=[name,date]",
			url:      "/users?sort=name",
			expected: true,
		},
		{
			name:     "trailing semicolon",
			rules:    "/products?page=[1-10];/users?sort=[name,date];",
			url:      "/products?page=5",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv := NewParamValidator(tt.rules)
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
			rules:    "/api/v1/*?page=[1-10];/api/v1/users?limit=[5,10]",
			url:      "/api/v1/users?page=5&limit=10",
			expected: "/api/v1/users?page=5&limit=10",
		},
		{
			name:     "same parameter name - more specific wins",
			rules:    "/api/*?page=[1-100];/api/users?page=[1-10]",
			url:      "/api/users?page=5",
			expected: "/api/users?page=5",
		},
		{
			name:     "same parameter name - more specific wins with invalid value",
			rules:    "/api/*?page=[1-100];/api/users?page=[1-10]",
			url:      "/api/users?page=50",
			expected: "/api/users",
		},
		{
			name:     "normalize with multiple rules",
			rules:    "/products?page=[1-10];/users?sort=[name,date]",
			url:      "/products?page=5&invalid=value",
			expected: "/products?page=5",
		},
		{
			name:     "normalize with second rule",
			rules:    "/products?page=[1-10];/users?sort=[name,date]",
			url:      "/users?sort=name&invalid=value",
			expected: "/users?sort=name",
		},
		{
			name:     "normalize with global and URL rules",
			rules:    "page=[1-10];/users?sort=[name,date]",
			url:      "/any/path?page=5&invalid=value",
			expected: "/any/path?page=5",
		},
		{
			name:     "multiple rules with different parameters",
			rules:    "/api/*?page=[1-10];/api/users?limit=[5,10]",
			url:      "/api/users?page=5&limit=10&invalid=value",
			expected: "/api/users?page=5&limit=10",
		},
		{
			name:     "conflicting parameter names",
			rules:    "/api/*?sort=[name,date];/api/users?sort=[name]",
			url:      "/api/users?sort=name",
			expected: "/api/users?sort=name",
		},
		{
			name:     "conflicting parameter names with invalid value",
			rules:    "/api/*?sort=[name,date];/api/users?sort=[name]",
			url:      "/api/users?sort=date",
			expected: "/api/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv := NewParamValidator(tt.rules)
			result := pv.NormalizeURL(tt.url)

			expectedURL, err := url.Parse(tt.expected)
			if err != nil {
				t.Fatalf("Invalid expected URL: %v", err)
			}
			expected := expectedURL.Path
			if expectedURL.RawQuery != "" {
				expected += "?" + expectedURL.RawQuery
			}

			if result != expected {
				t.Errorf("NormalizeURL(%q) with rules %q = %q, expected %q",
					tt.url, tt.rules, result, expected)
			}
		})
	}
}

func TestMultipleRulesPriority(t *testing.T) {
	tests := []struct {
		name     string
		rules    string
		url      string
		expected bool
	}{
		{
			name:     "more specific path wins - invalid case",
			rules:    "/api/*?page=[1-100];/api/users?page=[1-10]",
			url:      "/api/users?page=50",
			expected: false,
		},
		{
			name:     "more specific path wins",
			rules:    "/api/*?page=[1-100];/api/users?page=[1-10]",
			url:      "/api/users?page=5",
			expected: true,
		},
		{
			name:     "more specific path wins - invalid case",
			rules:    "/api/*?page=[1-100];/api/users?page=[1-10]",
			url:      "/api/users?page=50",
			expected: false,
		},
		{
			name:     "global rules have lowest priority",
			rules:    "page=[1-100];/api/users?page=[1-10];/api/users/list?page=[1-5]",
			url:      "/api/users/list?page=3",
			expected: true,
		},
		{
			name:     "exact match preferred over wildcard",
			rules:    "/api/*?sort=[name,date];/api/users?sort=[name]",
			url:      "/api/users?sort=name",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv := NewParamValidator(tt.rules)
			result := pv.ValidateURL(tt.url)
			if result != tt.expected {
				t.Errorf("ValidateURL(%q) with rules %q = %v, expected %v",
					tt.url, tt.rules, result, tt.expected)
			}
		})
	}
}

func BenchmarkValidateQueryParams(b *testing.B) {
	pv := NewParamValidator("/api/*?page=[1-100]&limit=[10,20,50]")
	urlPath := "/api/v1/data"
	query := "page=50&limit=20&invalid=value&extra=param"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateQueryParams(urlPath, query)
	}
}

func BenchmarkValidateQueryParamsParallel(b *testing.B) {
	pv := NewParamValidator("/api/*?page=[1-100]&limit=[10,20,50]")
	urlPath := "/api/v1/data"
	query := "page=50&limit=20"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pv.ValidateQueryParams(urlPath, query)
		}
	})
}

func TestValidateQueryParams(t *testing.T) {
	pv := NewParamValidator("/api/*?page=[1-100]&limit=[10,20,50]")

	tests := []struct {
		name     string
		urlPath  string
		query    string
		expected bool
	}{
		{
			name:     "valid parameters",
			urlPath:  "/api/v1/data",
			query:    "page=50&limit=20",
			expected: true,
		},
		{
			name:     "invalid parameter value",
			urlPath:  "/api/v1/data",
			query:    "page=150&limit=20",
			expected: false,
		},
		{
			name:     "unknown parameter",
			urlPath:  "/api/v1/data",
			query:    "page=50&unknown=value",
			expected: false,
		},
		{
			name:     "empty query string",
			urlPath:  "/api/v1/data",
			query:    "",
			expected: true,
		},
		{
			name:     "multiple invalid parameters",
			urlPath:  "/api/v1/data",
			query:    "page=150&limit=100&invalid=value",
			expected: false,
		},
		{
			name:     "wrong path - no rules apply",
			urlPath:  "/users",
			query:    "page=50",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pv.ValidateQueryParams(tt.urlPath, tt.query)
			if result != tt.expected {
				t.Errorf("ValidateQueryParams(%q, %q) = %v, expected %v",
					tt.urlPath, tt.query, result, tt.expected)
			}
		})
	}
}
