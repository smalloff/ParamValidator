package paramvalidator

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ValidateURL validates complete URL against loaded rules
// fullURL: Complete URL to validate including query parameters
// Returns true if URL and all parameters are valid according to rules
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
	if err != nil {
		return false
	}

	return valid
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

	for name, rule := range pv.compiledRules.globalParams {
		result[name] = rule
	}

	for pattern, rule := range pv.compiledRules.urlRules {
		if pv.urlMatchesPatternUnsafe(urlPath, pattern) {
			for paramName, paramRule := range rule.Params {
				result[paramName] = paramRule
			}
		}
	}

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

	hasWildcard := false
	wildcardCount := 0
	slashCount := 0
	lastCharIsWildcard := false
	hasMiddleWildcard := false

	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			wildcardCount++
			hasWildcard = true
			if i == len(pattern)-1 {
				lastCharIsWildcard = true
			} else if i > 0 {
				hasMiddleWildcard = true
			}
		case '/':
			slashCount++
		}
	}

	pathSegmentCount := slashCount
	if len(pattern) > 0 && pattern[0] != '/' {
		pathSegmentCount++
	}

	specificity := pathSegmentCount * 100

	if !hasWildcard {
		specificity += 1500
	} else {
		specificity -= wildcardCount * 200

		if lastCharIsWildcard {
			specificity -= 300
		}
		if hasMiddleWildcard {
			specificity -= 100
		}
	}

	if slashCount > 1 {
		specificity += slashCount * 50
	}

	return specificity
}

// urlMatchesPatternUnsafe checks if URL path matches pattern
func (pv *ParamValidator) urlMatchesPatternUnsafe(urlPath, pattern string) bool {
	urlPath = pv.normalizeURLPattern(urlPath)

	if pattern == PatternAll || pattern == urlPath {
		return true
	}

	if strings.HasSuffix(pattern, PatternAll) {
		prefix := strings.TrimSuffix(pattern, PatternAll)
		if prefix == "" {
			return true
		}
		prefix = strings.TrimSuffix(prefix, "/")
		return strings.HasPrefix(urlPath, prefix)
	}

	if strings.Contains(pattern, "*") {
		return pv.wildcardMatch(urlPath, pattern)
	}

	return pattern == urlPath
}

// wildcardMatch performs wildcard pattern matching
func (pv *ParamValidator) wildcardMatch(urlPath, pattern string) bool {
	maxSegments := 50
	maxSegmentLength := 200
	urlSegments := strings.Split(strings.Trim(urlPath, "/"), "/")
	patternSegments := strings.Split(strings.Trim(pattern, "/"), "/")

	if len(urlSegments) > maxSegments || len(patternSegments) > maxSegments {
		return false
	}

	if len(urlSegments) != len(patternSegments) {
		return false
	}

	for i := 0; i < len(urlSegments); i++ {
		if patternSegments[i] == "*" {
			continue
		}
		if len(urlSegments[i]) > maxSegmentLength || len(patternSegments[i]) > maxSegmentLength {
			return false
		}
		if patternSegments[i] != urlSegments[i] {
			return false
		}
	}

	return true
}

// isValueValidUnsafe checks if value is valid according to rule
func (pv *ParamValidator) isValueValidUnsafe(rule *ParamRule, value string) bool {
	switch rule.Pattern {
	case PatternKeyOnly:
		return value == ""
	case PatternAny:
		return true
	case PatternRange:
		num, err := strconv.ParseInt(value, 10, 64)
		return err == nil && num >= rule.Min && num <= rule.Max
	case PatternEnum:
		for _, allowedValue := range rule.Values {
			if value == allowedValue {
				return true
			}
		}
		return false
	case PatternCallback:
		if pv.callbackFunc != nil {
			return pv.callbackFunc(rule.Name, value)
		}
		return false
	default:
		return false
	}
}

// ValidateParam validates single parameter value for specific URL path
// urlPath: URL path to match against rules
// paramName: Name of parameter to validate
// paramValue: Value of parameter to validate
// Returns true if parameter is allowed and value is valid
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

	if rule == nil {
		return false
	}

	return pv.isValueValidUnsafe(rule, paramValue)
}

// NormalizeURL filters and normalizes URL according to validation rules
// fullURL: Complete URL to normalize
// Returns normalized URL with only allowed parameters and values
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

	var filteredParams string
	if u.RawQuery != "" {
		filteredParams, _, err = pv.parseAndFilterQueryParams(u.RawQuery, paramsRules)
		if err != nil {
			return u.Path
		}
	}

	if len(filteredParams) > 0 {
		u.RawQuery = filteredParams
		return u.String()
	}

	return u.Path
}

