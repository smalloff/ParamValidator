package paramvalidator

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ValidateURL validates complete URL against loaded rules
func (pv *ParamValidator) ValidateURL(fullURL string) bool {
	if pv == nil || !pv.initialized || fullURL == "" {
		return false
	}

	if err := pv.validateInputSize(fullURL, MaxURLLength); err != nil {
		return false
	}

	pv.mu.RLock()
	defer pv.mu.RUnlock()

	return pv.validateURLUnsafe(fullURL)
}

// validateURLUnsafe validates URL without locking
func (pv *ParamValidator) validateURLUnsafe(fullURL string) bool {
	u, err := url.Parse(fullURL)
	if err != nil {
		return false
	}

	paramsRules := pv.getParamsForURLUnsafe(u.Path)

	if pv.isAllowAllParams(paramsRules) {
		return true
	}

	if len(paramsRules) == 0 && u.RawQuery != "" {
		return false
	}

	if u.RawQuery == "" {
		return true
	}

	valid, err := pv.parseAndValidateQueryParams(u.RawQuery, paramsRules)
	return err == nil && valid
}

// isAllowAllParams checks if rules allow all parameters
func (pv *ParamValidator) isAllowAllParams(paramsRules map[string]*ParamRule) bool {
	return paramsRules != nil && paramsRules[PatternAll] != nil
}

// findParamRule finds matching rule for parameter name
func (pv *ParamValidator) findParamRule(paramName string, urlParams map[string]*ParamRule) *ParamRule {
	if rule, exists := urlParams[paramName]; exists {
		return rule
	}
	if rule, exists := pv.compiledRules.globalParams[paramName]; exists {
		return rule
	}
	return nil
}

// getParamsForURLUnsafe gets all applicable parameter rules for URL path
func (pv *ParamValidator) getParamsForURLUnsafe(urlPath string) map[string]*ParamRule {
	urlPath = pv.normalizeURLPattern(urlPath)
	mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath)

	result := make(map[string]*ParamRule)

	// Add global parameters
	for name, rule := range pv.compiledRules.globalParams {
		result[name] = rule
	}

	// Add URL-specific parameters from matching patterns
	for pattern, rule := range pv.compiledRules.urlRules {
		if pv.urlMatchesPatternUnsafe(urlPath, pattern) {
			for paramName, paramRule := range rule.Params {
				result[paramName] = paramRule
			}
		}
	}

	// Override with most specific rule parameters
	if mostSpecificRule != nil {
		for paramName, paramRule := range mostSpecificRule.Params {
			result[paramName] = paramRule
		}
	}

	return result
}

// findMostSpecificURLRuleUnsafe finds most specific matching URL rule
func (pv *ParamValidator) findMostSpecificURLRuleUnsafe(urlPath string) *URLRule {
	var mostSpecificRule *URLRule
	maxSpecificity := -1

	for pattern, rule := range pv.compiledRules.urlRules {
		if pv.urlMatchesPatternUnsafe(urlPath, pattern) {
			specificity := pv.calculateSpecificityUnsafe(pattern)
			if specificity > maxSpecificity {
				maxSpecificity = specificity
				mostSpecificRule = rule
			}
		}
	}

	return mostSpecificRule
}

