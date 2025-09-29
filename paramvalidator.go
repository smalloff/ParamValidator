package paramvalidator

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

// WithCallback sets the callback function for validation
func WithCallback(callback CallbackFunc) Option {
	return func(pv *ParamValidator) {
		pv.callbackFunc = callback
	}
}

// WithPlugins registers plugins for rule parser
func WithPlugins(plugins ...PluginConstraintParser) Option {
	return func(pv *ParamValidator) {
		if pv.parser == nil {
			pv.parser = NewRuleParser()
		}
		for _, plugin := range plugins {
			pv.parser.RegisterPlugin(plugin)
		}
	}
}

// NewParamValidator creates a new parameter validator with the given rules
func NewParamValidator(rulesStr string, options ...Option) (*ParamValidator, error) {
	pv := &ParamValidator{
		globalParams: make(map[string]*ParamRule),
		urlRules:     make(map[string]*URLRule),
		urlMatcher:   NewURLMatcher(),
		paramIndex:   NewParamIndex(),
		parser:       NewRuleParser(),
	}
	pv.initialized.Store(true)

	// Apply options
	for _, option := range options {
		option(pv)
	}

	if rulesStr != "" {
		if err := pv.checkSize(rulesStr, MaxRulesSize, "rules string"); err != nil {
			return nil, err
		}

		if err := pv.ParseRules(rulesStr); err != nil {
			return nil, fmt.Errorf("failed to parse initial rules: %w", err)
		}
	}

	return pv, nil
}

// SetCallback sets the custom validation callback function
func (pv *ParamValidator) SetCallback(callback CallbackFunc) {
	pv.mu.Lock()
	defer pv.mu.Unlock()
	pv.callbackFunc = callback
}

// checkSize validates input size against maximum allowed size
func (pv *ParamValidator) checkSize(input string, maxSize int, inputType string) error {
	if len(input) > maxSize {
		return fmt.Errorf("%s size %d exceeds maximum allowed size %d", inputType, len(input), maxSize)
	}

	if !utf8.ValidString(input) {
		return fmt.Errorf("%s contains invalid UTF-8 sequence", inputType)
	}

	return nil
}

// withSafeAccess executes function with lock and initialization check
func (pv *ParamValidator) withSafeAccess(fn func() bool) bool {
	if !pv.initialized.Load() {
		return false
	}

	pv.mu.RLock()
	defer pv.mu.RUnlock()
	return fn()
}

// ValidateURL validates complete URL against loaded rules
func (pv *ParamValidator) ValidateURL(fullURL string) bool {
	if !pv.initialized.Load() || fullURL == "" {
		return false
	}

	if len(fullURL) > MaxURLLength {
		return false
	}

	u, err := url.Parse(fullURL)
	if err != nil {
		return false
	}

	if u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	pv.mu.RLock()
	defer pv.mu.RUnlock()

	return pv.validateURLUnsafe(u)
}

// validateURLUnsafe validates URL without locking using masks
func (pv *ParamValidator) validateURLUnsafe(u *url.URL) bool {
	if u.RawQuery == "" {
		return true
	}

	if pv.compiledRules == nil || pv.compiledRules.paramIndex == nil {
		return false
	}

	if containsPathTraversal(u.Path) {
		return false
	}

	masks := pv.getParamMasksForURL(u.Path)

	if idx := pv.compiledRules.paramIndex.GetIndex(PatternAll); idx != -1 && masks.CombinedMask().GetBit(idx) {
		return true
	}

	if masks.CombinedMask().IsEmpty() {
		return false
	}

	return pv.validateQueryParamsFast(u.RawQuery, masks, u.Path)
}

// validateQueryParamsFast fast query parameters validation without allocations
func (pv *ParamValidator) validateQueryParamsFast(queryString string, masks ParamMasks, urlPath string) bool {
	start := 0
	paramCount := 0

	for i := 0; i <= len(queryString); i++ {
		if i == len(queryString) || queryString[i] == '&' {
			if start < i {
				if paramCount >= MaxParamValues {
					return false
				}

				segment := queryString[start:i]
				eqPos := strings.IndexByte(segment, '=')
				var key, value string

				if eqPos == -1 {
					key = segment
					value = ""
				} else {
					key = segment[:eqPos]
					value = segment[eqPos+1:]
				}

				if !pv.isParamAllowedFast(key, value, masks, urlPath) {
					return false
				}

				paramCount++
			}
			start = i + 1
		}
	}
	return true
}

