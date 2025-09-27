// urlmatcher.go
package paramvalidator

import (
	"strings"
	"sync"
)

// URLMatcher handles URL pattern matching and specificity calculation
type URLMatcher struct {
	urlRules map[string]*URLRule
	mu       sync.RWMutex
}

// NewURLMatcher creates a new URL matcher instance
func NewURLMatcher() *URLMatcher {
	return &URLMatcher{
		urlRules: make(map[string]*URLRule),
	}
}

// AddRule adds a URL rule to the matcher
func (um *URLMatcher) AddRule(pattern string, rule *URLRule) {
	um.mu.Lock()
	defer um.mu.Unlock()

	// Calculate specificity once and store it
	if rule != nil {
		rule.specificity = int16(calculateSpecificity(pattern))
	}
	um.urlRules[pattern] = rule
}

// RemoveRule removes a URL rule from the matcher
func (um *URLMatcher) RemoveRule(pattern string) {
	um.mu.Lock()
	defer um.mu.Unlock()
	delete(um.urlRules, pattern)
}

// ClearRules removes all URL rules
func (um *URLMatcher) ClearRules() {
	um.mu.Lock()
	defer um.mu.Unlock()
	um.urlRules = make(map[string]*URLRule)
}

// GetMatchingRules finds all rules that match the given URL path
func (um *URLMatcher) GetMatchingRules(urlPath string) []*URLRule {
	um.mu.RLock()
	defer um.mu.RUnlock()

	var matchingRules []*URLRule
	urlPath = NormalizeURLPattern(urlPath)

	for pattern, rule := range um.urlRules {
		if um.urlMatchesPattern(urlPath, pattern) {
			matchingRules = append(matchingRules, rule)
		}
	}

	return matchingRules
}

// GetMostSpecificRule finds the most specific matching rule for the URL path
func (um *URLMatcher) GetMostSpecificRule(urlPath string) *URLRule {
	um.mu.RLock()
	defer um.mu.RUnlock()

	var mostSpecificRule *URLRule
	maxSpecificity := int16(-1)
	urlPath = NormalizeURLPattern(urlPath)

	for pattern, rule := range um.urlRules {
		if rule != nil && um.urlMatchesPattern(urlPath, pattern) {
			// Use pre-calculated specificity
			if rule.specificity > maxSpecificity {
				maxSpecificity = rule.specificity
				mostSpecificRule = rule
			}
		}
	}

	return mostSpecificRule
}

// URL matching internals - these remain private
func (um *URLMatcher) urlMatchesPattern(urlPath, pattern string) bool {
	return urlMatchesPattern(urlPath, pattern)
}

// Export URL matching functions for use by ParamValidator
func urlMatchesPattern(urlPath, pattern string) bool {
	urlPath = NormalizeURLPattern(urlPath)

	switch {
	case pattern == PatternAll || pattern == urlPath:
		return true
	case strings.HasSuffix(pattern, PatternAll):
		return matchPrefixPattern(urlPath, pattern)
	case strings.Contains(pattern, "*"):
		return wildcardMatch(urlPath, pattern)
	default:
		return pattern == urlPath
	}
}

func matchPrefixPattern(urlPath, pattern string) bool {
	prefix := strings.TrimSuffix(pattern, PatternAll)
	if prefix == "" {
		return true
	}
	prefix = strings.TrimSuffix(prefix, "/")
	return strings.HasPrefix(urlPath, prefix)
}

func wildcardMatch(urlPath, pattern string) bool {
	urlStart, patternStart := 0, 0
	urlLen, patternLen := len(urlPath), len(pattern)

	for urlStart < urlLen && patternStart < patternLen {
		urlEnd, patternEnd := findSegmentEnds(urlPath, pattern, urlStart, patternStart)

		if !compareSegments(urlPath[urlStart:urlEnd], pattern[patternStart:patternEnd]) {
			return false
		}

		urlStart, patternStart = nextSegmentStart(urlPath, pattern, urlEnd, patternEnd)
	}

	return urlStart >= urlLen && patternStart >= patternLen
}

func findSegmentEnds(urlPath, pattern string, urlStart, patternStart int) (int, int) {
	urlEnd := urlStart
	for urlEnd < len(urlPath) && urlPath[urlEnd] != '/' {
		urlEnd++
	}

	patternEnd := patternStart
	for patternEnd < len(pattern) && pattern[patternEnd] != '/' {
		patternEnd++
	}

	return urlEnd, patternEnd
}

func compareSegments(urlSeg, patternSeg string) bool {
	if len(patternSeg) == 1 && patternSeg[0] == '*' {
		return true // Wildcard matches any segment
	}

	if len(urlSeg) != len(patternSeg) {
		return false
	}

	// Fast comparison for short segments
	if len(urlSeg) <= 8 {
		for i := 0; i < len(urlSeg); i++ {
			if urlSeg[i] != patternSeg[i] {
				return false
			}
		}
		return true
	}

	return urlSeg == patternSeg
}

func nextSegmentStart(urlPath, pattern string, urlEnd, patternEnd int) (int, int) {
	urlStart := urlEnd
	if urlStart < len(urlPath) && urlPath[urlStart] == '/' {
		urlStart++
	}

	patternStart := patternEnd
	if patternStart < len(pattern) && pattern[patternStart] == '/' {
		patternStart++
	}

	return urlStart, patternStart
}

func calculateSpecificity(pattern string) int {
	if pattern == PatternAll {
		return 0
	}

	wildcardStats := analyzeWildcardPattern(pattern)
	pathSegmentCount := countPathSegments(pattern)

	specificity := pathSegmentCount * 100

	if !wildcardStats.hasWildcard {
		specificity += 1500
	} else {
		specificity -= wildcardStats.count * 200

		if wildcardStats.lastCharIsWildcard {
			specificity -= 300
		}
		if wildcardStats.hasMiddleWildcard {
			specificity -= 100
		}
	}

	if wildcardStats.slashCount > 1 {
		specificity += wildcardStats.slashCount * 50
	}

	return specificity
}

func analyzeWildcardPattern(pattern string) wildcardPatternStats {
	var stats wildcardPatternStats

	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			stats.count++
			stats.hasWildcard = true
			if i == len(pattern)-1 {
				stats.lastCharIsWildcard = true
			} else if i > 0 {
				stats.hasMiddleWildcard = true
			}
		case '/':
			stats.slashCount++
		}
	}

	return stats
}

func countPathSegments(pattern string) int {
	slashCount := strings.Count(pattern, "/")
	if len(pattern) > 0 && pattern[0] != '/' {
		return slashCount + 1
	}
	return slashCount
}
