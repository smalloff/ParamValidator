package paramvalidator

import "sync"

// CallbackFunc defines function type for custom validation
type CallbackFunc func(paramName string, paramValue string) bool

// RuleType represents type of validation rules
type RuleType int

const (
	RuleTypeGlobal RuleType = iota
	RuleTypeURL
)

// Pattern constants for validation
const (
	PatternAll      = "*"
	PatternAny      = "any"
	PatternKeyOnly  = "key-only"
	PatternRange    = "range"
	PatternEnum     = "enum"
	PatternCallback = "callback"
)

// Validation limits
const (
	MaxURLLength       = 2048
	MaxParamNameLength = 100
	MaxParamValues     = 100
	MaxRulesSize       = 100 * 1024
	MaxPatternLength   = 200
)

// ParamRule defines validation rule for single parameter
type ParamRule struct {
	Name    string
	Pattern string
	Min     int64
	Max     int64
	Values  []string
}

// URLRule defines validation rules for specific URL pattern
type URLRule struct {
	URLPattern string
	Params     map[string]*ParamRule
}

// CompiledRules contains pre-compiled rules for faster access
type CompiledRules struct {
	globalParams map[string]*ParamRule
	urlRules     map[string]*URLRule
}

// wildcardPatternStats contains analysis of wildcard patterns
type wildcardPatternStats struct {
	count              int
	hasWildcard        bool
	lastCharIsWildcard bool
	hasMiddleWildcard  bool
	slashCount         int
}

// ParamValidator main struct for parameter validation
type ParamValidator struct {
	globalParams  map[string]*ParamRule
	urlRules      map[string]*URLRule
	compiledRules *CompiledRules
	callbackFunc  CallbackFunc
	initialized   bool
	mu            sync.RWMutex
	parser        *RuleParser
}