// isParamAllowedFast optimized parameter validation
func (pv *ParamValidator) isParamAllowedFast(paramName, paramValue string, masks ParamMasks, urlPath string) bool {
	idx := pv.compiledRules.paramIndex.GetIndex(paramName)
	if idx == -1 {
		return false
	}

	source := masks.GetRuleSource(idx)

	var rule *ParamRule
	switch source {
	case SourceSpecificURL:
		rule = pv.getParamFromSpecificURL(paramName, urlPath)
	case SourceURL:
		rule = pv.findURLRuleForParamFast(paramName, urlPath)
	case SourceGlobal:
		rule = pv.compiledRules.globalParams[paramName]
	}

	if rule != nil {
		return pv.isValueValid(rule, paramValue, true)
	}
	return false
}

// getParamFromSpecificURL fast search in specific URL rule
func (pv *ParamValidator) getParamFromSpecificURL(paramName, urlPath string) *ParamRule {
	if rule := pv.findMostSpecificURLRuleUnsafe(urlPath); rule != nil {
		return rule.Params[paramName]
	}
	return nil
}

// findURLRuleForParamFast fast URL rule search for parameter
func (pv *ParamValidator) findURLRuleForParamFast(paramName, urlPath string) *ParamRule {
	if pv.urlMatcher != nil {
		if rules := pv.urlMatcher.GetMatchingRules(urlPath); len(rules) > 0 {
			for _, rule := range rules {
				if paramRule, exists := rule.Params[paramName]; exists {
					return paramRule
				}
			}
		}
	}
	return nil
}

// isValueValid universal value validation function
func (pv *ParamValidator) isValueValid(rule *ParamRule, value string, useFast bool) bool {
	if rule == nil {
		return false
	}

	var result bool

	switch rule.Pattern {
	case PatternKeyOnly:
		result = value == ""
	case PatternAny:
		result = true
	case PatternEnum:
		result = false
		for _, allowedValue := range rule.Values {
			if value == allowedValue {
				result = true
				break
			}
		}
	case PatternCallback:
		if pv.callbackFunc != nil {
			result = pv.safeCallback(rule.Name, value, useFast)
		} else {
			result = false
		}
	case "plugin":
		if rule.CustomValidator != nil {
			result = pv.safeCustomValidator(rule.CustomValidator, value, useFast)
		} else {
			result = false
		}
	default:
		result = false
	}

	if rule.Inverted {
		return !result
	}
	return result
}

// safeCallback executes callback with panic protection
func (pv *ParamValidator) safeCallback(paramName, value string, useFast bool) bool {
	if !useFast {
		return pv.callbackFunc(paramName, value)
	}

	defer func() {
		if r := recover(); r != nil {
			// Panic recovered, result remains false
		}
	}()
	return pv.callbackFunc(paramName, value)
}

// safeCustomValidator executes custom validator with panic protection
func (pv *ParamValidator) safeCustomValidator(validator func(string) bool, value string, useFast bool) bool {
	if !useFast {
		return validator(value)
	}

	defer func() {
		if r := recover(); r != nil {
			// Panic recovered, result remains false
		}
	}()
	return validator(value)
}

