package paramvalidator

import (
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	PatternRange   = "range"
	PatternEnum    = "enum"
	PatternKeyOnly = "key-only"
	PatternAny     = "any"
	PatternAll     = "*"

	MaxRulesSize       = 64 * 1024
	MaxURLLength       = 4096
	MaxPatternLength   = 200
	MaxParamNameLength = 100
	MaxParamValues     = 100
)

type QueryParam struct {
	Key   string
	Value string
}

type ParamRule struct {
	Values  []string
	Name    string
	Pattern string
	Min     int64
	Max     int64
}

type URLRule struct {
	Params     map[string]*ParamRule
	URLPattern string
}

type CompiledRules struct {
	globalParams map[string]*ParamRule
	urlRules     map[string]*URLRule
}

type ParamValidator struct {
	mu            sync.RWMutex
	globalParams  map[string]*ParamRule
	urlRules      map[string]*URLRule
	rulesStr      string
	initialized   bool
	compiledRules *CompiledRules
}

// NewParamValidator creates a new parameter validator with optional initial rules
// rulesStr: String containing validation rules in specific format
// Returns initialized ParamValidator instance
func NewParamValidator(rulesStr string) *ParamValidator {
	pv := &ParamValidator{
		globalParams:  make(map[string]*ParamRule),
		urlRules:      make(map[string]*URLRule),
		compiledRules: &CompiledRules{},
		initialized:   true,
	}

	if rulesStr != "" {
		if err := pv.ParseRules(rulesStr); err != nil {
			fmt.Printf("Warning: Failed to parse initial rules: %v\n", err)
		}
	}

	return pv
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

// sanitizeParamName validates and cleans parameter name
func (pv *ParamValidator) sanitizeParamName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("parameter name cannot be empty")
	}
	if len(name) > MaxParamNameLength {
		return "", fmt.Errorf("parameter name too long: %d characters", len(name))
	}

	if !pv.isValidParamName(name) {
		return "", fmt.Errorf("invalid characters in parameter name: %s", name)
	}

	return name, nil
}

// isValidParamName checks if parameter name contains only allowed characters
func (pv *ParamValidator) isValidParamName(name string) bool {
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return false
		}
	}
	return true
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

	if err := pv.parseRulesUnsafe(rulesStr); err != nil {
		return err
	}

	pv.compileRulesUnsafe()
	return nil
}

