package paramvalidator

import (
	"reflect"
	"testing"
)

func TestValidationCacheBasic(t *testing.T) {
	cache := NewValidationCache()

	if size := cache.Size(); size != 0 {
		t.Errorf("Expected empty cache, got size %d", size)
	}

	validator := func(string) bool { return true }

	cache.Put("testPlugin", "param1", "constraint1", validator)
	if size := cache.Size(); size != 1 {
		t.Errorf("Expected cache size 1, got %d", size)
	}

	if cachedValidator, found := cache.Get("testPlugin", "param1", "constraint1"); !found {
		t.Error("Expected to find validator in cache")
	} else if cachedValidator == nil {
		t.Error("Retrieved validator should not be nil")
	} else if !cachedValidator("test") {
		t.Error("Cached validator should work correctly")
	}

	if _, found := cache.Get("unknown", "param", "constraint"); found {
		t.Error("Should not find non-existent validator")
	}
}

func TestValidationCacheClear(t *testing.T) {
	cache := NewValidationCache()

	cache.Put("plugin1", "param1", "constraint1", func(string) bool { return true })
	cache.Put("plugin2", "param2", "constraint2", func(string) bool { return false })

	if size := cache.Size(); size != 2 {
		t.Errorf("Expected cache size 2, got %d", size)
	}

	cache.Clear()

	if size := cache.Size(); size != 0 {
		t.Errorf("Expected empty cache after clear, got size %d", size)
	}

	if _, found := cache.Get("plugin1", "param1", "constraint1"); found {
		t.Error("Validator should be removed after clear")
	}
}

func TestValidationCacheKeyUniqueness(t *testing.T) {
	cache := NewValidationCache()

	validator1 := func(string) bool { return true }
	validator2 := func(string) bool { return false }

	cache.Put("plugin1", "param", "constraint", validator1)
	cache.Put("plugin2", "param", "constraint", validator2)

	if size := cache.Size(); size != 2 {
		t.Errorf("Expected 2 validators for different plugins, got %d", size)
	}

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

func TestValidationCacheConcurrent(t *testing.T) {
	cache := NewValidationCache()
	iterations := 1000

	done := make(chan bool)
	for i := 0; i < iterations; i++ {
		go func(n int) {
			cache.Put("plugin", "param", string(rune(n)), func(string) bool { return true })
			done <- true
		}(i)
	}

	for i := 0; i < iterations; i++ {
		<-done
	}

	for i := 0; i < iterations; i++ {
		if _, found := cache.Get("plugin", "param", string(rune(i))); !found {
			t.Errorf("Validator %d should be accessible", i)
		}
	}

	if size := cache.Size(); size != iterations {
		t.Errorf("Expected %d validators, got %d", iterations, size)
	}
}

func TestValidationCacheFunctionBehavior(t *testing.T) {
	cache := NewValidationCache()

	callCount := 0
	originalValidator := func(s string) bool {
		callCount++
		return s == "test"
	}

	cache.Put("testPlugin", "testParam", "testConstraint", originalValidator)

	validator1, found1 := cache.Get("testPlugin", "testParam", "testConstraint")
	validator2, found2 := cache.Get("testPlugin", "testParam", "testConstraint")

	if !found1 || !found2 {
		t.Error("Should find validator both times")
	}

	if validator1 == nil || validator2 == nil {
		t.Error("Validators should not be nil")
	}

	result1 := validator1("test")
	result2 := validator2("test")

	if result1 != result2 {
		t.Error("Cached validators should have identical behavior")
	}

	if !result1 {
		t.Error("Validator should return true for 'test'")
	}

	result3 := validator1("wrong")
	result4 := validator2("wrong")

	if result3 != result4 {
		t.Error("Cached validators should have identical behavior for different inputs")
	}

	if result3 {
		t.Error("Validator should return false for 'wrong'")
	}
}

func TestValidationCacheFunctionPointer(t *testing.T) {
	cache := NewValidationCache()

	validator := func(s string) bool { return len(s) > 0 }

	cache.Put("plugin", "param", "constraint", validator)
	retrieved, found := cache.Get("plugin", "param", "constraint")

	if !found {
		t.Error("Should find stored validator")
	}

	val1 := reflect.ValueOf(validator)
	val2 := reflect.ValueOf(retrieved)

	if val1.Kind() != reflect.Func || val2.Kind() != reflect.Func {
		t.Error("Both should be functions")
	}

	testCases := []string{"", "test", "hello"}
	for _, tc := range testCases {
		res1 := validator(tc)
		res2 := retrieved(tc)
		if res1 != res2 {
			t.Errorf("Different results for input '%s': %v vs %v", tc, res1, res2)
		}
	}
}

func BenchmarkValidationCache(b *testing.B) {
	cache := NewValidationCache()

	for i := 0; i < 100; i++ {
		cache.Put("plugin", "param", string(rune(i)), func(string) bool { return true })
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune(i % 150))
			cache.Get("plugin", "param", key)
			i++
		}
	})
}

func BenchmarkValidationCachePut(b *testing.B) {
	cache := NewValidationCache()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put("plugin", "param", string(rune(i)), func(string) bool { return true })
	}
}
