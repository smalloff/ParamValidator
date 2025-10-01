// validation_cache.go
package paramvalidator

import (
	"sync"
)

// cacheKey represents a unique key for validation function cache
type cacheKey struct {
	pluginName string
	paramName  string
	constraint string
}

// ValidationCache caches plugin validation functions to avoid regeneration
type ValidationCache struct {
	cache sync.Map // cacheKey -> func(string) bool
}

// NewValidationCache creates a new validation cache instance
func NewValidationCache() *ValidationCache {
	return &ValidationCache{}
}

// Get returns cached validation function if exists
func (vc *ValidationCache) Get(pluginName, paramName, constraint string) (func(string) bool, bool) {
	key := cacheKey{
		pluginName: pluginName,
		paramName:  paramName,
		constraint: constraint,
	}

	if validator, ok := vc.cache.Load(key); ok {
		return validator.(func(string) bool), true
	}
	return nil, false
}

// Put stores validation function in cache
func (vc *ValidationCache) Put(pluginName, paramName, constraint string, validator func(string) bool) {
	key := cacheKey{
		pluginName: pluginName,
		paramName:  paramName,
		constraint: constraint,
	}
	vc.cache.Store(key, validator)
}

// Clear removes all cached validation functions
func (vc *ValidationCache) Clear() {
	vc.cache = sync.Map{}
}

// Size returns approximate number of cached functions
func (vc *ValidationCache) Size() int {
	count := 0
	vc.cache.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}