// parseAndValidateQueryParams parses and validates query parameters, returning validation result
func (pv *ParamValidator) parseAndValidateQueryParams(queryString string, paramsRules map[string]*ParamRule) (bool, error) {
	if queryString == "" {
		return true, nil
	}

	paramCount := strings.Count(queryString, "&") + 1
	if paramCount > MaxParamValues {
		return false, fmt.Errorf("too many parameters")
	}

	isValid := true
	allowAll := pv.isAllowAllParams(paramsRules)

	for len(queryString) > 0 && paramCount > 0 {
		var segment string
		if pos := strings.IndexByte(queryString, '&'); pos >= 0 {
			segment = queryString[:pos]
			queryString = queryString[pos+1:]
		} else {
			segment = queryString
			queryString = ""
		}
		paramCount--

		if segment == "" {
			continue
		}

		eqPos := strings.IndexByte(segment, '=')
		var key, value string

		if eqPos == -1 {
			decodedKey, err := url.QueryUnescape(segment)
			if err != nil {
				isValid = false
				continue
			}
			key = decodedKey
			value = ""
		} else {
			originalKey := segment[:eqPos]
			originalValue := segment[eqPos+1:]

			decodedKey, err1 := url.QueryUnescape(originalKey)
			decodedValue, err2 := url.QueryUnescape(originalValue)

			if err1 != nil || err2 != nil {
				isValid = false
				continue
			}
			key = decodedKey
			value = decodedValue
		}

		if !allowAll && !pv.isParamAllowedUnsafe(key, value, paramsRules) {
			isValid = false
		}
	}

	return isValid, nil
}

// parseAndFilterQueryParams parses and filters query parameters, returning filtered query string
func (pv *ParamValidator) parseAndFilterQueryParams(queryString string, paramsRules map[string]*ParamRule) (string, bool, error) {
	if queryString == "" {
		return "", false, nil
	}

	paramCount := strings.Count(queryString, "&") + 1
	if paramCount > MaxParamValues {
		return "", false, fmt.Errorf("too many parameters")
	}

	filteredParams := ""

	isValid := true
	allowAll := pv.isAllowAllParams(paramsRules)
	firstParam := true

	for len(queryString) > 0 && paramCount > 0 {
		var segment string
		if pos := strings.IndexByte(queryString, '&'); pos >= 0 {
			segment = queryString[:pos]
			queryString = queryString[pos+1:]
		} else {
			segment = queryString
			queryString = ""
		}
		paramCount--

		if segment == "" {
			continue
		}

		eqPos := strings.IndexByte(segment, '=')
		var key, value string
		var originalKey, originalValue string

		if eqPos == -1 {
			originalKey = segment
			decodedKey, err := url.QueryUnescape(segment)
			if err != nil {
				isValid = false
				continue
			}
			key = decodedKey
			value = ""
			originalValue = ""
		} else {
			originalKey = segment[:eqPos]
			originalValue = segment[eqPos+1:]

			decodedKey, err1 := url.QueryUnescape(originalKey)
			decodedValue, err2 := url.QueryUnescape(originalValue)

			if err1 != nil || err2 != nil {
				isValid = false
				continue
			}
			key = decodedKey
			value = decodedValue
		}

		if allowAll || pv.isParamAllowedUnsafe(key, value, paramsRules) {
			if !firstParam {
				filteredParams += "&"
			} else {
				firstParam = false
			}

			if eqPos == -1 {
				filteredParams += originalKey
			} else {
				filteredParams += originalKey + "=" + originalValue
			}
		} else if !allowAll {
			isValid = false
		}
	}

	return filteredParams, isValid, nil
}

func (pv *ParamValidator) isParamAllowedUnsafe(paramName, paramValue string, paramsRules map[string]*ParamRule) bool {
	rule := pv.findParamRule(paramName, paramsRules)
	if rule == nil {
		return false
	}
	return pv.isValueValidUnsafe(rule, paramValue)
}

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
// urlPath: URL path to match against rules
// queryString: Query parameters string to filter
// Returns filtered query parameters string containing only allowed parameters and values
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
// urlPath: URL path to match against rules
// queryString: Query parameters string to validate
// Returns true if all parameters and values are valid according to rules
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

	paramsRules := pv.getParamsForURLUnsafe(urlPath)

	if pv.isAllowAllParams(paramsRules) {
		return true
	}

	if len(paramsRules) == 0 {
		return false
	}

	valid, err := pv.parseAndValidateQueryParams(queryString, paramsRules)
	if err != nil {
		return false
	}
	return valid
}