// getParamMasksForURL optimized version
func (pv *ParamValidator) getParamMasksForURL(urlPath string) ParamMasks {
	masks := ParamMasks{
		Global:      NewParamMask(),
		URL:         NewParamMask(),
		SpecificURL: NewParamMask(),
	}

	if pv.compiledRules == nil || pv.compiledRules.paramIndex == nil {
		return masks
	}

	// 1. Global parameters
	for name := range pv.compiledRules.globalParams {
		if idx := pv.compiledRules.paramIndex.GetIndex(name); idx != -1 {
			masks.Global.SetBit(idx)
		}
	}

	// 2. Most specific rule
	mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath)
	if mostSpecificRule != nil {
		for name := range mostSpecificRule.Params {
			if idx := pv.compiledRules.paramIndex.GetIndex(name); idx != -1 {
				masks.SpecificURL.SetBit(idx)
			}
		}
	}

	// 3. URL rules (excluding overridden parameters in specific rule)
	for pattern, urlRule := range pv.compiledRules.urlRules {
		if pv.urlMatchesPatternUnsafe(urlPath, pattern) {
			for name := range urlRule.Params {
				if mostSpecificRule != nil {
					if _, exists := mostSpecificRule.Params[name]; exists {
						continue
					}
				}
				if idx := pv.compiledRules.paramIndex.GetIndex(name); idx != -1 {
					masks.URL.SetBit(idx)
				}
			}
		}
	}

	return masks
}

// findParamRuleByMasks finds rule considering priorities using masks
func (pv *ParamValidator) findParamRuleByMasks(paramName string, masks ParamMasks, urlPath string) *ParamRule {
	if pv.compiledRules == nil {
		return nil
	}

	idx := pv.compiledRules.paramIndex.GetIndex(paramName)
	if idx == -1 {
		return nil
	}

	source := masks.GetRuleSource(idx)
	if source == SourceNone {
		return nil
	}

	if source == SourceSpecificURL {
		if mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath); mostSpecificRule != nil {
			return mostSpecificRule.Params[paramName]
		}
		return nil
	}

	if source == SourceURL {
		return pv.findURLRuleForParam(paramName, urlPath)
	}

	return pv.compiledRules.globalParams[paramName]
}

// findURLRuleForParam finds URL rule for parameter
func (pv *ParamValidator) findURLRuleForParam(paramName, urlPath string) *ParamRule {
	var mostSpecificURLRule *URLRule

	for pattern, urlRule := range pv.compiledRules.urlRules {
		if pv.urlMatchesPatternUnsafe(urlPath, pattern) {
			if _, exists := urlRule.Params[paramName]; exists {
				if mostSpecificURLRule == nil || isPatternMoreSpecific(pattern, mostSpecificURLRule.URLPattern) {
					mostSpecificURLRule = urlRule
				}
			}
		}
	}

	if mostSpecificURLRule != nil {
		return mostSpecificURLRule.Params[paramName]
	}
	return nil
}

func isPatternMoreSpecific(pattern1, pattern2 string) bool {
	wildcards1 := strings.Count(pattern1, "*")
	wildcards2 := strings.Count(pattern2, "*")

	if wildcards1 != wildcards2 {
		return wildcards1 < wildcards2
	}

	return len(pattern1) > len(pattern2)
}

// isParamAllowedWithMasks checks parameter using mask system
func (pv *ParamValidator) isParamAllowedWithMasks(paramName, paramValue string, masks ParamMasks, urlPath string) bool {
	rule := pv.findParamRuleByMasks(paramName, masks, urlPath)
	return rule != nil && pv.isValueValid(rule, paramValue, false)
}

// parseAndValidateQueryParamsWithMasks parses and validates parameters using masks
func (pv *ParamValidator) parseAndValidateQueryParamsWithMasks(queryString string, masks ParamMasks, urlPath string) (bool, error) {
	if queryString == "" {
		return true, nil
	}

	isValid := true
	allowAll := pv.isAllowAllParamsMasks(masks)

	err := pv.processQueryParamsCommon(queryString, func(keyBytes, valueBytes []byte) {
		key := string(keyBytes)
		value := string(valueBytes)

		if !allowAll && !pv.isParamAllowedWithMasks(key, value, masks, urlPath) {
			isValid = false
		}
	})

	return isValid, err
}

// findMostSpecificURLRuleUnsafe finds most specific matching URL rule
func (pv *ParamValidator) findMostSpecificURLRuleUnsafe(urlPath string) *URLRule {
	if pv.urlMatcher == nil {
		return nil
	}
	return pv.urlMatcher.GetMostSpecificRule(urlPath)
}