// compileRulesUnsafe compiles rules for faster access
func (pv *ParamValidator) compileRulesUnsafe() {
	pv.compiledRules = &CompiledRules{
		globalParams: make(map[string]*ParamRule),
		urlRules:     make(map[string]*URLRule),
	}

	for name, rule := range pv.globalParams {
		pv.compiledRules.globalParams[name] = pv.copyParamRuleUnsafe(rule)
	}

	for pattern, rule := range pv.urlRules {
		pv.compiledRules.urlRules[pattern] = pv.copyURLRuleUnsafe(rule)
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

// parseRulesUnsafe parses rules string without locking
func (pv *ParamValidator) parseRulesUnsafe(rulesStr string) error {
	if rulesStr == "" {
		pv.clearUnsafe()
		return nil
	}

	pv.clearUnsafe()

	ruleType := pv.detectRuleType(rulesStr)

	switch ruleType {
	case RuleTypeURL:
		return pv.parseURLRulesUnsafe(rulesStr)
	case RuleTypeGlobal:
		return pv.parseGlobalParamsUnsafe(rulesStr)
	default:
		return fmt.Errorf("unknown rule type")
	}
}

type RuleType int

const (
	RuleTypeUnknown RuleType = iota
	RuleTypeGlobal
	RuleTypeURL
)

// detectRuleType determines the type of rules in the string
func (pv *ParamValidator) detectRuleType(rulesStr string) RuleType {
	cleanRulesStr := strings.ReplaceAll(rulesStr, " ", "")

	if strings.Contains(cleanRulesStr, "/?") ||
		strings.Contains(cleanRulesStr, "*?") ||
		(strings.Contains(cleanRulesStr, "/") && strings.Contains(cleanRulesStr, "?")) {
		return RuleTypeURL
	}

	if strings.Contains(cleanRulesStr, "/") &&
		!strings.Contains(cleanRulesStr, "=") &&
		(strings.Contains(cleanRulesStr, "[") || strings.Contains(cleanRulesStr, "]")) {
		return RuleTypeURL
	}

	return RuleTypeGlobal
}

// parseGlobalParamsUnsafe parses global parameter rules
func (pv *ParamValidator) parseGlobalParamsUnsafe(rulesStr string) error {
	rules := pv.splitRules(rulesStr, '&')

	for _, ruleStr := range rules {
		if ruleStr == "" {
			continue
		}

		rule, err := pv.parseSingleParamRuleUnsafe(ruleStr)
		if err != nil {
			return err
		}
		if rule != nil {
			pv.globalParams[rule.Name] = rule
		}
	}

	pv.rulesStr = rulesStr
	return nil
}

// parseURLRulesUnsafe parses URL-specific rules
func (pv *ParamValidator) parseURLRulesUnsafe(rulesStr string) error {
	urlRuleStrings := pv.splitURLRules(rulesStr)

	for _, urlRuleStr := range urlRuleStrings {
		if urlRuleStr == "" {
			continue
		}

		urlPattern, paramsStr := pv.extractURLAndParams(urlRuleStr)

		if urlPattern == "" && paramsStr != "" {
			if err := pv.parseGlobalParamsUnsafe(paramsStr); err != nil {
				return fmt.Errorf("failed to parse global params: %w", err)
			}
			continue
		}

		urlPattern = pv.normalizeURLPattern(urlPattern)
		if urlPattern == "" {
			continue
		}

		params, err := pv.parseParamsStringUnsafe(paramsStr)
		if err != nil {
			return fmt.Errorf("failed to parse params for URL %s: %w", urlPattern, err)
		}

		if urlPattern != "" {
			urlRule := &URLRule{
				URLPattern: urlPattern,
				Params:     params,
			}
			pv.urlRules[urlPattern] = urlRule
		}
	}

	pv.rulesStr = rulesStr
	return nil
}

// splitRules splits rules string considering bracket nesting
func (pv *ParamValidator) splitRules(rulesStr string, separator byte) []string {
	var result []string
	var current strings.Builder
	bracketDepth := 0

	for i := 0; i < len(rulesStr); i++ {
		char := rulesStr[i]

		switch char {
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		}

		if char == separator && bracketDepth == 0 {
			if current.Len() > 0 {
				ruleStr := strings.TrimSpace(current.String())
				if ruleStr != "" {
					result = append(result, ruleStr)
				}
				current.Reset()
			}
			continue
		}

		current.WriteByte(char)
	}

	if current.Len() > 0 {
		ruleStr := strings.TrimSpace(current.String())
		if ruleStr != "" {
			result = append(result, ruleStr)
		}
	}

	return result
}

// splitURLRules splits URL rules string by semicolon or returns single rule
func (pv *ParamValidator) splitURLRules(rulesStr string) []string {
	var builder strings.Builder
	builder.Grow(len(rulesStr))

	for _, r := range rulesStr {
		if r != ' ' && r != '\n' {
			builder.WriteRune(r)
		}
	}
	cleanRulesStr := builder.String()

	if strings.Contains(cleanRulesStr, ";") {
		return pv.splitRules(rulesStr, ';')
	}

	return []string{rulesStr}
}

// extractURLAndParams separates URL pattern from parameters string
func (pv *ParamValidator) extractURLAndParams(urlRuleStr string) (string, string) {
	cleanStr := strings.ReplaceAll(urlRuleStr, " ", "")

	if strings.HasPrefix(cleanStr, "/") || strings.HasPrefix(cleanStr, "*") {
		bracketDepth := 0
		for i := 0; i < len(cleanStr); i++ {
			switch cleanStr[i] {
			case '[':
				bracketDepth++
			case ']':
				if bracketDepth > 0 {
					bracketDepth--
				}
			case '?':
				if bracketDepth == 0 {
					urlPattern := strings.TrimSpace(urlRuleStr[:i])
					paramsStr := strings.TrimSpace(urlRuleStr[i+1:])
					return urlPattern, paramsStr
				}
			}
		}

		if strings.Contains(cleanStr, "[") {
			parts := strings.SplitN(cleanStr, "[", 2)
			if len(parts) == 2 {
				urlPattern := strings.TrimSpace(parts[0])
				urlPattern = strings.TrimSuffix(urlPattern, "?")
				paramsStr := "[" + parts[1]
				return urlPattern, paramsStr
			}
		}

		return strings.TrimSpace(urlRuleStr), ""
	}

	return "", urlRuleStr
}

// normalizeURLPattern cleans and standardizes URL pattern
func (pv *ParamValidator) normalizeURLPattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return ""
	}

	if strings.Contains(pattern, "*") {
		return pattern
	}

	if !strings.HasPrefix(pattern, "/") {
		pattern = "/" + pattern
	}

	cleaned := path.Clean(pattern)
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

// parseParamsStringUnsafe parses parameters string into map of rules
func (pv *ParamValidator) parseParamsStringUnsafe(paramsStr string) (map[string]*ParamRule, error) {
	params := make(map[string]*ParamRule)

	if paramsStr == PatternAll {
		params[PatternAll] = &ParamRule{
			Name:    PatternAll,
			Pattern: PatternAny,
		}
		return params, nil
	}

	if paramsStr == "" {
		return params, nil
	}

	paramStrings := pv.splitRules(paramsStr, '&')

	for _, paramStr := range paramStrings {
		rule, err := pv.parseSingleParamRuleUnsafe(paramStr)
		if err != nil {
			return nil, err
		}
		if rule != nil {
			params[rule.Name] = rule
		}
	}

	return params, nil
}

// parseSingleParamRuleUnsafe parses single parameter rule
func (pv *ParamValidator) parseSingleParamRuleUnsafe(ruleStr string) (*ParamRule, error) {
	ruleStr = strings.TrimSpace(ruleStr)
	if ruleStr == "" {
		return nil, nil
	}

	if strings.HasSuffix(ruleStr, "=[]") {
		paramName := strings.TrimSuffix(ruleStr, "=[]")
		paramName, err := pv.sanitizeParamName(paramName)
		if err != nil {
			return nil, fmt.Errorf("invalid parameter name in key-only rule: %w", err)
		}
		return &ParamRule{
			Name:    paramName,
			Pattern: PatternKeyOnly,
		}, nil
	}

	startBracket := strings.Index(ruleStr, "[")
	if startBracket == -1 {
		return pv.parseSimpleParamRule(ruleStr)
	}

	return pv.parseComplexParamRule(ruleStr, startBracket)
}

// parseSimpleParamRule parses simple parameter rule without brackets
func (pv *ParamValidator) parseSimpleParamRule(ruleStr string) (*ParamRule, error) {
	if strings.Contains(ruleStr, "=") {
		paramName := strings.Split(ruleStr, "=")[0]
		paramName, err := pv.sanitizeParamName(paramName)
		if err != nil {
			return nil, fmt.Errorf("invalid parameter name in rule: %w", err)
		}
		return &ParamRule{
			Name:    paramName,
			Pattern: PatternKeyOnly,
		}, nil
	}

	if ruleStr == "" {
		return nil, fmt.Errorf("empty rule")
	}

	paramName, err := pv.sanitizeParamName(ruleStr)
	if err != nil {
		return nil, fmt.Errorf("invalid parameter name: %w", err)
	}

	return &ParamRule{
		Name:    paramName,
		Pattern: PatternAny,
	}, nil
}

// parseComplexParamRule parses parameter rule with bracket constraints
func (pv *ParamValidator) parseComplexParamRule(ruleStr string, startBracket int) (*ParamRule, error) {
	paramName := strings.TrimSpace(ruleStr[:startBracket])
	if strings.HasSuffix(paramName, "=") {
		paramName = strings.TrimSuffix(paramName, "=")
		paramName = strings.TrimSpace(paramName)
	}

	paramName, err := pv.sanitizeParamName(paramName)
	if err != nil {
		return nil, fmt.Errorf("invalid parameter name in rule: %w", err)
	}

	constraintStr, endBracket := pv.extractConstraint(ruleStr, startBracket)
	if endBracket == -1 {
		return nil, fmt.Errorf("unclosed bracket in rule: %s", ruleStr)
	}

	if constraintStr == "" {
		return &ParamRule{
			Name:    paramName,
			Pattern: PatternAny,
		}, nil
	}

	return pv.createParamRule(paramName, constraintStr)
}

// extractConstraint extracts content between brackets
func (pv *ParamValidator) extractConstraint(ruleStr string, startBracket int) (string, int) {
	bracketDepth := 1
	endBracket := -1

	for i := startBracket + 1; i < len(ruleStr); i++ {
		if ruleStr[i] == '[' {
			bracketDepth++
		} else if ruleStr[i] == ']' {
			bracketDepth--
			if bracketDepth == 0 {
				endBracket = i
				break
			}
		}
	}

	if endBracket == -1 {
		return "", -1
	}

	return strings.TrimSpace(ruleStr[startBracket+1 : endBracket]), endBracket
}

// createParamRule creates ParamRule from name and constraint
func (pv *ParamValidator) createParamRule(paramName, constraintStr string) (*ParamRule, error) {
	rule := &ParamRule{Name: paramName}

	switch {
	case constraintStr == "":
		rule.Pattern = PatternKeyOnly
	case constraintStr == PatternAll:
		rule.Pattern = PatternAny
	case pv.isRangeConstraint(constraintStr):
		if err := pv.parseRangeConstraint(rule, constraintStr); err != nil {
			return nil, err
		}
	case strings.Contains(constraintStr, ","):
		if err := pv.parseEnumConstraint(rule, constraintStr); err != nil {
			return nil, err
		}
	default:
		rule.Pattern = PatternEnum
		rule.Values = []string{constraintStr}
	}

	return rule, nil
}

// isRangeConstraint checks if constraint string represents a range
func (pv *ParamValidator) isRangeConstraint(constraintStr string) bool {
	return strings.Contains(constraintStr, "-") && !strings.Contains(constraintStr, ",")
}

// parseRangeConstraint parses range constraint into rule
func (pv *ParamValidator) parseRangeConstraint(rule *ParamRule, constraintStr string) error {
	parts := strings.Split(constraintStr, "-")
	if len(parts) != 2 {
		return fmt.Errorf("invalid range format: %s", constraintStr)
	}

	minStr := strings.TrimSpace(parts[0])
	maxStr := strings.TrimSpace(parts[1])

	if len(minStr) > 10 || len(maxStr) > 10 {
		return fmt.Errorf("range values too long")
	}

	min, err := strconv.ParseInt(minStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid min value in range: %s", minStr)
	}

	max, err := strconv.ParseInt(maxStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid max value in range: %s", maxStr)
	}

	if min < -1000000000 || max > 1000000000 {
		return fmt.Errorf("range values out of safe bounds")
	}

	if min > max {
		return fmt.Errorf("min value greater than max in range: %d-%d", min, max)
	}

	rule.Pattern = PatternRange
	rule.Min = min
	rule.Max = max
	return nil
}

// parseEnumConstraint parses enum constraint into rule
func (pv *ParamValidator) parseEnumConstraint(rule *ParamRule, constraintStr string) error {
	values := strings.Split(constraintStr, ",")
	if len(values) > MaxParamValues {
		return fmt.Errorf("too many enum values: %d, maximum is %d", len(values), MaxParamValues)
	}

	rule.Pattern = PatternEnum
	rule.Values = make([]string, 0, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			rule.Values = append(rule.Values, value)
		}
	}

	if len(rule.Values) == 0 {
		return fmt.Errorf("empty enum constraint")
	}

	return nil
}

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

	_, valid, err := pv.parseAndFilterQueryParams(u.RawQuery, paramsRules)
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

	// Быстрый подсчет метрик за один проход
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

	// Расчет базовой специфичности на основе сегментов пути
	pathSegmentCount := slashCount
	if len(pattern) > 0 && pattern[0] != '/' {
		pathSegmentCount++
	}

	// Базовый вес = количество сегментов * 100
	specificity := pathSegmentCount * 100

	if !hasWildcard {
		// Бонус для точного совпадения
		specificity += 1500
	} else {
		// Штраф за wildcard'ы
		specificity -= wildcardCount * 200

		// Дополнительные штрафы
		if lastCharIsWildcard {
			specificity -= 300
		}
		if hasMiddleWildcard {
			specificity -= 100
		}
	}

	// Бонус для глубоких путей
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
			// Параметр без значения
			originalKey = segment
			decodedKey, err := url.QueryUnescape(segment)
			if err != nil {
				isValid = false
				continue // Пропускаем некорректно закодированные параметры
			}
			key = decodedKey
			value = ""
			originalValue = ""
		} else {
			// Параметр со значением
			originalKey = segment[:eqPos]
			originalValue = segment[eqPos+1:]

			decodedKey, err1 := url.QueryUnescape(originalKey)
			decodedValue, err2 := url.QueryUnescape(originalValue)

			if err1 != nil || err2 != nil {
				isValid = false
				continue // Пропускаем некорректно закодированные параметры
			}
			key = decodedKey
			value = decodedValue
		}

		// Если разрешены все параметры или параметр разрешен правилами
		if allowAll || pv.isParamAllowedUnsafe(key, value, paramsRules) {
			// Сохраняем оригинальную строку параметра
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

	fullURL := urlPath
	if queryString != "" {
		fullURL = urlPath + "?" + queryString
	}

	return pv.ValidateURL(fullURL)
}

