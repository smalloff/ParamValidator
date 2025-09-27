// validation_cache_test.go
package paramvalidator

import (
	"reflect"
	"testing"
)

// TestValidationCacheBasic tests basic cache operations
func TestValidationCacheBasic(t *testing.T) {
	cache := NewValidationCache()

	// Test empty cache
	if size := cache.Size(); size != 0 {
		t.Errorf("Expected empty cache, got size %d", size)
	}

	validator := func(string) bool { return true }

	// Test Put and Get
	cache.Put("testPlugin", "param1", "constraint1", validator)
	if size := cache.Size(); size != 1 {
		t.Errorf("Expected cache size 1, got %d", size)
	}

	// Test retrieval
	if cachedValidator, found := cache.Get("testPlugin", "param1", "constraint1"); !found {
		t.Error("Expected to find validator in cache")
	} else if cachedValidator == nil {
		t.Error("Retrieved validator should not be nil")
	} else if !cachedValidator("test") {
		t.Error("Cached validator should work correctly")
	}

	// Test non-existent key
	if _, found := cache.Get("unknown", "param", "constraint"); found {
		t.Error("Should not find non-existent validator")
	}
}

// TestValidationCacheClear tests cache clearance
func TestValidationCacheClear(t *testing.T) {
	cache := NewValidationCache()

	// Add some validators
	cache.Put("plugin1", "param1", "constraint1", func(string) bool { return true })
	cache.Put("plugin2", "param2", "constraint2", func(string) bool { return false })

	if size := cache.Size(); size != 2 {
		t.Errorf("Expected cache size 2, got %d", size)
	}

	// Clear cache
	cache.Clear()

	if size := cache.Size(); size != 0 {
		t.Errorf("Expected empty cache after clear, got size %d", size)
	}

	// Verify validators are gone
	if _, found := cache.Get("plugin1", "param1", "constraint1"); found {
		t.Error("Validator should be removed after clear")
	}
}

// TestValidationCacheKeyUniqueness tests cache key uniqueness
func TestValidationCacheKeyUniqueness(t *testing.T) {
	cache := NewValidationCache()

	validator1 := func(string) bool { return true }
	validator2 := func(string) bool { return false }

	// Different plugins, same param/constraint
	cache.Put("plugin1", "param", "constraint", validator1)
	cache.Put("plugin2", "param", "constraint", validator2)

	if size := cache.Size(); size != 2 {
		t.Errorf("Expected 2 validators for different plugins, got %d", size)
	}

	// Retrieve and verify uniqueness
	if v1, found := cache.Get("plugin1", "param", "constraint"); !found || v1 == nil {
		t.Error("Should find validator for plugin1")
	} else if !v1("test") {
		t.Error("Plugin1 validator should return true")
	}

	if v2, found := cache.Get("plugin2", "param", "constraint"); !found || v2 == nil {
		t.Error("Should find validator for plugin2")
	} else if v2("test") {
		t.Error("Plugin2 validator should return false")
	}
}

// TestValidationCacheStats tests cache statistics
func TestValidationCacheStats(t *testing.T) {
	cache := NewValidationCache()

	// Add validators from different plugins
	cache.Put("rangePlugin", "age", "1-100", func(string) bool { return true })
	cache.Put("rangePlugin", "score", "0-1000", func(string) bool { return true })
	cache.Put("regexPlugin", "email", ".*@.*", func(string) bool { return true })
	cache.Put("customPlugin", "token", "secret", func(string) bool { return true })

	stats := cache.GetStats()

	if stats["size"] != 4 {
		t.Errorf("Expected stats size 4, got %v", stats["size"])
	}

	pluginStats, ok := stats["plugins"].(map[string]int)
	if !ok {
		t.Fatal("Plugin stats should be a map")
	}

	if pluginStats["rangePlugin"] != 2 {
		t.Errorf("Expected 2 validators for rangePlugin, got %d", pluginStats["rangePlugin"])
	}
	if pluginStats["regexPlugin"] != 1 {
		t.Errorf("Expected 1 validator for regexPlugin, got %d", pluginStats["regexPlugin"])
	}
	if pluginStats["customPlugin"] != 1 {
		t.Errorf("Expected 1 validator for customPlugin, got %d", pluginStats["customPlugin"])
	}
}

