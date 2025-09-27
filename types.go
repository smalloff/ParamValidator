package paramvalidator

import (
	"sync"
	"sync/atomic"
)

// CallbackFunc defines function type for custom validation
type CallbackFunc func(paramName string, paramValue string) bool

// RuleType represents type of validation rules
type RuleType int
type Option func(*ParamValidator)

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
	MaxParamsCount     = 128
)

// ParamRule defines validation rule for single parameter
type ParamRule struct {
	Name            string
	Pattern         string
	Min             int64
	Max             int64
	Values          []string
	CustomValidator func(string) bool
	BitmaskIndex    int
}

// URLRule defines validation rules for specific URL pattern
type URLRule struct {
	URLPattern string
	Params     map[string]*ParamRule
	ParamMask  ParamMask
}

type ParamMask struct {
	parts [4]uint32
}

type RuleSource int

const (
	SourceNone RuleSource = iota
	SourceGlobal
	SourceURL
	SourceSpecificURL
)

type ParamMasks struct {
	Global      ParamMask // Глобальные параметры (низший приоритет)
	URL         ParamMask // Обычные URL правила (средний приоритет)
	SpecificURL ParamMask // Специфичные URL правила (высший приоритет)
}

// CompiledRules contains pre-compiled rules for faster access
type CompiledRules struct {
	globalParams map[string]*ParamRule
	urlRules     map[string]*URLRule
	paramIndex   *ParamIndex
}

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
	initialized   bool
	mu            sync.RWMutex
	parser        *RuleParser
	paramIndex    *ParamIndex
}

type wildcardPatternStats struct {
	count              int
	hasWildcard        bool
	lastCharIsWildcard bool
	hasMiddleWildcard  bool
	slashCount         int
}
