package paramvalidator

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"unsafe"
)

var builderPool = sync.Pool{
	New: func() interface{} {
		b := &strings.Builder{}
		b.Grow(128)
		return b
	},
}

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

	pv.mu.RLock()
	defer pv.mu.RUnlock()

	return pv.validateURLUnsafe(u)
}

// validateURLUnsafe validates URL without locking using masks
func (pv *ParamValidator) validateURLUnsafe(u *url.URL) bool {
	// Fast check - if no query parameters, always valid
	if u.RawQuery == "" {
		return true
	}

	if pv.compiledRules == nil || pv.compiledRules.paramIndex == nil {
		return false
	}

	masks := pv.getParamMasksForURL(u.Path)

	// Fast allow all check
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
					return false // First invalid parameter fails validation
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
	if source == SourceNone {
		return false
	}

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
		return pv.isValueValidFast(rule, paramValue)
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

// isValueValidFast fast value validation
func (pv *ParamValidator) isValueValidFast(rule *ParamRule, value string) bool {
	switch rule.Pattern {
	case PatternKeyOnly:
		return value == ""
	case PatternAny:
		return true
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
	case "plugin":
		if rule.CustomValidator != nil {
			return rule.CustomValidator(value)
		}
		return false
	default:
		return false
	}
}

// isAllowAllParamsMasks checks if masks allow all parameters
func (pv *ParamValidator) isAllowAllParamsMasks(masks ParamMasks) bool {
	if pv.compiledRules == nil || pv.compiledRules.paramIndex == nil {
		return false
	}

	idx := pv.compiledRules.paramIndex.GetIndex(PatternAll)
	return idx != -1 && masks.CombinedMask().GetBit(idx)
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

	urlPath = NormalizeURLPattern(urlPath)

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
				// Skip if parameter already in specific rule
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

	// SpecificURL has absolute priority
	if source == SourceSpecificURL {
		if mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath); mostSpecificRule != nil {
			return mostSpecificRule.Params[paramName]
		}
		return nil
	}

	// For URL rules: find first matching rule
	if source == SourceURL {
		return pv.findURLRuleForParam(paramName, urlPath)
	}

	// Global rules
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
	return rule != nil && pv.isValueValidUnsafe(rule, paramValue)
}

// parseAndValidateQueryParamsWithMasks parses and validates parameters using masks
func (pv *ParamValidator) parseAndValidateQueryParamsWithMasks(queryString string, masks ParamMasks, urlPath string) (bool, error) {
	if queryString == "" {
		return true, nil
	}

	isValid := true
	allowAll := pv.isAllowAllParamsMasks(masks)

	err := pv.processQueryParamsCommon(queryString, func(keyBytes, valueBytes []byte) {
		key := unsafeGetString(keyBytes)
		value := unsafeGetString(valueBytes)

		if !allowAll && !pv.isParamAllowedWithMasks(key, value, masks, urlPath) {
			isValid = false
		}
	})

	return isValid, err
}