// calculateSpecificityUnsafe calculates specificity score for URL pattern
func (pv *ParamValidator) calculateSpecificityUnsafe(pattern string) int {
	if pattern == PatternAll {
		return 0
	}

	wildcardStats := pv.analyzeWildcardPattern(pattern)
	pathSegmentCount := pv.countPathSegments(pattern)

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
func (pv *ParamValidator) analyzeWildcardPattern(pattern string) wildcardPatternStats {
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
func (pv *ParamValidator) countPathSegments(pattern string) int {
	slashCount := strings.Count(pattern, "/")
	if len(pattern) > 0 && pattern[0] != '/' {
		return slashCount + 1
	}
	return slashCount
}

// urlMatchesPatternUnsafe checks if URL path matches pattern
func (pv *ParamValidator) urlMatchesPatternUnsafe(urlPath, pattern string) bool {
	urlPath = pv.normalizeURLPattern(urlPath)

	switch {
	case pattern == PatternAll || pattern == urlPath:
		return true
	case strings.HasSuffix(pattern, PatternAll):
		return pv.matchPrefixPattern(urlPath, pattern)
	case strings.Contains(pattern, "*"):
		return pv.wildcardMatch(urlPath, pattern)
	default:
		return pattern == urlPath
	}
}

// matchPrefixPattern matches URL against prefix pattern ending with wildcard
func (pv *ParamValidator) matchPrefixPattern(urlPath, pattern string) bool {
	prefix := strings.TrimSuffix(pattern, PatternAll)
	if prefix == "" {
		return true
	}
	prefix = strings.TrimSuffix(prefix, "/")
	return strings.HasPrefix(urlPath, prefix)
}

// wildcardMatch performs efficient wildcard matching without allocations
func (pv *ParamValidator) wildcardMatch(urlPath, pattern string) bool {

	urlStart, patternStart := 0, 0
	urlLen, patternLen := len(urlPath), len(pattern)
	segmentCount := 0

	for urlStart < urlLen && patternStart < patternLen {
		urlEnd, patternEnd := pv.findSegmentEnds(urlPath, pattern, urlStart, patternStart)

		if !pv.compareSegments(urlPath[urlStart:urlEnd], pattern[patternStart:patternEnd]) {
			return false
		}

		urlStart, patternStart = pv.nextSegmentStart(urlPath, pattern, urlEnd, patternEnd)
		segmentCount++
	}

	return urlStart >= urlLen && patternStart >= patternLen
}

// findSegmentEnds finds the end positions of current segments
func (pv *ParamValidator) findSegmentEnds(urlPath, pattern string, urlStart, patternStart int) (int, int) {
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
func (pv *ParamValidator) compareSegments(urlSeg, patternSeg string) bool {
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
func (pv *ParamValidator) nextSegmentStart(urlPath, pattern string, urlEnd, patternEnd int) (int, int) {
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

// isValueValidUnsafe checks if value is valid according to rule
func (pv *ParamValidator) isValueValidUnsafe(rule *ParamRule, value string) bool {
	switch rule.Pattern {
	case PatternKeyOnly:
		return value == ""
	case PatternAny:
		return true
	case PatternRange:
		return pv.validateRangeValue(rule, value)
	case PatternEnum:
		return pv.validateEnumValue(rule, value)
	case PatternCallback:
		return pv.validateCallbackValue(rule, value)
	default:
		return false
	}
}

// validateRangeValue validates numeric range values
func (pv *ParamValidator) validateRangeValue(rule *ParamRule, value string) bool {
	num, err := strconv.ParseInt(value, 10, 64)
	return err == nil && num >= rule.Min && num <= rule.Max
}

// validateEnumValue validates enum values
func (pv *ParamValidator) validateEnumValue(rule *ParamRule, value string) bool {
	for _, allowedValue := range rule.Values {
		if value == allowedValue {
			return true
		}
	}
	return false
}

// validateCallbackValue validates values using callback function
func (pv *ParamValidator) validateCallbackValue(rule *ParamRule, value string) bool {
	if pv.callbackFunc != nil {
		return pv.callbackFunc(rule.Name, value)
	}
	return false
}

// ValidateParam validates single parameter value for specific URL path
func (pv *ParamValidator) ValidateParam(urlPath, paramName, paramValue string) bool {
	if !pv.initialized || urlPath == "" || paramName == "" {
		return false
	}

	if err := pv.validateInputSize(urlPath, MaxURLLength); err != nil {
		return false
	}

	pv.mu.RLock()
	defer pv.mu.RUnlock()

	return pv.validateParamUnsafe(urlPath, paramName, paramValue)
}

// validateParamUnsafe validates single parameter without locking
func (pv *ParamValidator) validateParamUnsafe(urlPath, paramName, paramValue string) bool {
	paramsRules := pv.getParamsForURLUnsafe(urlPath)
	rule := pv.findParamRule(paramName, paramsRules)

	return rule != nil && pv.isValueValidUnsafe(rule, paramValue)
}

// NormalizeURL filters and normalizes URL according to validation rules
func (pv *ParamValidator) NormalizeURL(fullURL string) string {
	if pv == nil || !pv.initialized || fullURL == "" {
		return fullURL
	}

	if err := pv.validateInputSize(fullURL, MaxURLLength); err != nil {
		return fullURL
	}

	pv.mu.RLock()
	defer pv.mu.RUnlock()

	return pv.normalizeURLUnsafe(fullURL)
}

// normalizeURLUnsafe normalizes URL without locking
func (pv *ParamValidator) normalizeURLUnsafe(fullURL string) string {
	u, err := url.Parse(fullURL)
	if err != nil {
		return fullURL
	}

	paramsRules := pv.getParamsForURLUnsafe(u.Path)

	if pv.isAllowAllParams(paramsRules) {
		return fullURL
	}

	if len(paramsRules) == 0 && len(pv.compiledRules.globalParams) == 0 {
		return u.Path
	}

	if u.RawQuery == "" {
		return u.Path
	}

	filteredParams, _, err := pv.parseAndFilterQueryParams(u.RawQuery, paramsRules)
	if err != nil {
		return u.Path
	}

	if filteredParams != "" {
		u.RawQuery = filteredParams
		return u.String()
	}

	return u.Path
}

// parseAndValidateQueryParams parses and validates query parameters
func (pv *ParamValidator) parseAndValidateQueryParams(queryString string, paramsRules map[string]*ParamRule) (bool, error) {
	if queryString == "" {
		return true, nil
	}

	isValid := true
	allowAll := pv.isAllowAllParams(paramsRules)

	err := pv.processQueryParams(queryString, func(key, value, originalKey, originalValue string) {
		if !allowAll && !pv.isParamAllowedUnsafe(key, value, paramsRules) {
			isValid = false
		}
	})

	return isValid, err
}

// parseAndFilterQueryParams parses and filters query parameters
func (pv *ParamValidator) parseAndFilterQueryParams(queryString string, paramsRules map[string]*ParamRule) (string, bool, error) {
	if queryString == "" {
		return "", false, nil
	}

	var filteredParams strings.Builder
	isValid := true
	allowAll := pv.isAllowAllParams(paramsRules)
	firstParam := true

	err := pv.processQueryParams(queryString, func(key, value, originalKey, originalValue string) {
		if allowAll || pv.isParamAllowedUnsafe(key, value, paramsRules) {
			if !firstParam {
				filteredParams.WriteString("&")
			} else {
				firstParam = false
			}

			if originalValue == "" {
				filteredParams.WriteString(originalKey)
			} else {
				filteredParams.WriteString(originalKey + "=" + originalValue)
			}
		} else if !allowAll {
			isValid = false
		}
	})

	return filteredParams.String(), isValid, err
}

// processQueryParams processes query parameters with a callback function
func (pv *ParamValidator) processQueryParams(queryString string, processor func(key, value, originalKey, originalValue string)) error {
	if queryString == "" {
		return nil
	}

	start := 0
	paramCount := 0

	for i := 0; i <= len(queryString); i++ {
		if i == len(queryString) || queryString[i] == '&' {
			if start < i && paramCount < MaxParamValues {
				segment := queryString[start:i]
				key, value, originalKey, originalValue, err := pv.parseParamSegment(segment)
				if err == nil {
					processor(key, value, originalKey, originalValue)
				}
				paramCount++
			}
			start = i + 1
		}
	}

	if paramCount > MaxParamValues {
		return fmt.Errorf("too many parameters")
	}

	return nil
}

// parseParamSegment parses a single parameter segment into key and value
func (pv *ParamValidator) parseParamSegment(segment string) (key, value, originalKey, originalValue string, err error) {
	eqPos := strings.IndexByte(segment, '=')

	if eqPos == -1 {
		decodedKey, err := url.QueryUnescape(segment)
		if err != nil {
			return "", "", "", "", err
		}
		return decodedKey, "", segment, "", nil
	}

	originalKey = segment[:eqPos]
	originalValue = segment[eqPos+1:]

	decodedKey, err1 := url.QueryUnescape(originalKey)
	decodedValue, err2 := url.QueryUnescape(originalValue)

	if err1 != nil || err2 != nil {
		return "", "", "", "", fmt.Errorf("decoding error")
	}

	return decodedKey, decodedValue, originalKey, originalValue, nil
}

// isParamAllowedUnsafe checks if parameter is allowed according to rules
func (pv *ParamValidator) isParamAllowedUnsafe(paramName, paramValue string, paramsRules map[string]*ParamRule) bool {
	rule := pv.findParamRule(paramName, paramsRules)
	return rule != nil && pv.isValueValidUnsafe(rule, paramValue)
}

// filterQueryParamsUnsafe filters query parameters without locking
func (pv *ParamValidator) filterQueryParamsUnsafe(urlPath, queryString string) string {
	paramsRules := pv.getParamsForURLUnsafe(urlPath)

	if pv.isAllowAllParams(paramsRules) {
		return queryString
	}

	if len(paramsRules) == 0 && len(pv.compiledRules.globalParams) == 0 {
		return ""
	}

	filteredParams, _, err := pv.parseAndFilterQueryParams(queryString, paramsRules)
	if err != nil {
		return ""
	}

	return filteredParams
}

// FilterQueryParams filters query parameters string according to validation rules
func (pv *ParamValidator) FilterQueryParams(urlPath, queryString string) string {
	if !pv.initialized || queryString == "" {
		return ""
	}

	if err := pv.validateInputSize(urlPath, MaxURLLength); err != nil {
		return ""
	}

	pv.mu.RLock()
	defer pv.mu.RUnlock()

	return pv.filterQueryParamsUnsafe(urlPath, queryString)
}

// ValidateQueryParams validates query parameters string for URL path
func (pv *ParamValidator) ValidateQueryParams(urlPath, queryString string) bool {
	if !pv.initialized || urlPath == "" {
		return false
	}

	if err := pv.validateInputSize(urlPath, MaxURLLength); err != nil {
		return false
	}

	if err := pv.validateInputSize(queryString, MaxURLLength); err != nil {
		return false
	}

	if queryString == "" {
		return true
	}

	pv.mu.RLock()
	defer pv.mu.RUnlock()

	paramsRules := pv.getParamsForURLUnsafe(urlPath)

	if pv.isAllowAllParams(paramsRules) {
		return true
	}

	if len(paramsRules) == 0 {
		return false
	}

	valid, err := pv.parseAndValidateQueryParams(queryString, paramsRules)
	return err == nil && valid
}