// clearUnsafe clears all rules without locking
func (pv *ParamValidator) clearUnsafe() {
	pv.globalParams = make(map[string]*ParamRule)
	pv.urlRules = make(map[string]*URLRule)
	pv.rulesStr = ""
	pv.compiledRules = &CompiledRules{
		globalParams: make(map[string]*ParamRule),
		urlRules:     make(map[string]*URLRule),
	}
}

// Clear removes all validation rules
func (pv *ParamValidator) Clear() {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	pv.clearUnsafe()
}

// copyParamRuleUnsafe creates deep copy of ParamRule
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

// AddURLRule adds URL-specific validation rule
// urlPattern: URL pattern to match (supports wildcards)
// params: Map of parameter rules for this URL pattern
func (pv *ParamValidator) AddURLRule(urlPattern string, params map[string]*ParamRule) {
	if urlPattern == "" || len(params) == 0 {
		return
	}

	if err := pv.validateInputSize(urlPattern, MaxPatternLength); err != nil {
		return
	}

	pv.mu.Lock()
	defer pv.mu.Unlock()

	if !strings.HasPrefix(urlPattern, "/") {
		urlPattern = "/" + urlPattern
	}

	paramsCopy := make(map[string]*ParamRule)
	for k, v := range params {
		paramsCopy[k] = pv.copyParamRuleUnsafe(v)
	}

	pv.urlRules[urlPattern] = &URLRule{
		URLPattern: urlPattern,
		Params:     paramsCopy,
	}
	pv.compileRulesUnsafe()
	pv.updateRulesStringUnsafe()
}

