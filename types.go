package paramvalidator

import (
	"sync"
	"sync/atomic"
)

// CallbackFunc defines function type for custom validation
type CallbackFunc func(paramName string, paramValue string) bool

// RuleType represents type of validation rules
type RuleType int

const (
	RuleTypeGlobal RuleType = iota
	RuleTypeURL
)

// Option defines configuration function type for ParamValidator
type Option func(*ParamValidator)

// Pattern constants for validation
const (
	PatternAll      = "*"
	PatternAny      = "any"
	PatternKeyOnly  = "key-only"
	PatternRange    = "range"
	PatternEnum     = "enum"
	PatternCallback = "callback"
	PatternInverted = "inverted"
)

// Validation limits
const (
	MaxURLLength       = 4096
	MaxParamNameLength = 256
	MaxParamValues     = 500
	MaxRulesSize       = 10000
	MaxPatternLength   = 1024
	MaxParamsCount     = 128
)

// RuleSource represents the source of parameter rule
type RuleSource int

const (
	SourceNone RuleSource = iota
	SourceGlobal
	SourceURL
	SourceSpecificURL
)

// ParamRule defines validation rule for single parameter
type ParamRule struct {
	Name            string
	Pattern         string
	Values          []string
	CustomValidator func(string) bool
	BitmaskIndex    int
	Inverted        bool
	ConstraintStr   string
}

// URLRule defines validation rules for specific URL pattern
type URLRule struct {
	URLPattern    string
	Params        map[string]*ParamRule
	ParamMask     ParamMask
	specificity   int16
	paramsByIndex map[int]*ParamRule
}

// ParamMask represents a bitmask for parameter indexing
type ParamMask struct {
	parts [4]uint32
}

// ParamMasks contains masks for different rule sources with priorities
type ParamMasks struct {
	Global      ParamMask // Global parameters (lowest priority)
	URL         ParamMask // Regular URL rules (medium priority)
	SpecificURL ParamMask // Specific URL rules (highest priority)
}

// CompiledRules contains pre-compiled rules for faster access
type CompiledRules struct {
	globalParams        map[string]*ParamRule
	urlRules            map[string]*URLRule
	paramIndex          *ParamIndex
	globalParamsMask    ParamMask
	globalParamsByIndex map[int]*ParamRule
	urlRulesByIndex     map[int][]*URLRule
}

// ParamIndex provides lock-free parameter indexing
type ParamIndex struct {
	paramToIndex sync.Map // string -> int (lock-free)
	nextIndex    atomic.Int32
	maxIndex     int32
}

// ParamValidator main struct for parameter validation
type ParamValidator struct {
	globalParams  map[string]*ParamRule
	urlRules      map[string]*URLRule
	urlMatcher    *URLMatcher
	compiledRules *CompiledRules
	callbackFunc  CallbackFunc
	initialized   atomic.Bool
	mu            sync.RWMutex
	parser        *RuleParser
	paramIndex    *ParamIndex
}

// wildcardPatternStats contains statistics for URL pattern matching optimization
type wildcardPatternStats struct {
	count              int
	hasWildcard        bool
	lastCharIsWildcard bool
	hasMiddleWildcard  bool
	slashCount         int
}
