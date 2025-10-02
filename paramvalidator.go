// paramvalidator.go
package paramvalidator

import (
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

	masks := pv.getParamMasksForURL(u.Path)

	if idx := pv.compiledRules.paramIndex.GetIndex(PatternAll); idx != -1 && masks.CombinedMask().GetBit(idx) {
		return true
	}

	if masks.CombinedMask().IsEmpty() {
		return false
	}

	return pv.validateQueryParams(u.RawQuery, masks, u.Path, false)
}

// validateQueryParams universal query parameters validation
func (pv *ParamValidator) validateQueryParams(queryString string, masks ParamMasks, urlPath string, useBytes bool) bool {
	if queryString == "" {
		return true
	}

	allowAll := pv.isAllowAllParamsMasks(masks)
	start := 0
	paramCount := 0

	for i := 0; i <= len(queryString); i++ {
		if i == len(queryString) || queryString[i] == '&' {
			if start < i && paramCount < MaxParamValues {
				if !allowAll {
					var allowed bool
					if useBytes {
						allowed = pv.isParamAllowedBytesSegment([]byte(queryString[start:i]), masks, urlPath)
					} else {
						allowed = pv.isParamAllowedSegment(queryString[start:i], masks, urlPath)
					}
					if !allowed {
						return false
					}
				}
				paramCount++
			}
			start = i + 1
		}
	}
	return paramCount <= MaxParamValues
}

// parseQuerySegment parses query segment and returns positions
func (pv *ParamValidator) parseQuerySegment(queryString string, start, end int) (eqPos, keyStart, keyEnd, valStart, valEnd int) {
	keyStart = start
	eqPos = -1

	for j := start; j < end; j++ {
		if queryString[j] == '=' {
			eqPos = j
			break
		}
	}

	if eqPos == -1 {
		keyEnd = end
		valStart = end
		valEnd = end
	} else {
		keyEnd = eqPos
		valStart = eqPos + 1
		valEnd = end
	}

	return eqPos, keyStart, keyEnd, valStart, valEnd
}

// GetIndexByRange finds parameter by byte range without creating string
func (pi *ParamIndex) GetIndexByRange(str string, start, end int) int {
	length := end - start
	if length <= 0 {
		return -1
	}

	var result int = -1
	pi.paramToIndex.Range(func(key, value interface{}) bool {
		paramName := key.(string)
		if len(paramName) == length {
			match := true
			for i := 0; i < length; i++ {
				if str[start+i] != paramName[i] {
					match = false
					break
				}
			}
			if match {
				result = value.(int)
				return false
			}
		}
		return true
	})
	return result
}

// isParamAllowedFast optimized parameter validation
func (pv *ParamValidator) isParamAllowedFast(paramName, paramValue string, masks ParamMasks, urlPath string) bool {
	idx := pv.compiledRules.paramIndex.GetIndex(paramName)
	if idx == -1 {
		return false
	}

	if !masks.CombinedMask().GetBit(idx) {
		return false
	}

	rule := pv.findParamRuleByIndex(idx, masks, urlPath)
	return rule != nil && pv.isValueValidFast(rule, paramValue)
}

// findParamRuleByIndex finds rule by index without name lookup
func (pv *ParamValidator) findParamRuleByIndex(paramIndex int, masks ParamMasks, urlPath string) *ParamRule {
	source := masks.GetRuleSource(paramIndex)

	switch source {
	case SourceSpecificURL:
		if mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath); mostSpecificRule != nil {
			return pv.findParamInURLRuleByIndex(mostSpecificRule, paramIndex)
		}
	case SourceURL:
		return pv.findURLRuleForParamByIndex(paramIndex, urlPath)
	case SourceGlobal:
		return pv.findGlobalParamByIndex(paramIndex)
	}
	return nil
}

// findGlobalParamByIndex finds global parameter by index (read-only)
func (pv *ParamValidator) findGlobalParamByIndex(paramIndex int) *ParamRule {
	return pv.compiledRules.globalParamsByIndex[paramIndex]
}

