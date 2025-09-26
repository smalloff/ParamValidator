package paramvalidator

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// NewParamValidator creates a new parameter validator with optional initial rules
// rulesStr: String containing validation rules in specific format
// Returns initialized ParamValidator instance or error if parsing fails
func NewParamValidator(rulesStr string, callback ...CallbackFunc) (*ParamValidator, error) {
	pv := &ParamValidator{
		globalParams:  make(map[string]*ParamRule),
		urlRules:      make(map[string]*URLRule),
		urlMatcher:    NewURLMatcher(),
		compiledRules: &CompiledRules{},
		initialized:   true,
		parser:        NewRuleParser(),
	}

	if len(callback) > 0 && callback[0] != nil {
		pv.callbackFunc = callback[0]
	}

	if rulesStr != "" {
		if err := pv.ParseRules(rulesStr); err != nil {
			fmt.Printf("Warning: Failed to parse initial rules: %v\n", err)
			return nil, err
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

// validateInputSize checks if input size exceeds allowed limits
func (pv *ParamValidator) validateInputSize(input string, maxSize int) error {
	if len(input) > maxSize {
		return fmt.Errorf("input size %d exceeds maximum allowed size %d", len(input), maxSize)
	}

	if len(input) > 10*1024*1024 {
		return fmt.Errorf("input size exceeds absolute maximum")
	}

	return nil
}

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
	urlPath = NormalizeURLPattern(urlPath)
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
	return pv.urlMatcher.GetMostSpecificRule(urlPath)
}

// urlMatchesPatternUnsafe checks if URL path matches pattern
func (pv *ParamValidator) urlMatchesPatternUnsafe(urlPath, pattern string) bool {
	return urlMatchesPattern(urlPath, pattern)
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
	case "plugin":
		if rule.CustomValidator != nil {
			return rule.CustomValidator(value)
		}
		return false
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

	err := pv.processQueryParamsCommon(queryString, func(key, value, originalKey, originalValue string) {
		if !allowAll && !pv.isParamAllowedUnsafe(key, value, paramsRules) {
			isValid = false
		}
	})

	return isValid, err
}

// parseAndFilterQueryParams parses and filters query parameters
func (pv *ParamValidator) parseAndFilterQueryParams(queryString string, paramsRules map[string]*ParamRule) (string, bool, error) {
	if queryString == "" {
		return "", true, nil
	}

	var filteredParams strings.Builder
	isValid := true
	allowAll := pv.isAllowAllParams(paramsRules)
	firstParam := true

	err := pv.processQueryParamsCommon(queryString, func(key, value, originalKey, originalValue string) {
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

func (pv *ParamValidator) processQueryParamsCommon(queryString string, processor func(key, value, originalKey, originalValue string)) error {
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

// Clear removes all validation rules
func (pv *ParamValidator) Clear() {
	pv.mu.Lock()
	defer pv.mu.Unlock()
	pv.clearUnsafe()
}

// copyParamRuleUnsafe creates a deep copy of ParamRule
func (pv *ParamValidator) copyParamRuleUnsafe(rule *ParamRule) *ParamRule {
	if rule == nil {
		return nil
	}

	ruleCopy := &ParamRule{
		Name:    rule.Name,
		Pattern: rule.Pattern,
		Min:     rule.Min,
		Max:     rule.Max,
	}

	if rule.Values != nil {
		ruleCopy.Values = make([]string, len(rule.Values))
		copy(ruleCopy.Values, rule.Values)
	}

	return ruleCopy
}

// clearUnsafe resets all rules without locking
func (pv *ParamValidator) clearUnsafe() {
	pv.globalParams = make(map[string]*ParamRule)
	pv.urlRules = make(map[string]*URLRule)
	pv.compiledRules = &CompiledRules{
		globalParams: make(map[string]*ParamRule),
		urlRules:     make(map[string]*URLRule),
	}
	pv.urlMatcher.ClearRules() // Добавляем очистку matcher
}

// ClearRules clears all loaded validation rules
func (pv *ParamValidator) ClearRules() {
	pv.mu.Lock()
	defer pv.mu.Unlock()
	pv.clearUnsafe()
}

// ParseRules parses and loads validation rules from string
// rulesStr: String containing validation rules in specific format
// Returns error if parsing fails
func (pv *ParamValidator) ParseRules(rulesStr string) error {
	if !pv.initialized {
		return fmt.Errorf("validator not initialized")
	}

	if rulesStr == "" {
		pv.mu.Lock()
		defer pv.mu.Unlock()
		pv.clearUnsafe()
		return nil
	}

	if err := pv.validateInputSize(rulesStr, MaxRulesSize); err != nil {
		return err
	}

	pv.mu.Lock()
	defer pv.mu.Unlock()

	globalParams, urlRules, err := pv.parser.parseRulesUnsafe(rulesStr)
	if err != nil {
		return err
	}

	pv.globalParams = globalParams
	pv.urlRules = urlRules
	pv.compileRulesUnsafe()
	return nil
}

// compileRulesUnsafe compiles rules for faster access
func (pv *ParamValidator) compileRulesUnsafe() {
	pv.compiledRules = &CompiledRules{
		globalParams: make(map[string]*ParamRule),
		urlRules:     make(map[string]*URLRule),
	}

	// Копируем глобальные параметры
	for name, rule := range pv.globalParams {
		pv.compiledRules.globalParams[name] = pv.copyParamRuleUnsafe(rule)
	}

	// Копируем URL-правила (используем актуальные pv.urlRules)
	for pattern, rule := range pv.urlRules {
		pv.compiledRules.urlRules[pattern] = pv.copyURLRuleUnsafe(rule)
	}

	// Также обновляем URLMatcher с новыми правилами
	pv.updateURLMatcherUnsafe()
}

// updateURLMatcherUnsafe обновляет URLMatcher с текущими правилами
func (pv *ParamValidator) updateURLMatcherUnsafe() {
	pv.urlMatcher.ClearRules()
	for pattern, rule := range pv.urlRules {
		pv.urlMatcher.AddRule(pattern, pv.copyURLRuleUnsafe(rule))
	}
}

// copyURLRuleUnsafe creates a deep copy of URLRule
func (pv *ParamValidator) copyURLRuleUnsafe(rule *URLRule) *URLRule {
	if rule == nil {
		return nil
	}

	ruleCopy := &URLRule{
		URLPattern: rule.URLPattern,
		Params:     make(map[string]*ParamRule),
	}

	for paramName, paramRule := range rule.Params {
		ruleCopy.Params[paramName] = pv.copyParamRuleUnsafe(paramRule)
	}

	return ruleCopy
}

// Close освобождает ресурсы валидатора (включая плагины)
func (pv *ParamValidator) Close() error {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	if !pv.initialized {
		return nil
	}

	// Освобождаем ресурсы парсера (которые освободят ресурсы плагинов)
	if pv.parser != nil {
		if err := pv.parser.Close(); err != nil {
			return fmt.Errorf("failed to close parser: %w", err)
		}
	}

	// Дополнительная очистка ресурсов валидатора
	pv.clearUnsafe()
	pv.initialized = false

	return nil
}

// Reset сбрасывает правила и освобождает ресурсы, но оставляет валидатор работоспособным
func (pv *ParamValidator) Reset() {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	pv.clearUnsafe()

	// Сбрасываем состояние, но оставляем initialized = true
	pv.initialized = true
	pv.callbackFunc = nil
}