// updateRulesStringUnsafe updates internal rules string representation
func (pv *ParamValidator) updateRulesStringUnsafe() {
	var rules []string

	if len(pv.globalParams) > 0 {
		var globalRules []string
		for _, rule := range pv.globalParams {
			globalRules = append(globalRules, pv.paramRuleToStringUnsafe(rule))
		}
		sort.Strings(globalRules)
		rules = append(rules, strings.Join(globalRules, "&"))
	}

	var urlRulesList []string
	for _, urlRule := range pv.urlRules {
		var paramRules []string
		for _, paramRule := range urlRule.Params {
			paramRules = append(paramRules, pv.paramRuleToStringUnsafe(paramRule))
		}
		sort.Strings(paramRules)
		urlRuleStr := urlRule.URLPattern + "/?" + strings.Join(paramRules, "&")
		urlRulesList = append(urlRulesList, urlRuleStr)
	}
	sort.Strings(urlRulesList)

	if len(rules) > 0 && len(urlRulesList) > 0 {
		pv.rulesStr = strings.Join(rules, ";") + ";" + strings.Join(urlRulesList, ";")
	} else if len(rules) > 0 {
		pv.rulesStr = strings.Join(rules, ";")
	} else if len(urlRulesList) > 0 {
		pv.rulesStr = strings.Join(urlRulesList, ";")
	} else {
		pv.rulesStr = ""
	}
}