// findURLRuleForParamByIndex finds URL rule by parameter index
func (pv *ParamValidator) findURLRuleForParamByIndex(paramIndex int, urlPath string) *ParamRule {
	rules := pv.compiledRules.urlRulesByIndex[paramIndex]
	if len(rules) == 0 {
		return nil
	}

	var mostSpecificRule *URLRule
	for _, rule := range rules {
		if pv.urlMatchesPatternUnsafe(urlPath, rule.URLPattern) {
			if mostSpecificRule == nil || isPatternMoreSpecific(rule.URLPattern, mostSpecificRule.URLPattern) {
				mostSpecificRule = rule
			}
		}
	}

	if mostSpecificRule != nil {
		return mostSpecificRule.paramsByIndex[paramIndex]
	}
	return nil
}

// findParamInURLRuleByIndex finds parameter in URL rule by index
func (pv *ParamValidator) findParamInURLRuleByIndex(urlRule *URLRule, paramIndex int) *ParamRule {
	if urlRule.paramsByIndex == nil {
		pv.buildURLRuleParamsByIndex(urlRule)
	}
	return urlRule.paramsByIndex[paramIndex]
}

// buildURLRuleParamsByIndex builds URL rule parameters cache by index
func (pv *ParamValidator) buildURLRuleParamsByIndex(urlRule *URLRule) {
	urlRule.paramsByIndex = make(map[int]*ParamRule)
	for paramName, paramRule := range urlRule.Params {
		if idx := pv.compiledRules.paramIndex.GetIndex(paramName); idx != -1 {
			urlRule.paramsByIndex[idx] = paramRule
		}
	}
}

// isValueValidFast optimized value validation without callback panic protection for fast path
func (pv *ParamValidator) isValueValidFast(rule *ParamRule, value string) bool {
	return pv.isValueValidInternal(rule, value, true)
}

// isValueValid universal value validation function with panic protection
func (pv *ParamValidator) isValueValid(rule *ParamRule, value string, useFast bool) bool {
	return pv.isValueValidInternal(rule, value, useFast)
}

// isValueValidInternal internal implementation of value validation
func (pv *ParamValidator) isValueValidInternal(rule *ParamRule, value string, useFast bool) bool {
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
		result = pv.validateEnum(rule, value, useFast)
	case PatternCallback:
		result = pv.validateCallback(rule, value, useFast)
	case "plugin":
		result = pv.validatePlugin(rule, value, useFast)
	default:
		result = false
	}

	if rule.Inverted {
		return !result
	}
	return result
}

// validateEnum validates enum pattern
func (pv *ParamValidator) validateEnum(rule *ParamRule, value string, useFast bool) bool {
	if useFast {
		for i := 0; i < len(rule.Values); i++ {
			if value == rule.Values[i] {
				return true
			}
		}
		return false
	}

	for _, allowedValue := range rule.Values {
		if value == allowedValue {
			return true
		}
	}
	return false
}

// validateCallback validates callback pattern
func (pv *ParamValidator) validateCallback(rule *ParamRule, value string, useFast bool) bool {
	if pv.callbackFunc == nil {
		return false
	}

	if useFast {
		return pv.callbackFunc(rule.Name, value)
	}
	return pv.safeCallback(rule.Name, value)
}

// validatePlugin validates plugin pattern
func (pv *ParamValidator) validatePlugin(rule *ParamRule, value string, useFast bool) bool {
	if rule.CustomValidator == nil {
		return false
	}

	if useFast {
		return rule.CustomValidator(value)
	}
	return pv.safeCustomValidator(rule.CustomValidator, value)
}

// safeCallback executes callback with panic protection
func (pv *ParamValidator) safeCallback(paramName, value string) (result bool) {
	defer func() {
		if r := recover(); r != nil {
			result = false
		}
	}()
	return pv.callbackFunc(paramName, value)
}