// parseAndFilterQueryParamsWithMasks filters parameters using masks
func (pv *ParamValidator) parseAndFilterQueryParamsWithMasks(queryString string, masks ParamMasks, urlPath string) (string, bool, error) {
	if queryString == "" {
		return "", true, nil
	}

	filteredParams := builderPool.Get().(*strings.Builder)
	defer func() {
		filteredParams.Reset()
		builderPool.Put(filteredParams)
	}()

	isValid := true
	allowAll := pv.isAllowAllParamsMasks(masks)
	firstParam := true

	err := pv.processQueryParamsCommon(queryString, func(keyBytes, valueBytes []byte) {
		key := unsafeGetString(keyBytes)
		value := unsafeGetString(valueBytes)

		// Restore original key and value from bytes
		originalKey := string(keyBytes)
		originalValue := string(valueBytes)

		if allowAll || pv.isParamAllowedWithMasks(key, value, masks, urlPath) {
			if !firstParam {
				filteredParams.WriteString("&")
			} else {
				firstParam = false
			}

			if len(valueBytes) == 0 {
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

// isValueValidUnsafe checks if value is valid according to rule
func (pv *ParamValidator) isValueValidUnsafe(rule *ParamRule, value string) bool {
	if rule == nil {
		return false
	}

	switch rule.Pattern {
	case PatternKeyOnly:
		return value == ""
	case PatternAny:
		return true
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
	case "plugin":
		if rule.CustomValidator != nil {
			return rule.CustomValidator(value)
		}
		return false
	default:
		return false
	}
}

// ValidateParam validates single parameter value for specific URL path
func (pv *ParamValidator) ValidateParam(urlPath, paramName, paramValue string) bool {
	return pv.withSafeAccess(func() bool {
		if urlPath == "" || paramName == "" {
			return false
		}

		if err := pv.checkSize(urlPath, MaxURLLength, "URL path"); err != nil {
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
	// No parameters - return as is
	if u.RawQuery == "" {
		return u.String()
	}

	if pv.compiledRules == nil || pv.compiledRules.paramIndex == nil {
		return u.Path // No rules - remove parameters
	}

	masks := pv.getParamMasksForURL(u.Path)

	// Check for allow all
	if idx := pv.compiledRules.paramIndex.GetIndex(PatternAll); idx != -1 && masks.CombinedMask().GetBit(idx) {
		return u.String()
	}

	// No allowed parameters
	if masks.CombinedMask().IsEmpty() {
		return u.Path
	}

	// Filter parameters
	filtered := pv.filterQueryParamsFast(u.RawQuery, masks, u.Path)
	if filtered == "" {
		return u.Path
	}

	if filtered == u.RawQuery {
		return u.String()
	}

	// Build URL optimally
	if u.Path == "" {
		return "?" + filtered
	}
	return u.Path + "?" + filtered
}

// filterQueryParamsFast fast parameter filtering
func (pv *ParamValidator) filterQueryParamsFast(queryString string, masks ParamMasks, urlPath string) string {
	if queryString == "" {
		return ""
	}

	var result strings.Builder
	result.Grow(len(queryString)) // Pre-allocate memory
	start := 0
	firstParam := true

	for i := 0; i <= len(queryString); i++ {
		if i == len(queryString) || queryString[i] == '&' {
			if start < i {
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

				if pv.isParamAllowedFast(key, value, masks, urlPath) {
					if !firstParam {
						result.WriteByte('&')
					} else {
						firstParam = false
					}
					result.WriteString(segment)
				}
			}
			start = i + 1
		}
	}

	return result.String()
}

// FilterQueryParams filters query parameters string according to validation rules
func (pv *ParamValidator) FilterQueryParams(urlPath, queryString string) string {
	if !pv.initialized.Load() || queryString == "" {
		return ""
	}

	var result string
	pv.withSafeAccess(func() bool {
		if err := pv.checkSize(urlPath, MaxURLLength, "URL path"); err != nil {
			result = ""
			return false
		}

		result = pv.filterQueryParamsUnsafe(urlPath, queryString)
		return true
	})

	return result
}

// filterQueryParamsUnsafe filters query parameters using masks
func (pv *ParamValidator) filterQueryParamsUnsafe(urlPath, queryString string) string {
	masks := pv.getParamMasksForURL(urlPath)

	if pv.isAllowAllParamsMasks(masks) {
		return queryString
	}

	if masks.CombinedMask().IsEmpty() {
		return ""
	}

	filteredParams, _, err := pv.parseAndFilterQueryParamsWithMasks(queryString, masks, urlPath)
	if err != nil {
		return ""
	}

	return filteredParams
}

// ValidateQueryParams validates query parameters string for URL path
func (pv *ParamValidator) ValidateQueryParams(urlPath, queryString string) bool {
	if !pv.initialized.Load() || urlPath == "" {
		return false
	}

	return pv.withSafeAccess(func() bool {
		if err := pv.checkSize(urlPath, MaxURLLength, "URL path"); err != nil {
			return false
		}

		if queryString == "" {
			return true
		}

		if err := pv.checkSize(queryString, MaxURLLength, "query string"); err != nil {
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

	queryBytes := unsafeGetBytes(queryString)
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

// unsafeGetBytes converts string to []byte without allocation (USE WITH CAUTION!)
func unsafeGetBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&s))
}

// unsafeGetString converts []byte to string without allocation
func unsafeGetString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
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
	// Очищаем кэш парсера
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
	// Clear index before compiling new rules
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

		// Copy parameters and update indexes
		for paramName, paramRule := range rule.Params {
			paramRuleCopy := pv.copyParamRuleUnsafe(paramRule)
			idx := pv.paramIndex.GetOrCreateIndex(paramName)
			if idx != -1 {
				paramRuleCopy.BitmaskIndex = idx
				ruleCopy.Params[paramName] = paramRuleCopy
				// Set bit in URL rule mask
				ruleCopy.ParamMask.SetBit(idx)
			}
		}

		pv.compiledRules.urlRules[pattern] = ruleCopy
	}

	// Update URLMatcher
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
