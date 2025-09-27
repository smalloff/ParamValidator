package paramvalidator

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

// WithCallback устанавливает callback-функцию для валидации
func WithCallback(callback CallbackFunc) Option {
	return func(pv *ParamValidator) {
		pv.callbackFunc = callback
	}
}

// WithPlugins регистрирует плагины для парсера правил
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

func NewParamValidator(rulesStr string, options ...Option) (*ParamValidator, error) {
	pv := &ParamValidator{
		globalParams: make(map[string]*ParamRule),
		urlRules:     make(map[string]*URLRule),
		urlMatcher:   NewURLMatcher(),
		paramIndex:   NewParamIndex(),
		initialized:  true,
		parser:       NewRuleParser(),
	}

	// Применяем опции
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

// checkSize проверяет размер входных данных
func (pv *ParamValidator) checkSize(input string, maxSize int, inputType string) error {
	if len(input) > maxSize {
		return fmt.Errorf("%s size %d exceeds maximum allowed size %d", inputType, len(input), maxSize)
	}
	return nil
}

// withSafeAccess выполняет функцию с блокировкой и проверкой инициализации
func (pv *ParamValidator) withSafeAccess(fn func() bool) bool {
	if pv == nil || !pv.initialized {
		return false
	}

	pv.mu.RLock()
	defer pv.mu.RUnlock()
	return fn()
}

// processURL обрабатывает URL с безопасным доступом
func (pv *ParamValidator) processURL(fullURL string, processor func(*url.URL) bool) bool {
	return pv.withSafeAccess(func() bool {
		if err := pv.checkSize(fullURL, MaxURLLength, "URL"); err != nil {
			return false
		}

		u, err := url.Parse(fullURL)
		if err != nil {
			return false
		}

		return processor(u)
	})
}

// ValidateURL validates complete URL against loaded rules
func (pv *ParamValidator) ValidateURL(fullURL string) bool {
	return pv.processURL(fullURL, func(u *url.URL) bool {
		return pv.validateURLUnsafe(u)
	})
}

// validateURLUnsafe validates URL without locking using masks
func (pv *ParamValidator) validateURLUnsafe(u *url.URL) bool {
	masks := pv.getParamMasksForURL(u.Path)

	// Проверяем разрешение всех параметров
	if pv.isAllowAllParamsMasks(masks) {
		return true
	}

	// Если нет правил для параметров, но есть query string - невалидно
	if masks.CombinedMask().IsEmpty() && u.RawQuery != "" {
		return false
	}

	// Если нет query string - валидно
	if u.RawQuery == "" {
		return true
	}

	valid, err := pv.parseAndValidateQueryParamsWithMasks(u.RawQuery, masks, u.Path)
	return err == nil && valid
}

// isAllowAllParamsMasks checks if masks allow all parameters
func (pv *ParamValidator) isAllowAllParamsMasks(masks ParamMasks) bool {
	if pv.compiledRules == nil || pv.compiledRules.paramIndex == nil {
		return false
	}

	// Проверяем наличие параметра "*" в комбинированной маске
	idx := pv.compiledRules.paramIndex.GetIndex(PatternAll)
	return idx != -1 && masks.CombinedMask().GetBit(idx)
}

// getParamMasksForURL возвращает маски параметров для URL
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

	// 1. Глобальные параметры
	masks.Global = pv.compiledRules.paramIndex.CreateMaskForParams(pv.compiledRules.globalParams)

	// 2. Находим самое специфичное правило
	mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath)

	// 3. Для URL правил: добавляем только те параметры, которые НЕ переопределены в самом специфичном правиле
	for pattern, urlRule := range pv.compiledRules.urlRules {
		if pv.urlMatchesPatternUnsafe(urlPath, pattern) {
			// Для каждого параметра в этом правиле проверяем, есть ли он в самом специфичном правиле
			for paramName := range urlRule.Params {
				idx := pv.compiledRules.paramIndex.GetIndex(paramName)
				if idx == -1 {
					continue
				}

				// Если этот параметр уже есть в самом специфичном правиле - пропускаем
				if mostSpecificRule != nil {
					if _, exists := mostSpecificRule.Params[paramName]; exists {
						continue
					}
				}

				// Добавляем параметр в URL маску
				masks.URL.SetBit(idx)
			}
		}
	}

	// 4. Добавляем параметры из самого специфичного правила
	if mostSpecificRule != nil {
		masks.SpecificURL = pv.compiledRules.paramIndex.CreateMaskForParams(mostSpecificRule.Params)
	}

	return masks
}

// addNonOverriddenParams добавляет параметры, не переопределенные в специфичном правиле
func (pv *ParamValidator) addNonOverriddenParams(masks ParamMasks, urlRule *URLRule, mostSpecificRule *URLRule) {
	for paramName := range urlRule.Params {
		idx := pv.compiledRules.paramIndex.GetIndex(paramName)
		if idx == -1 {
			continue
		}

		// Если параметр уже есть в самом специфичном правиле - пропускаем
		if _, exists := mostSpecificRule.Params[paramName]; exists {
			continue
		}

		masks.URL.SetBit(idx)
	}
}

// findParamRuleByMasks находит правило с учетом приоритетов через маски
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

	// ВАЖНО: SpecificURL имеет абсолютный приоритет
	if source == SourceSpecificURL {
		if mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath); mostSpecificRule != nil {
			return mostSpecificRule.Params[paramName]
		}
		return nil
	}

	// Для URL правил: ищем первое подходящее правило
	if source == SourceURL {
		return pv.findURLRuleForParam(paramName, urlPath)
	}

	// Глобальные правила
	return pv.compiledRules.globalParams[paramName]
}