// urlMatchesPatternUnsafe checks if URL path matches pattern
func (pv *ParamValidator) urlMatchesPatternUnsafe(urlPath, pattern string) bool {
	return urlMatchesPattern(urlPath, pattern)
}

// ValidateParam validates single parameter value for specific URL path
func (pv *ParamValidator) ValidateParam(urlPath, paramName, paramValue string) bool {
	return pv.withSafeAccess(func() bool {
		if urlPath == "" || paramName == "" {
			return false
		}

		if len(urlPath) > MaxURLLength {
			return false
		}

		return pv.validateParamUnsafe(urlPath, paramName, paramValue)
	})
}

// validateParamUnsafe validates single parameter using masks
func (pv *ParamValidator) validateParamUnsafe(urlPath, paramName, paramValue string) bool {
	if pv.compiledRules == nil {
		return false
	}

	masks := pv.getParamMasksForURL(urlPath)
	return pv.isParamAllowedWithMasks(paramName, paramValue, masks, urlPath)
}

// NormalizeURL optimized version
func (pv *ParamValidator) NormalizeURL(fullURL string) string {
	if !pv.initialized.Load() || fullURL == "" {
		return fullURL
	}

	if len(fullURL) > MaxURLLength {
		return fullURL
	}

	u, err := url.Parse(fullURL)
	if err != nil {
		return fullURL
	}

	pv.mu.RLock()
	defer pv.mu.RUnlock()

	return pv.normalizeURLFast(u)
}

// normalizeURLFast fast normalization
func (pv *ParamValidator) normalizeURLFast(u *url.URL) string {
	if u.RawQuery == "" {
		return u.String()
	}

	if pv.compiledRules == nil || pv.compiledRules.paramIndex == nil {
		return u.Path
	}

	masks := pv.getParamMasksForURL(u.Path)

	if idx := pv.compiledRules.paramIndex.GetIndex(PatternAll); idx != -1 && masks.CombinedMask().GetBit(idx) {
		return u.String()
	}

	if masks.CombinedMask().IsEmpty() {
		return u.Path
	}

	filteredQuery := pv.filterQueryParamsFast(u.RawQuery, masks, u.Path)
	if filteredQuery == "" {
		return u.Path
	}

	return buildNormalizedURL(u.Path, filteredQuery)
}

// buildNormalizedURL builds normalized URL efficiently
func buildNormalizedURL(path, query string) string {
	if len(path)+1+len(query) < 64 {
		var buf [256]byte
		n := copy(buf[:], path)
		buf[n] = '?'
		n++
		n += copy(buf[n:], query)
		return string(buf[:n])
	}

	result := make([]byte, len(path)+1+len(query))
	copy(result, path)
	result[len(path)] = '?'
	copy(result[len(path)+1:], query)
	return string(result)
}

// filterQueryParamsFast fast parameter filtering
func (pv *ParamValidator) filterQueryParamsFast(queryString string, masks ParamMasks, urlPath string) string {
	if queryString == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(queryString))

	start := 0
	firstParam := true

	for i := 0; i <= len(queryString); i++ {
		if i == len(queryString) || queryString[i] == '&' {
			if start < i {
				segment := queryString[start:i]
				if pv.isParamAllowedSegment(segment, masks, urlPath) {
					if !firstParam {
						builder.WriteByte('&')
					} else {
						firstParam = false
					}
					builder.WriteString(segment)
				}
			}
			start = i + 1
		}
	}

	return builder.String()
}

func (pv *ParamValidator) isParamAllowedSegment(segment string, masks ParamMasks, urlPath string) bool {
	eqPos := strings.IndexByte(segment, '=')
	var key, value string

	if eqPos == -1 {
		key = segment
		value = ""
	} else {
		key = segment[:eqPos]
		value = segment[eqPos+1:]
	}

	return pv.isParamAllowedFast(key, value, masks, urlPath)
}

