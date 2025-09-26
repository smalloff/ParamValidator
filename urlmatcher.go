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
	maxSpecificity := -1
	urlPath = NormalizeURLPattern(urlPath)

	for pattern, rule := range um.urlRules {
		if um.urlMatchesPattern(urlPath, pattern) {
			specificity := um.calculateSpecificity(pattern)
			if specificity > maxSpecificity {
				maxSpecificity = specificity
				mostSpecificRule = rule
			}
		}
	}

	return mostSpecificRule
}

// urlMatchesPattern checks if URL path matches pattern
func (um *URLMatcher) urlMatchesPattern(urlPath, pattern string) bool {
	urlPath = NormalizeURLPattern(urlPath)

	switch {
	case pattern == PatternAll || pattern == urlPath:
		return true
	case strings.HasSuffix(pattern, PatternAll):
		return um.matchPrefixPattern(urlPath, pattern)
	case strings.Contains(pattern, "*"):
		return um.wildcardMatch(urlPath, pattern)
	default:
		return pattern == urlPath
	}
}

// matchPrefixPattern matches URL against prefix pattern ending with wildcard
func (um *URLMatcher) matchPrefixPattern(urlPath, pattern string) bool {
	prefix := strings.TrimSuffix(pattern, PatternAll)
	if prefix == "" {
		return true
	}
	prefix = strings.TrimSuffix(prefix, "/")
	return strings.HasPrefix(urlPath, prefix)
}

// wildcardMatch performs efficient wildcard matching without allocations
func (um *URLMatcher) wildcardMatch(urlPath, pattern string) bool {
	urlStart, patternStart := 0, 0
	urlLen, patternLen := len(urlPath), len(pattern)

	for urlStart < urlLen && patternStart < patternLen {
		urlEnd, patternEnd := um.findSegmentEnds(urlPath, pattern, urlStart, patternStart)

		if !um.compareSegments(urlPath[urlStart:urlEnd], pattern[patternStart:patternEnd]) {
			return false
		}

		urlStart, patternStart = um.nextSegmentStart(urlPath, pattern, urlEnd, patternEnd)
	}

	return urlStart >= urlLen && patternStart >= patternLen
}

// findSegmentEnds finds the end positions of current segments
func (um *URLMatcher) findSegmentEnds(urlPath, pattern string, urlStart, patternStart int) (int, int) {
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

// compareSegments compares URL and pattern segments
func (um *URLMatcher) compareSegments(urlSeg, patternSeg string) bool {
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

// nextSegmentStart advances to the next segment start position
func (um *URLMatcher) nextSegmentStart(urlPath, pattern string, urlEnd, patternEnd int) (int, int) {
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

// calculateSpecificity calculates specificity score for URL pattern
func (um *URLMatcher) calculateSpecificity(pattern string) int {
	if pattern == PatternAll {
		return 0
	}

	wildcardStats := um.analyzeWildcardPattern(pattern)
	pathSegmentCount := um.countPathSegments(pattern)

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

// analyzeWildcardPattern analyzes wildcard pattern characteristics
func (um *URLMatcher) analyzeWildcardPattern(pattern string) wildcardPatternStats {
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

// countPathSegments counts segments in URL path
func (um *URLMatcher) countPathSegments(pattern string) int {
	slashCount := strings.Count(pattern, "/")
	if len(pattern) > 0 && pattern[0] != '/' {
		return slashCount + 1
	}
	return slashCount
}