// findURLRuleForParam находит URL правило для параметра
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

// isParamAllowedWithMasks проверяет параметр используя систему масок
func (pv *ParamValidator) isParamAllowedWithMasks(paramName, paramValue string, masks ParamMasks, urlPath string) bool {
	rule := pv.findParamRuleByMasks(paramName, masks, urlPath)
	return rule != nil && pv.isValueValidUnsafe(rule, paramValue)
}

// parseAndValidateQueryParamsWithMasks парсит и валидирует параметры с использованием масок
func (pv *ParamValidator) parseAndValidateQueryParamsWithMasks(queryString string, masks ParamMasks, urlPath string) (bool, error) {
	if queryString == "" {
		return true, nil
	}

	isValid := true
	allowAll := pv.isAllowAllParamsMasks(masks)

	err := pv.processQueryParamsCommon(queryString, func(key, value, originalKey, originalValue string) {
		if !allowAll && !pv.isParamAllowedWithMasks(key, value, masks, urlPath) {
			isValid = false
		}
	})

	return isValid, err
}

var builderPool = sync.Pool{
	New: func() interface{} {
		b := &strings.Builder{}
		b.Grow(256)
		return b
	},
}

// parseAndFilterQueryParamsWithMasks фильтрует параметры с использованием масок
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

	err := pv.processQueryParamsCommon(queryString, func(key, value, originalKey, originalValue string) {
		if allowAll || pv.isParamAllowedWithMasks(key, value, masks, urlPath) {
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

// NormalizeURL filters and normalizes URL according to validation rules
func (pv *ParamValidator) NormalizeURL(fullURL string) string {
	if pv == nil || !pv.initialized || fullURL == "" {
		return fullURL
	}

	var result string
	pv.processURL(fullURL, func(u *url.URL) bool {
		result = pv.normalizeURLUnsafe(u)
		return true
	})

	return result
}

// normalizeURLUnsafe normalizes URL using masks
func (pv *ParamValidator) normalizeURLUnsafe(u *url.URL) string {
	masks := pv.getParamMasksForURL(u.Path)

	if pv.isAllowAllParamsMasks(masks) {
		return u.String()
	}

	if masks.CombinedMask().IsEmpty() {
		if u.RawQuery != "" {
			return u.Path
		}
		return u.String()
	}

	if u.RawQuery == "" {
		return u.String()
	}

	filteredParams, _, err := pv.parseAndFilterQueryParamsWithMasks(u.RawQuery, masks, u.Path)
	if err != nil {
		return u.Path
	}

	if filteredParams != "" {
		u.RawQuery = filteredParams
		return u.String()
	}

	return u.Path
}

// FilterQueryParams filters query parameters string according to validation rules
func (pv *ParamValidator) FilterQueryParams(urlPath, queryString string) string {
	if !pv.initialized || queryString == "" {
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
	if !pv.initialized || urlPath == "" {
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

// processQueryParamsCommon обрабатывает параметры query string
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

// parseParamSegment parses a single parameter segment
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
		Min:             rule.Min,
		Max:             rule.Max,
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
	if !pv.initialized {
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
	// Очищаем индекс перед компиляцией новых правил
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

	// Копируем глобальные параметры и индексируем их
	for name, rule := range pv.globalParams {
		ruleCopy := pv.copyParamRuleUnsafe(rule)
		idx := pv.paramIndex.GetOrCreateIndex(name)
		if idx != -1 {
			ruleCopy.BitmaskIndex = idx
			pv.compiledRules.globalParams[name] = ruleCopy
		}
	}

	// Копируем URL-правила и создаем для них битовые маски
	for pattern, rule := range pv.urlRules {
		ruleCopy := &URLRule{
			URLPattern: rule.URLPattern,
			Params:     make(map[string]*ParamRule),
			ParamMask:  NewParamMask(),
		}

		// Копируем параметры и обновляем индексы
		for paramName, paramRule := range rule.Params {
			paramRuleCopy := pv.copyParamRuleUnsafe(paramRule)
			idx := pv.paramIndex.GetOrCreateIndex(paramName)
			if idx != -1 {
				paramRuleCopy.BitmaskIndex = idx
				ruleCopy.Params[paramName] = paramRuleCopy
				// Устанавливаем бит в маске URL правила
				ruleCopy.ParamMask.SetBit(idx)
			}
		}

		pv.compiledRules.urlRules[pattern] = ruleCopy
	}

	// Обновляем URLMatcher
	pv.updateURLMatcherUnsafe()
}

// updateURLMatcherUnsafe обновляет URLMatcher с текущими правилами
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

// Close освобождает ресурсы валидатора
func (pv *ParamValidator) Close() error {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	if !pv.initialized {
		return nil
	}

	if pv.parser != nil {
		if err := pv.parser.Close(); err != nil {
			return fmt.Errorf("failed to close parser: %w", err)
		}
	}

	pv.clearUnsafe()
	pv.initialized = false
	return nil
}

// Reset сбрасывает правила
func (pv *ParamValidator) Reset() {
	pv.mu.Lock()
	defer pv.mu.Unlock()
	pv.clearUnsafe()
	pv.initialized = true
	pv.callbackFunc = nil
}