// FilterQueryParams filters query parameters string according to validation rules
func (pv *ParamValidator) FilterQueryParams(urlPath, queryString string) string {
	if !pv.initialized.Load() || queryString == "" {
		return ""
	}

	pv.mu.RLock()
	defer pv.mu.RUnlock()

	if len(urlPath) > MaxURLLength {
		return ""
	}

	return pv.filterQueryParamsFast(queryString, pv.getParamMasksForURL(urlPath), urlPath)
}

// ValidateQueryParams validates query parameters string for URL path
func (pv *ParamValidator) ValidateQueryParams(urlPath, queryString string) bool {
	if !pv.initialized.Load() || urlPath == "" {
		return false
	}

	return pv.withSafeAccess(func() bool {
		if len(urlPath) > MaxURLLength {
			return false
		}

		if queryString == "" {
			return true
		}

		if len(queryString) > MaxURLLength {
			return false
		}

		masks := pv.getParamMasksForURL(urlPath)

		if pv.isAllowAllParamsMasks(masks) {
			return true
		}

		if masks.CombinedMask().IsEmpty() {
			return false
		}

		valid, err := pv.parseAndValidateQueryParamsWithMasks(queryString, masks, urlPath)
		return err == nil && valid
	})
}

// processQueryParamsCommon fast parameter processing using byte slices
func (pv *ParamValidator) processQueryParamsCommon(queryString string, processor func(key, value []byte)) error {
	if queryString == "" {
		return nil
	}

	queryBytes := []byte(queryString)
	start := 0
	paramCount := 0

	for i := 0; i <= len(queryBytes); i++ {
		if i == len(queryBytes) || queryBytes[i] == '&' {
			if start < i && paramCount < MaxParamValues {
				segment := queryBytes[start:i]
				eqPos := bytes.IndexByte(segment, '=')

				var key, value []byte
				if eqPos == -1 {
					key = segment
					value = nil
				} else {
					key = segment[:eqPos]
					value = segment[eqPos+1:]
				}

				processor(key, value)
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

// Clear removes all validation rules
func (pv *ParamValidator) Clear() {
	pv.mu.Lock()
	defer pv.mu.Unlock()
	pv.clearUnsafe()
}

// ClearRules clears all loaded validation rules
func (pv *ParamValidator) ClearRules() {
	pv.Clear()
}

// clearUnsafe resets all rules without locking
func (pv *ParamValidator) clearUnsafe() {
	if pv.parser != nil {
		pv.parser.ClearCache()
	}

	pv.globalParams = make(map[string]*ParamRule)
	pv.urlRules = make(map[string]*URLRule)
	pv.compiledRules = &CompiledRules{
		globalParams: make(map[string]*ParamRule),
		urlRules:     make(map[string]*URLRule),
		paramIndex:   NewParamIndex(),
	}
	if pv.paramIndex != nil {
		pv.paramIndex.Clear()
	}
	pv.urlMatcher.ClearRules()
}

// copyParamRuleUnsafe creates a deep copy of ParamRule
func (pv *ParamValidator) copyParamRuleUnsafe(rule *ParamRule) *ParamRule {
	if rule == nil {
		return nil
	}

	ruleCopy := &ParamRule{
		Name:            rule.Name,
		Pattern:         rule.Pattern,
		CustomValidator: rule.CustomValidator,
		Inverted:        rule.Inverted,
	}

	if rule.Values != nil {
		ruleCopy.Values = make([]string, len(rule.Values))
		copy(ruleCopy.Values, rule.Values)
	}

	return ruleCopy
}

// copyURLRuleUnsafe creates a deep copy of URLRule
func (pv *ParamValidator) copyURLRuleUnsafe(rule *URLRule) *URLRule {
	if rule == nil {
		return nil
	}

	ruleCopy := &URLRule{
		URLPattern: rule.URLPattern,
		Params:     make(map[string]*ParamRule),
		ParamMask:  NewParamMask(),
	}

	for paramName, paramRule := range rule.Params {
		ruleCopy.Params[paramName] = pv.copyParamRuleUnsafe(paramRule)
	}

	return ruleCopy
}

// ParseRules parses and loads validation rules from string
func (pv *ParamValidator) ParseRules(rulesStr string) error {
	if !pv.initialized.Load() {
		return fmt.Errorf("validator not initialized")
	}

	if rulesStr == "" {
		pv.mu.Lock()
		defer pv.mu.Unlock()
		pv.clearUnsafe()
		return nil
	}

	if err := pv.checkSize(rulesStr, MaxRulesSize, "rules string"); err != nil {
		return err
	}

	pv.mu.Lock()
	defer pv.mu.Unlock()

	if pv.parser != nil {
		pv.parser.ClearCache()
	}

	globalParams, urlRules, err := pv.parser.parseRulesUnsafe(rulesStr)
	if err != nil {
		return err
	}

	pv.globalParams = globalParams
	pv.urlRules = urlRules
	pv.compileRulesUnsafe()
	return nil
}

// compileRulesUnsafe compiles rules for faster access with masks
func (pv *ParamValidator) compileRulesUnsafe() {
	if pv.paramIndex == nil {
		pv.paramIndex = NewParamIndex()
	} else {
		pv.paramIndex.Clear()
	}

	pv.compiledRules = &CompiledRules{
		globalParams: make(map[string]*ParamRule),
		urlRules:     make(map[string]*URLRule),
		paramIndex:   pv.paramIndex,
	}

	// Copy global parameters and index them
	for name, rule := range pv.globalParams {
		ruleCopy := pv.copyParamRuleUnsafe(rule)
		idx := pv.paramIndex.GetOrCreateIndex(name)
		if idx != -1 {
			ruleCopy.BitmaskIndex = idx
			pv.compiledRules.globalParams[name] = ruleCopy
		}
	}

	// Copy URL rules and create bit masks for them
	for pattern, rule := range pv.urlRules {
		ruleCopy := &URLRule{
			URLPattern: rule.URLPattern,
			Params:     make(map[string]*ParamRule),
			ParamMask:  NewParamMask(),
		}

		for paramName, paramRule := range rule.Params {
			paramRuleCopy := pv.copyParamRuleUnsafe(paramRule)
			idx := pv.paramIndex.GetOrCreateIndex(paramName)
			if idx != -1 {
				paramRuleCopy.BitmaskIndex = idx
				ruleCopy.Params[paramName] = paramRuleCopy
				ruleCopy.ParamMask.SetBit(idx)
			}
		}

		pv.compiledRules.urlRules[pattern] = ruleCopy
	}

	pv.updateURLMatcherUnsafe()
}

// updateURLMatcherUnsafe updates URLMatcher with current rules
func (pv *ParamValidator) updateURLMatcherUnsafe() {
	if pv.urlMatcher == nil {
		pv.urlMatcher = NewURLMatcher()
	} else {
		pv.urlMatcher.ClearRules()
	}

	for pattern, rule := range pv.urlRules {
		ruleCopy := pv.copyURLRuleUnsafe(rule)
		pv.urlMatcher.AddRule(pattern, ruleCopy)
	}
}

// Close releases validator resources
func (pv *ParamValidator) Close() error {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	if !pv.initialized.Load() {
		return nil
	}

	if pv.parser != nil {
		if err := pv.parser.Close(); err != nil {
			return fmt.Errorf("failed to close parser: %w", err)
		}
	}

	pv.clearUnsafe()
	pv.initialized.Store(false)
	return nil
}

// Reset resets rules
func (pv *ParamValidator) Reset() {
	pv.mu.Lock()
	defer pv.mu.Unlock()
	pv.clearUnsafe()
	pv.initialized.Store(true)
	pv.callbackFunc = nil
}

// isAllowAllParamsMasks checks if masks allow all parameters
func (pv *ParamValidator) isAllowAllParamsMasks(masks ParamMasks) bool {
	if pv.compiledRules == nil || pv.compiledRules.paramIndex == nil {
		return false
	}

	idx := pv.compiledRules.paramIndex.GetIndex(PatternAll)
	return idx != -1 && masks.CombinedMask().GetBit(idx)
}
