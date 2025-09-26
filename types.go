package paramvalidator

import "sync"

const (
	PatternRange    = "range"
	PatternEnum     = "enum"
	PatternKeyOnly  = "key-only"
	PatternAny      = "any"
	PatternAll      = "*"
	PatternCallback = "callback"

	MaxRulesSize       = 64 * 1024
	MaxURLLength       = 4096
	MaxPatternLength   = 200
	MaxParamNameLength = 100
	MaxParamValues     = 100
)

// QueryParam represents a single query parameter key-value pair
type QueryParam struct {
	Key   string
	Value string
}

// CallbackFunc defines the signature for custom validation callback
type CallbackFunc func(key string, value string) bool

// ParamRule defines validation rules for a specific parameter
type ParamRule struct {
	Values  []string
	Name    string
	Pattern string
	Min     int64
	Max     int64
}

// URLRule defines validation rules for a specific URL pattern
type URLRule struct {
	Params     map[string]*ParamRule
	URLPattern string
}

// CompiledRules contains optimized rule structures for fast validation
type CompiledRules struct {
	globalParams map[string]*ParamRule
	urlRules     map[string]*URLRule
}

// ParamValidator validates URL parameters against configured rules
type ParamValidator struct {
	mu            sync.RWMutex
	globalParams  map[string]*ParamRule
	urlRules      map[string]*URLRule
	rulesStr      string
	initialized   bool
	compiledRules *CompiledRules
	callbackFunc  CallbackFunc
}

type RuleType int

const (
	RuleTypeUnknown RuleType = iota
	RuleTypeGlobal
	RuleTypeURL
)

// wildcardPatternStats holds analysis results for wildcard patterns
type wildcardPatternStats struct {
	count              int
	slashCount         int
	hasWildcard        bool
	lastCharIsWildcard bool
	hasMiddleWildcard  bool
}