// safeCustomValidator executes custom validator with panic protection
func (pv *ParamValidator) safeCustomValidator(validator func(string) bool, value string) (result bool) {
	defer func() {
		if r := recover(); r != nil {
			result = false
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

	// Global parameters
	for name := range pv.compiledRules.globalParams {
		if idx := pv.compiledRules.paramIndex.GetIndex(name); idx != -1 {
			masks.Global.SetBit(idx)
		}
	}

	// Most specific rule
	mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath)
	if mostSpecificRule != nil {
		for name := range mostSpecificRule.Params {
			if idx := pv.compiledRules.paramIndex.GetIndex(name); idx != -1 {
				masks.SpecificURL.SetBit(idx)
			}
		}
	}

	// URL rules - INCLUDE all parameters, but check priorities during validation
	for pattern, urlRule := range pv.compiledRules.urlRules {
		if pv.urlMatchesPatternUnsafe(urlPath, pattern) {
			for name := range urlRule.Params {
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

	// Check in priority order: SpecificURL -> URL -> Global
	if masks.SpecificURL.GetBit(idx) {
		if mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath); mostSpecificRule != nil {
			if rule, exists := mostSpecificRule.Params[paramName]; exists {
				return rule
			}
		}
	}

	if masks.URL.GetBit(idx) {
		if rule := pv.findURLRuleForParam(paramName, urlPath); rule != nil {
			return rule
		}
	}

	if masks.Global.GetBit(idx) {
		return pv.compiledRules.globalParams[paramName]
	}

	return nil
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
	if rule == nil {
		return false
	}

	if masks.SpecificURL.GetBit(pv.compiledRules.paramIndex.GetIndex(paramName)) {
		if mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath); mostSpecificRule != nil {
			if specificRule, exists := mostSpecificRule.Params[paramName]; exists {
				return pv.isValueValid(specificRule, paramValue, false)
			}
		}
	}

	return pv.isValueValid(rule, paramValue, false)
}

// FilterQueryBytes filters query parameters into provided buffer
// Returns slice of buffer containing filtered parameters (zero allocations)
// buffer must have sufficient capacity (at least len(queryString))
func (pv *ParamValidator) FilterQueryBytes(urlPath, queryBytes, buffer []byte) []byte {
	if !pv.initialized.Load() || len(queryBytes) == 0 {
		return nil
	}

	pv.mu.RLock()
	defer pv.mu.RUnlock()

	if len(urlPath) > MaxURLLength || len(queryBytes) > MaxURLLength {
		return nil
	}

	urlPathStr := string(urlPath)
	masks := pv.createParamMasks(urlPathStr)

	return pv.filterQueryParamsToBuffer(queryBytes, masks, urlPathStr, buffer, true)
}

// filterQueryParamsToBuffer filters into provided buffer (fully []byte)
func (pv *ParamValidator) filterQueryParamsToBuffer(queryBytes []byte, masks ParamMasks, urlPath string, buffer []byte, useBytes bool) []byte {
	if cap(buffer) < len(queryBytes) {
		return nil
	}

	result := buffer[:0]
	firstParam := true
	start := 0

	for i := 0; i <= len(queryBytes); i++ {
		if i == len(queryBytes) || queryBytes[i] == '&' {
			if start < i {
				var allowed bool
				if useBytes {
					allowed = pv.isParamAllowedBytesSegment(queryBytes[start:i], masks, urlPath)
				} else {
					allowed = pv.isParamAllowedSegment(string(queryBytes[start:i]), masks, urlPath)
				}

				if allowed {
					if !firstParam {
						result = append(result, '&')
					} else {
						firstParam = false
					}
					result = append(result, queryBytes[start:i]...)
				}
			}
			start = i + 1
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// ValidateQueryBytes validates query parameters bytes for URL path
// Zero-allocs version for high-performance scenarios
func (pv *ParamValidator) ValidateQueryBytes(urlPath, queryBytes []byte) bool {
	if !pv.initialized.Load() || len(urlPath) == 0 {
		return false
	}

	return pv.withSafeAccess(func() bool {
		if len(urlPath) > MaxURLLength || len(queryBytes) > MaxURLLength {
			return false
		}

		if len(queryBytes) == 0 {
			return true
		}

		// Convert urlPath to string once (this allocation is necessary for URL matching)
		urlPathStr := string(urlPath)
		masks := pv.createParamMasks(urlPathStr)

		if pv.isAllowAllParamsMasks(masks) {
			return true
		}

		if masks.CombinedMask().IsEmpty() {
			return false
		}

		// Use bytes version without converting queryBytes to string
		return pv.validateQueryParamsBytes(queryBytes, masks, urlPathStr)
	})
}

// validateQueryParamsBytes validates query parameters in []byte form without allocations
func (pv *ParamValidator) validateQueryParamsBytes(queryBytes []byte, masks ParamMasks, urlPath string) bool {
	if len(queryBytes) == 0 {
		return true
	}

	allowAll := pv.isAllowAllParamsMasks(masks)
	start := 0
	paramCount := 0

	for i := 0; i <= len(queryBytes); i++ {
		if i == len(queryBytes) || queryBytes[i] == '&' {
			if start < i && paramCount < MaxParamValues {
				if !allowAll {
					if !pv.isParamAllowedBytesSegment(queryBytes[start:i], masks, urlPath) {
						return false
					}
				}
				paramCount++
			}
			start = i + 1
		}
	}
	return paramCount <= MaxParamValues
}

// createParamMasks creates parameter masks for URL path
func (pv *ParamValidator) createParamMasks(urlPath string) ParamMasks {
	masks := ParamMasks{
		Global:      NewParamMask(),
		URL:         NewParamMask(),
		SpecificURL: NewParamMask(),
	}
	pv.fillParamMasksDirect(&masks, urlPath)
	return masks
}

// isParamAllowedBytesSegment checks segment in []byte form
func (pv *ParamValidator) isParamAllowedBytesSegment(segment []byte, masks ParamMasks, urlPath string) bool {
	eqPos := -1
	for i := 0; i < len(segment); i++ {
		if segment[i] == '=' {
			eqPos = i
			break
		}
	}

	var keyBytes, valueBytes []byte
	if eqPos == -1 {
		keyBytes = segment
		valueBytes = nil
	} else {
		keyBytes = segment[:eqPos]
		valueBytes = segment[eqPos+1:]
	}

	idx := pv.compiledRules.paramIndex.GetIndexByBytes(keyBytes)
	if idx == -1 {
		return false
	}

	if !masks.CombinedMask().GetBit(idx) {
		return false
	}

	rule := pv.findParamRuleByIndex(idx, masks, urlPath)
	if rule == nil {
		return false
	}

	return pv.isValueValidBytesFast(rule, valueBytes)
}

func (pv *ParamValidator) isValueValidBytesFast(rule *ParamRule, valueBytes []byte) bool {
	if rule == nil {
		return false
	}

	var result bool

	switch rule.Pattern {
	case PatternKeyOnly:
		result = len(valueBytes) == 0
	case PatternAny:
		result = true
	case PatternEnum:
		result = false
		for _, allowedValue := range rule.Values {
			if bytesEqual(valueBytes, []byte(allowedValue)) {
				result = true
				break
			}
		}
	case PatternCallback:
		if pv.callbackFunc != nil {
			valueStr := string(valueBytes)
			result = pv.callbackFunc(rule.Name, valueStr)
		} else {
			result = false
		}
	case "plugin":
		if rule.CustomValidator != nil {
			valueStr := string(valueBytes)
			result = rule.CustomValidator(valueStr)
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

// FilterURL optimized version
func (pv *ParamValidator) FilterURL(fullURL string) string {
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

	var buf [1024]byte
	result := buf[:0]
	firstParam := true

	start := 0
	for i := 0; i <= len(queryString); i++ {
		if i == len(queryString) || queryString[i] == '&' {
			if start < i {
				segment := queryString[start:i]
				if pv.isParamAllowedSegment(segment, masks, urlPath) {
					if !firstParam {
						result = append(result, '&')
					} else {
						firstParam = false
					}
					result = append(result, segment...)
				}
			}
			start = i + 1
		}
	}

	return string(result)
}

func (pv *ParamValidator) isParamAllowedSegment(segment string, masks ParamMasks, urlPath string) bool {
	eqPos, _, _, _, _ := pv.parseQuerySegment(segment, 0, len(segment))

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

// FilterQuery filters query parameters string according to validation rules
func (pv *ParamValidator) FilterQuery(urlPath, queryString string) string {
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

// ValidateQuery validates query parameters string for URL path
func (pv *ParamValidator) ValidateQuery(urlPath, queryString string) bool {
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

		masks := pv.createParamMasks(urlPath)

		if pv.isAllowAllParamsMasks(masks) {
			return true
		}

		if masks.CombinedMask().IsEmpty() {
			return false
		}

		return pv.validateQueryParams(queryString, masks, urlPath, false)
	})
}

// fillParamMasksDirect fills masks directly without extra allocations
func (pv *ParamValidator) fillParamMasksDirect(masks *ParamMasks, urlPath string) {
	if pv.compiledRules == nil || pv.compiledRules.paramIndex == nil {
		return
	}

	// Global parameters - use pre-calculated mask
	masks.Global = pv.compiledRules.globalParamsMask

	// Most specific rule
	if mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath); mostSpecificRule != nil {
		masks.SpecificURL = mostSpecificRule.ParamMask
	}

	// URL rules
	for pattern, urlRule := range pv.compiledRules.urlRules {
		if pv.urlMatchesPatternUnsafe(urlPath, pattern) {
			filteredMask := urlRule.ParamMask.Difference(masks.SpecificURL)
			masks.URL = masks.URL.Union(filteredMask)
		}
	}
}

// Clear removes all validation rules
func (pv *ParamValidator) ClearRules() {
	pv.mu.Lock()
	defer pv.mu.Unlock()
	pv.clearUnsafe()
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
	pv.rules = rulesStr
	pv.compileRulesUnsafe()
	return nil
}

func (pv *ParamValidator) RulesString() (string, error) {
	if !pv.initialized.Load() {
		return "", fmt.Errorf("validator not initialized")
	}
	pv.mu.Lock()
	defer pv.mu.Unlock()
	result := pv.rules
	return result, nil
}

// compileRulesUnsafe compiles rules for faster access with masks
func (pv *ParamValidator) compileRulesUnsafe() {
	if pv.paramIndex == nil {
		pv.paramIndex = NewParamIndex()
	} else {
		pv.paramIndex.Clear()
	}

	pv.compiledRules = &CompiledRules{
		globalParams:        make(map[string]*ParamRule),
		urlRules:            make(map[string]*URLRule),
		paramIndex:          pv.paramIndex,
		globalParamsByIndex: make(map[int]*ParamRule),
		urlRulesByIndex:     make(map[int][]*URLRule),
	}

	// Copy global parameters and index them
	for name, rule := range pv.globalParams {
		ruleCopy := pv.copyParamRuleUnsafe(rule)
		idx := pv.paramIndex.GetOrCreateIndex(name)
		if idx != -1 {
			ruleCopy.BitmaskIndex = idx
			pv.compiledRules.globalParams[name] = ruleCopy
			pv.compiledRules.globalParamsByIndex[idx] = ruleCopy
		}
	}

	// Copy URL rules and create bit masks for them
	for pattern, rule := range pv.urlRules {
		ruleCopy := &URLRule{
			URLPattern:    rule.URLPattern,
			Params:        make(map[string]*ParamRule),
			ParamMask:     NewParamMask(),
			paramsByIndex: make(map[int]*ParamRule),
		}

		for paramName, paramRule := range rule.Params {
			paramRuleCopy := pv.copyParamRuleUnsafe(paramRule)
			idx := pv.paramIndex.GetOrCreateIndex(paramName)
			if idx != -1 {
				paramRuleCopy.BitmaskIndex = idx
				ruleCopy.Params[paramName] = paramRuleCopy
				ruleCopy.ParamMask.SetBit(idx)
				ruleCopy.paramsByIndex[idx] = paramRuleCopy

				pv.compiledRules.urlRulesByIndex[idx] = append(pv.compiledRules.urlRulesByIndex[idx], ruleCopy)
			}
		}

		pv.compiledRules.urlRules[pattern] = ruleCopy
	}

	// Pre-calculate global parameters mask
	globalMask := NewParamMask()
	for name := range pv.compiledRules.globalParams {
		if idx := pv.paramIndex.GetIndex(name); idx != -1 {
			globalMask.SetBit(idx)
		}
	}
	pv.compiledRules.globalParamsMask = globalMask

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
		pv.urlMatcher.AddRule(pattern, rule)
	}
}

// isAllowAllParamsMasks checks if masks allow all parameters
func (pv *ParamValidator) isAllowAllParamsMasks(masks ParamMasks) bool {
	idx := pv.compiledRules.paramIndex.GetIndex(PatternAll)
	return idx != -1 && masks.CombinedMask().GetBit(idx)
}

// CheckRules quickly checks validity of rules string
func (pv *ParamValidator) CheckRules(rulesStr string) error {
	if !pv.initialized.Load() {
		return fmt.Errorf("validator not initialized")
	}

	return pv.parser.CheckRulesSyntax(rulesStr)
}

// CheckRulesStatic static rules validation
func CheckRulesStatic(rulesStr string) error {
	if rulesStr == "" {
		return nil
	}

	parser := NewRuleParser()
	defer parser.Close()

	return parser.CheckRulesSyntax(rulesStr)
}

// CheckRulesStaticWithPlugins static validation with plugins
func CheckRulesStaticWithPlugins(rulesStr string, plugins []PluginConstraintParser) error {
	if rulesStr == "" {
		return nil
	}

	parser := NewRuleParser(plugins...)
	defer parser.Close()

	return parser.CheckRulesSyntax(rulesStr)
}
