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
// validateQueryParamsFast полностью на стеке
func (pv *ParamValidator) validateQueryParamsFast(queryString string, masks ParamMasks, urlPath string) bool {
	if queryString == "" {
		return true
	}

	allowAll := pv.isAllowAllParamsMasks(masks)
	start := 0
	paramCount := 0

	for i := 0; i <= len(queryString); i++ {
		if i == len(queryString) || queryString[i] == '&' {
			if start < i && paramCount < MaxParamValues {
				// Ищем '=' в текущем сегменте
				eqPos := -1
				for j := start; j < i; j++ {
					if queryString[j] == '=' {
						eqPos = j
						break
					}
				}

				if !allowAll {
					var keyStart, keyEnd, valStart, valEnd int
					keyStart = start

					if eqPos == -1 {
						keyEnd = i
						valStart = i
						valEnd = i
					} else {
						keyEnd = eqPos
						valStart = eqPos + 1
						valEnd = i
					}

					if !pv.isParamAllowedByIndexes(queryString, keyStart, keyEnd, valStart, valEnd, masks, urlPath) {
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

// isParamAllowedByIndexes валидирует по индексам без создания подстрок для ключа
func (pv *ParamValidator) isParamAllowedByIndexes(fullStr string, keyStart, keyEnd, valStart, valEnd int, masks ParamMasks, urlPath string) bool {
	// Ищем параметр по диапазону байт без создания строки
	idx := pv.compiledRules.paramIndex.GetIndexByRange(fullStr, keyStart, keyEnd)
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

	// Для значения все равно создаем строку (1 аллокация вместо 2)
	value := ""
	if valStart < valEnd {
		value = fullStr[valStart:valEnd]
	}

	return pv.isValueValidFast(rule, value)
}

// GetIndexByRange ищет параметр по диапазону байтов без создания строки
func (pi *ParamIndex) GetIndexByRange(str string, start, end int) int {
	length := end - start
	if length <= 0 {
		return -1
	}

	var result int = -1
	pi.paramToIndex.Range(func(key, value interface{}) bool {
		paramName := key.(string)
		if len(paramName) == length {
			// Быстрое сравнение по диапазону
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

	// Быстрая проверка по маскам
	if !masks.CombinedMask().GetBit(idx) {
		return false
	}

	// Находим правило используя предрасчитанные данные
	rule := pv.findParamRuleByIndex(idx, masks, urlPath)
	return rule != nil && pv.isValueValidFast(rule, paramValue)
}

// findParamRuleByIndex находит правило по индексу без поиска по имени
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

// findGlobalParamByIndex находит глобальный параметр по индексу (только чтение)
func (pv *ParamValidator) findGlobalParamByIndex(paramIndex int) *ParamRule {
	return pv.compiledRules.globalParamsByIndex[paramIndex] // только чтение
}

// findURLRuleForParamByIndex находит URL правило по индексу параметра
func (pv *ParamValidator) findURLRuleForParamByIndex(paramIndex int, urlPath string) *ParamRule {
	rules := pv.compiledRules.urlRulesByIndex[paramIndex]
	if len(rules) == 0 {
		return nil
	}

	// Находим самое специфичное правило для данного URL
	var mostSpecificRule *URLRule
	for _, rule := range rules {
		if pv.urlMatchesPatternUnsafe(urlPath, rule.URLPattern) {
			if mostSpecificRule == nil || isPatternMoreSpecific(rule.URLPattern, mostSpecificRule.URLPattern) {
				mostSpecificRule = rule
			}
		}
	}

	if mostSpecificRule != nil {
		return mostSpecificRule.paramsByIndex[paramIndex] // только чтение
	}
	return nil
}

// findParamInURLRuleByIndex находит параметр в URL правиле по индексу
func (pv *ParamValidator) findParamInURLRuleByIndex(urlRule *URLRule, paramIndex int) *ParamRule {
	if urlRule.paramsByIndex == nil {
		pv.buildURLRuleParamsByIndex(urlRule)
	}
	return urlRule.paramsByIndex[paramIndex]
}

// buildURLRuleParamsByIndex строит кэш параметров в URL правиле по индексу
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
		// Оптимизация: проверяем напрямую без создания итератора
		for i := 0; i < len(rule.Values); i++ {
			if value == rule.Values[i] {
				result = true
				break
			}
		}
	case PatternCallback:
		if pv.callbackFunc != nil {
			result = pv.callbackFunc(rule.Name, value)
		} else {
			result = false
		}
	case "plugin":
		if rule.CustomValidator != nil {
			result = rule.CustomValidator(value)
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

// isValueValid universal value validation function with panic protection
func (pv *ParamValidator) isValueValid(rule *ParamRule, value string, useFast bool) bool {
	if rule == nil {
		return false
	}

	if useFast {
		return pv.isValueValidFast(rule, value)
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
			result = pv.safeCallback(rule.Name, value)
		} else {
			result = false
		}
	case "plugin":
		if rule.CustomValidator != nil {
			result = pv.safeCustomValidator(rule.CustomValidator, value)
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
func (pv *ParamValidator) safeCallback(paramName, value string) bool {
	defer func() {
		if r := recover(); r != nil {
			// Panic recovered, result remains false
		}
	}()
	return pv.callbackFunc(paramName, value)
}

// safeCustomValidator executes custom validator with panic protection
func (pv *ParamValidator) safeCustomValidator(validator func(string) bool, value string) bool {
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

	// 3. URL rules - ВКЛЮЧАЕМ все параметры, но при валидации будем проверять приоритеты
	for pattern, urlRule := range pv.compiledRules.urlRules {
		if pv.urlMatchesPatternUnsafe(urlPath, pattern) {
			for name := range urlRule.Params {
				// НЕ исключаем параметры из specific rule - они будут обработаны в findParamRuleByMasks
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

	// Проверяем в порядке приоритета: SpecificURL -> URL -> Global
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

	// Дополнительная проверка: если параметр есть в specific rule,
	// но текущее значение не валидно по specific rule, не fallback'аем на URL rules
	if masks.SpecificURL.GetBit(pv.compiledRules.paramIndex.GetIndex(paramName)) {
		if mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath); mostSpecificRule != nil {
			if specificRule, exists := mostSpecificRule.Params[paramName]; exists {
				// Проверяем валидность по самому специфичному правилу
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

	// Конвертируем urlPath в string для поиска масок (это делается 1 раз)
	urlPathStr := string(urlPath)

	// Создаем маски на стеке
	var masks ParamMasks
	masks.Global = NewParamMask()
	masks.URL = NewParamMask()
	masks.SpecificURL = NewParamMask()
	pv.fillParamMasksDirect(&masks, urlPathStr)

	return pv.filterQueryParamsToBufferBytes(queryBytes, masks, urlPathStr, buffer)
}

// filterQueryParamsToBufferBytes фильтрация в предоставленный буфер (полностью []byte)
func (pv *ParamValidator) filterQueryParamsToBufferBytes(queryBytes []byte, masks ParamMasks, urlPath string, buffer []byte) []byte {
	if cap(buffer) < len(queryBytes) {
		return nil
	}

	result := buffer[:0]
	firstParam := true
	start := 0

	for i := 0; i <= len(queryBytes); i++ {
		if i == len(queryBytes) || queryBytes[i] == '&' {
			if start < i {
				if pv.isParamAllowedBytesSegment(queryBytes[start:i], masks, urlPath) {
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

		// Конвертируем urlPath в string (1 раз)
		urlPathStr := string(urlPath)

		// Создаем маски на стеке
		var masks ParamMasks
		masks.Global = NewParamMask()
		masks.URL = NewParamMask()
		masks.SpecificURL = NewParamMask()
		pv.fillParamMasksDirect(&masks, urlPathStr)

		if pv.isAllowAllParamsMasks(masks) {
			return true
		}

		if masks.CombinedMask().IsEmpty() {
			return false
		}

		return pv.validateQueryParamsBytesFast(queryBytes, masks, urlPathStr)
	})
}

// validateQueryParamsBytesFast быстрая валидация []byte параметров
func (pv *ParamValidator) validateQueryParamsBytesFast(queryBytes []byte, masks ParamMasks, urlPath string) bool {
	allowAll := pv.isAllowAllParamsMasks(masks)
	start := 0
	paramCount := 0

	for i := 0; i <= len(queryBytes); i++ {
		if i == len(queryBytes) || queryBytes[i] == '&' {
			if start < i && paramCount < MaxParamValues {
				segment := queryBytes[start:i]
				if !allowAll {
					if !pv.isParamAllowedBytesSegment(segment, masks, urlPath) {
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

// isParamAllowedBytesSegment проверка сегмента в []byte форме
func (pv *ParamValidator) isParamAllowedBytesSegment(segment []byte, masks ParamMasks, urlPath string) bool {
	// Ищем '=' в сегменте
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

	// Ищем параметр по []byte
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

	// Для значения используем быструю проверку по []byte
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
		// Сравниваем []byte напрямую с каждым значением enum
		result = false
		for _, allowedValue := range rule.Values {
			if bytesEqual(valueBytes, []byte(allowedValue)) {
				result = true
				break
			}
		}
	case PatternCallback:
		if pv.callbackFunc != nil {
			// Для callback нужна конвертация в string
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
// filterQueryParamsFast на стеке
func (pv *ParamValidator) filterQueryParamsFast(queryString string, masks ParamMasks, urlPath string) string {
	if queryString == "" {
		return ""
	}

	// Используем stack-allocated буфер
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
	eqPos := -1
	for i := 0; i < len(segment); i++ {
		if segment[i] == '=' {
			eqPos = i
			break
		}
	}

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

		// Создаем маски на стеке
		var masks ParamMasks
		masks.Global = NewParamMask()
		masks.URL = NewParamMask()
		masks.SpecificURL = NewParamMask()
		pv.fillParamMasksDirect(&masks, urlPath)

		if pv.isAllowAllParamsMasks(masks) {
			return true
		}

		if masks.CombinedMask().IsEmpty() {
			return false
		}

		return pv.validateQueryParamsFast(queryString, masks, urlPath)
	})
}

// fillParamMasksDirect заполняет маски напрямую без лишних аллокаций
func (pv *ParamValidator) fillParamMasksDirect(masks *ParamMasks, urlPath string) {
	if pv.compiledRules == nil || pv.compiledRules.paramIndex == nil {
		return
	}

	// 1. Global parameters - используем предрасчитанную маску
	masks.Global = pv.compiledRules.globalParamsMask

	// 2. Most specific rule
	if mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath); mostSpecificRule != nil {
		masks.SpecificURL = mostSpecificRule.ParamMask
	}

	// 3. URL rules
	for pattern, urlRule := range pv.compiledRules.urlRules {
		if pv.urlMatchesPatternUnsafe(urlPath, pattern) {
			// Исключаем параметры, которые уже в SpecificURL
			filteredMask := urlRule.ParamMask.Difference(masks.SpecificURL)
			masks.URL = masks.URL.Union(filteredMask)
		}
	}
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
// compileRulesUnsafe строит все кэши заранее
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
		globalParamsByIndex: make(map[int]*ParamRule), // сразу создаем
		urlRulesByIndex:     make(map[int][]*URLRule), // сразу создаем
	}

	// Копируем глобальные параметры и индексируем их
	for name, rule := range pv.globalParams {
		ruleCopy := pv.copyParamRuleUnsafe(rule)
		idx := pv.paramIndex.GetOrCreateIndex(name)
		if idx != -1 {
			ruleCopy.BitmaskIndex = idx
			pv.compiledRules.globalParams[name] = ruleCopy
			pv.compiledRules.globalParamsByIndex[idx] = ruleCopy // сразу заполняем кэш
		}
	}

	// Копируем URL правила и создаем битовые маски для них
	for pattern, rule := range pv.urlRules {
		ruleCopy := &URLRule{
			URLPattern:    rule.URLPattern,
			Params:        make(map[string]*ParamRule),
			ParamMask:     NewParamMask(),
			paramsByIndex: make(map[int]*ParamRule), // сразу создаем
		}

		for paramName, paramRule := range rule.Params {
			paramRuleCopy := pv.copyParamRuleUnsafe(paramRule)
			idx := pv.paramIndex.GetOrCreateIndex(paramName)
			if idx != -1 {
				paramRuleCopy.BitmaskIndex = idx
				ruleCopy.Params[paramName] = paramRuleCopy
				ruleCopy.ParamMask.SetBit(idx)
				ruleCopy.paramsByIndex[idx] = paramRuleCopy // сразу заполняем кэш

				// Добавляем в urlRulesByIndex
				pv.compiledRules.urlRulesByIndex[idx] = append(pv.compiledRules.urlRulesByIndex[idx], ruleCopy)
			}
		}

		pv.compiledRules.urlRules[pattern] = ruleCopy
	}

	// Предрасчитываем маску глобальных параметров
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
