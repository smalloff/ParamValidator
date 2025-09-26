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
		if err := pv.validateInputSize(rulesStr, MaxRulesSize); err != nil {
			return nil, fmt.Errorf("rules string too large: %w", err)
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

// validateInputSize checks if input size exceeds allowed limits
func (pv *ParamValidator) validateInputSize(input string, maxSize int) error {
	if len(input) > maxSize {
		return fmt.Errorf("input size %d exceeds maximum allowed size %d", len(input), maxSize)
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

// validateURLUnsafe validates URL without locking using masks
func (pv *ParamValidator) validateURLUnsafe(fullURL string) bool {
	u, err := url.Parse(fullURL)
	if err != nil {
		return false
	}

	masks := pv.getParamMasksForURL(u.Path)
	combinedMask := masks.CombinedMask()

	// Проверяем разрешение всех параметров
	if pv.isAllowAllParamsMasks(masks) {
		return true
	}

	// Если нет правил для параметров, но есть query string - невалидно
	if combinedMask.IsEmpty() && u.RawQuery != "" {
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
			for paramName, _ := range urlRule.Params {
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

	// Для URL правил: ищем первое подходящее правило, где есть этот параметр
	if source == SourceURL {
		// Сначала пытаемся найти наиболее специфичное правило (даже если оно не самое специфичное в целом)
		var mostSpecificURLRule *URLRule
		for pattern, urlRule := range pv.compiledRules.urlRules {
			if pv.urlMatchesPatternUnsafe(urlPath, pattern) {
				if mostSpecificURLRule == nil || isPatternMoreSpecific(pattern, mostSpecificURLRule.URLPattern) {
					if _, exists := urlRule.Params[paramName]; exists {
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

	// Глобальные правила
	if rule, exists := pv.compiledRules.globalParams[paramName]; exists {
		return rule
	}

	return nil
}

func isPatternMoreSpecific(pattern1, pattern2 string) bool {
	// Эвристика: паттерн с меньшим количеством wildcards считается более специфичным
	wildcards1 := strings.Count(pattern1, "*")
	wildcards2 := strings.Count(pattern2, "*")

	if wildcards1 != wildcards2 {
		return wildcards1 < wildcards2
	}

	// Если одинаковое количество wildcards, более длинный путь считается более специфичным
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
		result := value == ""

		return result

	case PatternAny:

		return true

	case PatternEnum:
		result := pv.validateEnumValue(rule, value)

		return result

	case PatternCallback:
		result := pv.validateCallbackValue(rule, value)

		return result

	case "plugin":
		if rule.CustomValidator != nil {
			result := rule.CustomValidator(value)

			return result
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

	if err := pv.validateInputSize(fullURL, MaxURLLength); err != nil {
		return fullURL
	}

	pv.mu.RLock()
	defer pv.mu.RUnlock()

	return pv.normalizeURLUnsafe(fullURL)
}

// normalizeURLUnsafe normalizes URL using masks
func (pv *ParamValidator) normalizeURLUnsafe(fullURL string) string {
	u, err := url.Parse(fullURL)
	if err != nil {
		return fullURL
	}

	masks := pv.getParamMasksForURL(u.Path)
	combinedMask := masks.CombinedMask()

	if pv.isAllowAllParamsMasks(masks) {
		return fullURL
	}

	if combinedMask.IsEmpty() {
		if u.RawQuery != "" {
			return u.Path
		}
		return fullURL
	}

	if u.RawQuery == "" {
		return u.Path
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

	if err := pv.validateInputSize(urlPath, MaxURLLength); err != nil {
		return ""
	}

	pv.mu.RLock()
	defer pv.mu.RUnlock()

	return pv.filterQueryParamsUnsafe(urlPath, queryString)
}

// filterQueryParamsUnsafe filters query parameters using masks
func (pv *ParamValidator) filterQueryParamsUnsafe(urlPath, queryString string) string {
	masks := pv.getParamMasksForURL(urlPath)
	combinedMask := masks.CombinedMask()

	if pv.isAllowAllParamsMasks(masks) {
		return queryString
	}

	if combinedMask.IsEmpty() {
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

	masks := pv.getParamMasksForURL(urlPath)
	combinedMask := masks.CombinedMask()

	if pv.isAllowAllParamsMasks(masks) {
		return true
	}

	if combinedMask.IsEmpty() {
		return false
	}

	valid, err := pv.parseAndValidateQueryParamsWithMasks(queryString, masks, urlPath)
	return err == nil && valid
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
