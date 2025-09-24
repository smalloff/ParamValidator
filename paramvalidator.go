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

	// Security limits
	MaxRulesSize       = 64 * 1024 // 64KB
	MaxURLLength       = 4096      // 4KB
	MaxPatternLength   = 200
	MaxParamNameLength = 100
	MaxParamValues     = 100
)

type ParamRule struct {
	Name    string
	Pattern string
	Values  []string
	Min     int64
	Max     int64
}

type URLRule struct {
	URLPattern string
	Params     map[string]*ParamRule
}

type ParamValidator struct {
	mu           sync.RWMutex
	globalParams map[string]*ParamRule
	urlRules     map[string]*URLRule
	rulesStr     string
	initialized  bool
}

func NewParamValidator(rulesStr string) *ParamValidator {
	pv := &ParamValidator{
		globalParams: make(map[string]*ParamRule),
		urlRules:     make(map[string]*URLRule),
		initialized:  true,
	}

	if rulesStr != "" {
		if err := pv.ParseRules(rulesStr); err != nil {
			fmt.Printf("Warning: Failed to parse initial rules: %v\n", err)
		}
	}

	return pv
}

func (pv *ParamValidator) validateInputSize(input string, maxSize int) error {
	if len(input) > maxSize {
		return fmt.Errorf("input size %d exceeds maximum allowed size %d", len(input), maxSize)
	}
	if len(input) == 0 {
		return fmt.Errorf("input cannot be empty")
	}

	if len(input) > 10*1024*1024 { // 10MB
		return fmt.Errorf("input size exceeds absolute maximum")
	}

	return nil
}

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