// paramRuleToStringUnsafe converts ParamRule to string representation
func (pv *ParamValidator) paramRuleToStringUnsafe(rule *ParamRule) string {
	if rule == nil {
		return ""
	}

	switch rule.Pattern {
	case PatternKeyOnly:
		return rule.Name + "=[]"
	case PatternAny:
		return rule.Name
	case PatternRange:
		return fmt.Sprintf("%s=[%d-%d]", rule.Name, rule.Min, rule.Max)
	case PatternEnum:
		if len(rule.Values) == 1 {
			return fmt.Sprintf("%s=[%s]", rule.Name, rule.Values[0])
		}
		return fmt.Sprintf("%s=[%s]", rule.Name, strings.Join(rule.Values, ","))
	default:
		return rule.Name
	}
}

// GetRules returns current validation rules as string
func (pv *ParamValidator) GetRules() string {
	pv.mu.RLock()
	defer pv.mu.RUnlock()

	return pv.rulesStr
}

// GetURLRules returns all URL-specific rules
func (pv *ParamValidator) GetURLRules() map[string]*URLRule {
	pv.mu.RLock()
	defer pv.mu.RUnlock()

	rules := make(map[string]*URLRule)
	for k, v := range pv.urlRules {
		rules[k] = pv.copyURLRuleUnsafe(v)
	}
	return rules
}

// GetGlobalParams returns all global parameter rules
func (pv *ParamValidator) GetGlobalParams() map[string]*ParamRule {
	pv.mu.RLock()
	defer pv.mu.RUnlock()

	params := make(map[string]*ParamRule)
	for k, v := range pv.globalParams {
		params[k] = pv.copyParamRuleUnsafe(v)
	}
	return params
}

// IsInitialized checks if validator is properly initialized
func (pv *ParamValidator) IsInitialized() bool {
	return pv != nil && pv.initialized
}