// TestValidationCacheConcurrent tests concurrent cache access
func TestValidationCacheConcurrent(t *testing.T) {
	cache := NewValidationCache()
	iterations := 1000

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < iterations; i++ {
		go func(n int) {
			cache.Put("plugin", "param", string(rune(n)), func(string) bool { return true })
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < iterations; i++ {
		<-done
	}

	// Verify all entries are accessible
	for i := 0; i < iterations; i++ {
		if _, found := cache.Get("plugin", "param", string(rune(i))); !found {
			t.Errorf("Validator %d should be accessible", i)
		}
	}

	if size := cache.Size(); size != iterations {
		t.Errorf("Expected %d validators, got %d", iterations, size)
	}
}

// TestValidationCacheFunctionBehavior tests that cached functions maintain correct behavior
func TestValidationCacheFunctionBehavior(t *testing.T) {
	cache := NewValidationCache()

	// Create a validator with specific behavior
	callCount := 0
	originalValidator := func(s string) bool {
		callCount++
		return s == "test"
	}

	// Store validator
	cache.Put("testPlugin", "testParam", "testConstraint", originalValidator)

	// Retrieve multiple times
	validator1, found1 := cache.Get("testPlugin", "testParam", "testConstraint")
	validator2, found2 := cache.Get("testPlugin", "testParam", "testConstraint")

	if !found1 || !found2 {
		t.Error("Should find validator both times")
	}

	if validator1 == nil || validator2 == nil {
		t.Error("Validators should not be nil")
	}

	// Test that both validators have the same behavior
	result1 := validator1("test")
	result2 := validator2("test")

	if result1 != result2 {
		t.Error("Cached validators should have identical behavior")
	}

	if !result1 {
		t.Error("Validator should return true for 'test'")
	}

	// Test with different input
	result3 := validator1("wrong")
	result4 := validator2("wrong")

	if result3 != result4 {
		t.Error("Cached validators should have identical behavior for different inputs")
	}

	if result3 {
		t.Error("Validator should return false for 'wrong'")
	}
}

// TestValidationCacheFunctionPointer tests function pointer consistency using reflect
func TestValidationCacheFunctionPointer(t *testing.T) {
	cache := NewValidationCache()

	validator := func(s string) bool { return len(s) > 0 }

	// Store and retrieve
	cache.Put("plugin", "param", "constraint", validator)
	retrieved, found := cache.Get("plugin", "param", "constraint")

	if !found {
		t.Error("Should find stored validator")
	}

	// Use reflect to get function pointers (for testing purposes only)
	val1 := reflect.ValueOf(validator)
	val2 := reflect.ValueOf(retrieved)

	// Both should be functions
	if val1.Kind() != reflect.Func || val2.Kind() != reflect.Func {
		t.Error("Both should be functions")
	}

	// Test they behave the same
	testCases := []string{"", "test", "hello"}
	for _, tc := range testCases {
		res1 := validator(tc)
		res2 := retrieved(tc)
		if res1 != res2 {
			t.Errorf("Different results for input '%s': %v vs %v", tc, res1, res2)
		}
	}
}

// BenchmarkValidationCache measures cache performance
func BenchmarkValidationCache(b *testing.B) {
	cache := NewValidationCache()

	// Pre-populate cache
	for i := 0; i < 100; i++ {
		cache.Put("plugin", "param", string(rune(i)), func(string) bool { return true })
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Mix of hits and misses
			key := string(rune(i % 150)) // 100 hits, 50 misses
			cache.Get("plugin", "param", key)
			i++
		}
	})
}

// BenchmarkValidationCachePut measures cache write performance
func BenchmarkValidationCachePut(b *testing.B) {
	cache := NewValidationCache()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put("plugin", "param", string(rune(i)), func(string) bool { return true })
	}
}