func (pv *ParamValidator) isValidParamName(name string) bool {
	// Разрешаем только буквы, цифры, дефисы и подчеркивания
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

func (pv *ParamValidator) ParseRules(rulesStr string) error {
	if !pv.initialized {
		return fmt.Errorf("validator not initialized")
	}

	if err := pv.validateInputSize(rulesStr, MaxRulesSize); err != nil {
		return err
	}

	pv.mu.Lock()
	defer pv.mu.Unlock()

	return pv.parseRulesUnsafe(rulesStr)
}

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

func (pv *ParamValidator) isRangeConstraint(constraintStr string) bool {
	return strings.Contains(constraintStr, "-") && !strings.Contains(constraintStr, ",")
}

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

func (pv *ParamValidator) validateURLUnsafe(fullURL string) bool {
	u, err := url.Parse(fullURL)
	if err != nil {
		return false
	}

	paramsRules := pv.getParamsForURLUnsafe(u.Path)

	if pv.isAllowAllParams(paramsRules) {
		return true
	}

	if len(paramsRules) == 0 && len(u.Query()) > 0 {
		return false
	}

	if len(u.Query()) > 0 && len(paramsRules) == 0 {
		return false
	}

	return pv.validateQueryParamsUnsafe(u.Query(), paramsRules)
}

func (pv *ParamValidator) isAllowAllParams(paramsRules map[string]*ParamRule) bool {
	return paramsRules != nil && paramsRules[PatternAll] != nil
}

func (pv *ParamValidator) validateQueryParamsUnsafe(queryParams url.Values, urlParams map[string]*ParamRule) bool {
	for paramName, values := range queryParams {
		rule := pv.findParamRule(paramName, urlParams)
		if rule == nil {
			return false
		}

		if !pv.validateParamValues(rule, values) {
			return false
		}
	}

	return true
}

func (pv *ParamValidator) findParamRule(paramName string, urlParams map[string]*ParamRule) *ParamRule {
	if rule, exists := urlParams[paramName]; exists {
		return rule
	}

	if rule, exists := pv.globalParams[paramName]; exists {
		return rule
	}

	return nil
}

func (pv *ParamValidator) validateParamValues(rule *ParamRule, values []string) bool {
	for _, value := range values {
		if !pv.isValueValidUnsafe(rule, value) {
			return false
		}
	}
	return true
}

func (pv *ParamValidator) getParamsForURLUnsafe(urlPath string) map[string]*ParamRule {
	urlPath = pv.normalizeURLPattern(urlPath)

	mostSpecificRule := pv.findMostSpecificURLRuleUnsafe(urlPath)

	result := make(map[string]*ParamRule)

	for name, rule := range pv.globalParams {
		result[name] = rule
	}

	for pattern, rule := range pv.urlRules {
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

func (pv *ParamValidator) findMostSpecificURLRuleUnsafe(urlPath string) *URLRule {
	var mostSpecificRule *URLRule
	maxSpecificity := -1

	for pattern, rule := range pv.urlRules {
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

func (pv *ParamValidator) calculateSpecificityUnsafe(pattern string) int {
	if pattern == PatternAll {
		return 0
	}

	specificity := 0

	if !strings.Contains(pattern, "*") {
		specificity += 1000
	}

	pathParts := strings.Split(strings.Trim(pattern, "/"), "/")
	if len(pathParts) > 100 {
		specificity += 100 * 100
	} else {
		specificity += len(pathParts) * 100
	}

	if !strings.Contains(pattern, "*") {
		specificity += 500
	}

	wildcardCount := strings.Count(pattern, "*")
	specificity -= wildcardCount * 200

	if strings.HasSuffix(pattern, "*") {
		specificity -= 300
	}

	if strings.Contains(pattern, "*") && !strings.HasSuffix(pattern, "*") {
		specificity -= 100
	}

	if strings.Count(pattern, "/") > 1 {
		specificity += strings.Count(pattern, "/") * 50
	}

	return specificity
}

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

func (pv *ParamValidator) validateParamUnsafe(urlPath, paramName, paramValue string) bool {
	paramsRules := pv.getParamsForURLUnsafe(urlPath)
	rule := pv.findParamRule(paramName, paramsRules)

	if rule == nil {
		return false
	}

	return pv.isValueValidUnsafe(rule, paramValue)
}

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

func (pv *ParamValidator) filterParamValues(rule *ParamRule, values []string) []string {
	var allowed []string
	for _, value := range values {
		if pv.isValueValidUnsafe(rule, value) {
			allowed = append(allowed, value)
		}
	}
	return allowed
}

func (pv *ParamValidator) normalizeURLUnsafe(fullURL string) string {
	u, err := url.Parse(fullURL)
	if err != nil {
		return fullURL
	}

	paramsRules := pv.getParamsForURLUnsafe(u.Path)

	if pv.isAllowAllParams(paramsRules) {
		return fullURL
	}

	if len(paramsRules) == 0 && len(pv.globalParams) == 0 {
		return u.Path
	}

	filteredParams := pv.filterQueryParamsUnsafeValues(u.Query(), paramsRules)

	if len(filteredParams) > 0 {
		u.RawQuery = filteredParams.Encode()
		return u.String()
	}

	return u.Path
}

func (pv *ParamValidator) filterQueryParamsUnsafe(urlPath, queryString string) string {
	paramsRules := pv.getParamsForURLUnsafe(urlPath)

	if pv.isAllowAllParams(paramsRules) {
		return queryString
	}

	if len(paramsRules) == 0 && len(pv.globalParams) == 0 {
		return ""
	}

	params, err := url.ParseQuery(queryString)
	if err != nil {
		return ""
	}

	return pv.filterQueryParamsValuesUnsafe(params, paramsRules)
}

func (pv *ParamValidator) filterQueryParamsValuesUnsafe(params url.Values, urlParams map[string]*ParamRule) string {
	var filtered []string

	for paramName, values := range params {
		rule := pv.findParamRule(paramName, urlParams)
		if rule == nil {
			continue
		}

		for _, value := range values {
			if pv.isValueValidUnsafe(rule, value) {
				filtered = append(filtered, url.QueryEscape(paramName)+"="+url.QueryEscape(value))
			}
		}
	}

	if len(filtered) == 0 {
		return ""
	}

	return strings.Join(filtered, "&")
}

func (pv *ParamValidator) filterQueryParamsUnsafeValues(queryParams url.Values, paramsRules map[string]*ParamRule) url.Values {
	filtered := url.Values{}

	for paramName, values := range queryParams {
		rule := pv.findParamRule(paramName, paramsRules)
		if rule == nil {
			continue
		}

		allowedValues := pv.filterParamValues(rule, values)
		if len(allowedValues) > 0 {
			filtered[paramName] = allowedValues
		}
	}

	return filtered
}

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

func (pv *ParamValidator) clearUnsafe() {
	pv.globalParams = make(map[string]*ParamRule)
	pv.urlRules = make(map[string]*URLRule)
	pv.rulesStr = ""
}

func (pv *ParamValidator) Clear() {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	pv.clearUnsafe()
}

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
	pv.updateRulesStringUnsafe()
}

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
		pv.rulesStr = strings.Join(rules, "&") + "; " + strings.Join(urlRulesList, "; ")
	} else if len(rules) > 0 {
		pv.rulesStr = strings.Join(rules, "&")
	} else if len(urlRulesList) > 0 {
		pv.rulesStr = strings.Join(urlRulesList, "; ")
	} else {
		pv.rulesStr = ""
	}
}

func (pv *ParamValidator) paramRuleToStringUnsafe(rule *ParamRule) string {
	if rule == nil {
		return ""
	}

	if rule.Name == "*" && rule.Pattern == "any" {
		return "*"
	}

	switch rule.Pattern {
	case "key-only":
		return rule.Name + "=[]"
	case "any":
		return rule.Name + "=[*]"
	case "range":
		return fmt.Sprintf("%s=[%d-%d]", rule.Name, rule.Min, rule.Max)
	case "enum":
		return rule.Name + "=[" + strings.Join(rule.Values, ",") + "]"
	default:
		return rule.Name
	}
}

func (pv *ParamValidator) AddGlobalParam(rule *ParamRule) {
	if rule == nil || rule.Name == "" {
		return
	}

	pv.mu.Lock()
	defer pv.mu.Unlock()

	pv.globalParams[rule.Name] = pv.copyParamRuleUnsafe(rule)
	pv.updateRulesStringUnsafe()
}
