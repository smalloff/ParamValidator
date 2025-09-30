package paramvalidator

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// PluginConstraintParser defines interface for custom constraint parsers
type PluginConstraintParser interface {
	Parse(paramName, constraintStr string) (func(string) bool, error)
	GetName() string
}

// PluginResourceManager defines interface for plugin resource management
type PluginResourceManager interface {
	Close() error
}

// RuleParser handles parsing of validation rules with plugin support
type RuleParser struct {
	plugins []PluginConstraintParser
	cache   *ValidationCache
}

// NewRuleParser creates a new rule parser with optional plugins
func NewRuleParser(plugins ...PluginConstraintParser) *RuleParser {
	return &RuleParser{
		plugins: plugins,
		cache:   NewValidationCache(),
	}
}

// RegisterPlugin adds a new plugin to the parser
func (rp *RuleParser) RegisterPlugin(plugin PluginConstraintParser) {
	rp.plugins = append(rp.plugins, plugin)
}

// Close releases all parser resources including plugins
func (rp *RuleParser) Close() error {
	if rp.cache != nil {
		rp.cache.Clear()
	}

	var errors []error
	for _, plugin := range rp.plugins {
		if resourcePlugin, ok := plugin.(PluginResourceManager); ok {
			if err := resourcePlugin.Close(); err != nil {
				errors = append(errors, fmt.Errorf("plugin %s: %w", plugin.GetName(), err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing plugins: %v", errors)
	}
	return nil
}

// validateRulesString performs common validation on rules string
func (rp *RuleParser) validateRulesString(rulesStr string) error {
	if len(rulesStr) > MaxRulesSize {
		return fmt.Errorf("rules size %d exceeds maximum %d", len(rulesStr), MaxRulesSize)
	}
	if !utf8.ValidString(rulesStr) {
		return fmt.Errorf("rules contain invalid UTF-8 sequence")
	}
	return nil
}

// sanitizeParamName validates and cleans parameter name
func (rp *RuleParser) sanitizeParamName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("parameter name cannot be empty")
	}
	if len(name) > MaxParamNameLength {
		return "", fmt.Errorf("parameter name too long: %d characters", len(name))
	}

	if !rp.isValidParamName(name) {
		return "", fmt.Errorf("invalid characters in parameter name: %s", name)
	}

	return name, nil
}

// isValidParamName checks if parameter name contains only allowed characters
func (rp *RuleParser) isValidParamName(name string) bool {
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

// isValidURLPattern validates URL pattern for security and format
func (rp *RuleParser) isValidURLPattern(pattern string) bool {
	if pattern == "" || len(pattern) > MaxURLLength {
		return false
	}

	if strings.Contains(pattern, "..") ||
		strings.Contains(pattern, "//") ||
		strings.Contains(pattern, "./") ||
		strings.Contains(pattern, "/.") ||
		strings.HasPrefix(pattern, "javascript:") ||
		strings.HasPrefix(pattern, "data:") ||
		strings.HasPrefix(pattern, "vbscript:") ||
		strings.HasPrefix(pattern, "file:") {
		return false
	}

	return true
}

// parseRulesUnsafe parses rules string without locking
func (rp *RuleParser) parseRulesUnsafe(rulesStr string) (map[string]*ParamRule, map[string]*URLRule, error) {
	if rulesStr == "" {
		return make(map[string]*ParamRule), make(map[string]*URLRule), nil
	}

	if err := rp.validateRulesString(rulesStr); err != nil {
		return nil, nil, err
	}

	globalParams := make(map[string]*ParamRule)
	urlRules := make(map[string]*URLRule)

	ruleType := rp.detectRuleType(rulesStr)

	switch ruleType {
	case RuleTypeURL:
		parsedURLRules, parsedGlobalParams, err := rp.parseURLRulesUnsafe(rulesStr)
		if err != nil {
			return nil, nil, err
		}
		for k, v := range parsedURLRules {
			urlRules[k] = v
		}
		for k, v := range parsedGlobalParams {
			globalParams[k] = v
		}
	case RuleTypeGlobal:
		parsedGlobalParams, err := rp.parseGlobalParamsUnsafe(rulesStr)
		if err != nil {
			return nil, nil, err
		}
		for k, v := range parsedGlobalParams {
			globalParams[k] = v
		}
	default:
		return nil, nil, fmt.Errorf("unknown rule type")
	}

	return globalParams, urlRules, nil
}

// detectRuleType determines the type of rules in the string
func (rp *RuleParser) detectRuleType(rulesStr string) RuleType {
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
func (rp *RuleParser) parseGlobalParamsUnsafe(rulesStr string) (map[string]*ParamRule, error) {
	return rp.parseParamsFromString(rulesStr, '&')
}

// parseURLRulesUnsafe parses URL-specific rules
func (rp *RuleParser) parseURLRulesUnsafe(rulesStr string) (map[string]*URLRule, map[string]*ParamRule, error) {
	urlRules := make(map[string]*URLRule)
	globalParams := make(map[string]*ParamRule)

	urlRuleStrings := rp.splitURLRules(rulesStr)

	for _, urlRuleStr := range urlRuleStrings {
		if urlRuleStr == "" {
			continue
		}

		urlPattern, paramsStr := rp.extractURLAndParams(urlRuleStr)

		if urlPattern == "" && paramsStr != "" {
			parsedGlobalParams, err := rp.parseGlobalParamsUnsafe(paramsStr)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse global params: %w", err)
			}
			for k, v := range parsedGlobalParams {
				globalParams[k] = v
			}
			continue
		}

		urlPattern = normalizeURLPattern(urlPattern)
		if urlPattern == "" {
			continue
		}

		params, err := rp.parseParamsFromString(paramsStr, '&')
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse params for URL %s: %w", urlPattern, err)
		}

		if urlPattern != "" {
			urlRule := &URLRule{
				URLPattern: urlPattern,
				Params:     params,
			}
			urlRules[urlPattern] = urlRule
		}
	}

	return urlRules, globalParams, nil
}

// parseParamsFromString parses parameters from string with given separator
func (rp *RuleParser) parseParamsFromString(paramsStr string, separator byte) (map[string]*ParamRule, error) {
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

	paramStrings := rp.splitRules(paramsStr, separator)

	for _, paramStr := range paramStrings {
		rule, err := rp.parseSingleParamRuleUnsafe(paramStr)
		if err != nil {
			return nil, err
		}
		if rule != nil {
			params[rule.Name] = rule
		}
	}

	return params, nil
}

// splitRules splits rules string considering bracket nesting
func (rp *RuleParser) splitRules(rulesStr string, separator byte) []string {
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
func (rp *RuleParser) splitURLRules(rulesStr string) []string {
	var builder strings.Builder
	builder.Grow(len(rulesStr))

	for _, r := range rulesStr {
		if r != ' ' && r != '\n' {
			builder.WriteRune(r)
		}
	}
	cleanRulesStr := builder.String()

	if strings.Contains(cleanRulesStr, ";") {
		return rp.splitRules(rulesStr, ';')
	}

	return []string{rulesStr}
}

// extractURLAndParams separates URL pattern from parameters string
func (rp *RuleParser) extractURLAndParams(urlRuleStr string) (string, string) {
	cleanStr := strings.ReplaceAll(urlRuleStr, " ", "")
	cleanStr = strings.ReplaceAll(cleanStr, "**", "*")
	if !rp.isValidURLPattern(cleanStr) {
		return "", ""
	}
	if strings.HasPrefix(cleanStr, "/") || strings.HasPrefix(cleanStr, "*") {
		bracketDepth := 0
		questionMarkPos := -1
		breakMe := false

		for i := 0; i < len(cleanStr); i++ {
			if breakMe {
				break
			}
			switch cleanStr[i] {
			case '[':
				bracketDepth++
			case ']':
				if bracketDepth > 0 {
					bracketDepth--
				}
			case '?':
				if bracketDepth == 0 {
					questionMarkPos = i
					breakMe = true
				}
			}
		}

		if questionMarkPos != -1 {
			urlPattern := strings.TrimSpace(cleanStr[:questionMarkPos])
			paramsStr := strings.TrimSpace(cleanStr[questionMarkPos+1:])
			return urlPattern, paramsStr
		} else {
			bracketPos := strings.Index(cleanStr, "[")
			if bracketPos != -1 {
				urlPattern := strings.TrimSpace(cleanStr[:bracketPos])
				paramsStr := strings.TrimSpace(cleanStr[bracketPos:])
				return urlPattern, paramsStr
			} else {
				return strings.TrimSpace(cleanStr), ""
			}
		}
	}

	return "", urlRuleStr
}

// parseSingleParamRuleUnsafe parses single parameter rule
func (rp *RuleParser) parseSingleParamRuleUnsafe(ruleStr string) (*ParamRule, error) {
	ruleStr = strings.TrimSpace(ruleStr)
	if ruleStr == "" {
		return nil, nil
	}
	ruleStr = strings.ReplaceAll(ruleStr, "**", "*")
	ruleStr = strings.ReplaceAll(ruleStr, "![*]", "[]")

	inverted := false
	if strings.Contains(ruleStr, "![") {
		inverted = true
		ruleStr = strings.Replace(ruleStr, "![", "[", 1)
	}

	if strings.HasSuffix(ruleStr, "=[]") {
		paramName := strings.TrimSuffix(ruleStr, "=[]")
		paramName, err := rp.sanitizeParamName(paramName)
		if err != nil {
			return nil, fmt.Errorf("invalid parameter name in key-only rule: %w", err)
		}
		return &ParamRule{
			Name:     paramName,
			Pattern:  PatternKeyOnly,
			Inverted: inverted,
		}, nil
	}

	if strings.HasSuffix(ruleStr, "=[?]") {
		paramName := strings.TrimSuffix(ruleStr, "=[?]")
		paramName, err := rp.sanitizeParamName(paramName)
		if err != nil {
			return nil, fmt.Errorf("invalid parameter name in callback rule: %w", err)
		}
		return &ParamRule{
			Name:     paramName,
			Pattern:  PatternCallback,
			Inverted: inverted,
		}, nil
	}

	startBracket := strings.Index(ruleStr, "[")
	if startBracket == -1 {
		return rp.parseSimpleParamRule(ruleStr)
	}

	return rp.parseComplexParamRule(ruleStr, startBracket, inverted)
}

// parseSimpleParamRule parses simple parameter rule without brackets
func (rp *RuleParser) parseSimpleParamRule(ruleStr string) (*ParamRule, error) {
	if strings.Contains(ruleStr, "=") {
		paramName := strings.Split(ruleStr, "=")[0]
		paramName, err := rp.sanitizeParamName(paramName)
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

	paramName, err := rp.sanitizeParamName(ruleStr)
	if err != nil {
		return nil, fmt.Errorf("invalid parameter name: %w", err)
	}

	return &ParamRule{
		Name:    paramName,
		Pattern: PatternAny,
	}, nil
}

// parseComplexParamRule parses parameter rule with bracket constraints
func (rp *RuleParser) parseComplexParamRule(ruleStr string, startBracket int, inverted bool) (*ParamRule, error) {
	paramName := strings.TrimSpace(ruleStr[:startBracket])

	if strings.HasSuffix(paramName, "=") {
		paramName = strings.TrimSuffix(paramName, "=")
		paramName = strings.TrimSpace(paramName)
	}

	if idx := strings.Index(paramName, "?"); idx != -1 {
		paramName = paramName[idx+1:]
	}
	if idx := strings.Index(paramName, "&"); idx != -1 {
		paramName = paramName[idx+1:]
	}

	paramName, err := rp.sanitizeParamName(paramName)
	if err != nil {
		return nil, fmt.Errorf("invalid parameter name in rule: %w", err)
	}

	constraintStr, endBracket := rp.extractConstraint(ruleStr, startBracket)
	if endBracket == -1 {
		return nil, fmt.Errorf("unclosed bracket in rule: %s", ruleStr)
	}

	if constraintStr == "" {
		return &ParamRule{
			Name:     paramName,
			Pattern:  PatternAny,
			Inverted: inverted,
		}, nil
	}

	rule, err := rp.createParamRule(paramName, constraintStr)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, fmt.Errorf("failed to create rule for parameter %s with constraint %s", paramName, constraintStr)
	}

	rule.Inverted = inverted
	return rule, nil
}

// extractConstraint extracts content between brackets
func (rp *RuleParser) extractConstraint(ruleStr string, startBracket int) (string, int) {
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

	constraint := strings.TrimSpace(ruleStr[startBracket+1 : endBracket])

	if len(constraint) > MaxPatternLength {
		return "", -1
	}

	return constraint, endBracket
}

// createParamRule creates parameter rule using plugins or standard parsing
func (rp *RuleParser) createParamRule(paramName, constraintStr string) (*ParamRule, error) {
	rule := &ParamRule{Name: paramName}

	// Try plugins first
	validatorFunc, pluginUsed, err := rp.tryPlugins(paramName, constraintStr)
	if err != nil {
		return nil, err
	}

	if pluginUsed {
		rule.Pattern = "plugin"
		rule.CustomValidator = validatorFunc
		rule.ConstraintStr = constraintStr
		return rule, nil
	}

	// Fall back to standard rules
	standardRule, err := rp.createStandardParamRule(paramName, constraintStr)
	if err != nil {
		return nil, err
	}
	standardRule.ConstraintStr = constraintStr
	return standardRule, nil
}

// tryPlugins attempts to parse constraint using registered plugins
// Returns: validatorFunc, pluginUsed, error
func (rp *RuleParser) tryPlugins(paramName, constraintStr string) (func(string) bool, bool, error) {
	// First check cache
	for _, plugin := range rp.plugins {
		if rp.cache != nil {
			if validatorFunc, found := rp.cache.Get(plugin.GetName(), paramName, constraintStr); found {
				return validatorFunc, true, nil
			}
		}
	}

	// Try parsing with each plugin
	var pluginErrors []string
	for _, plugin := range rp.plugins {
		validatorFunc, err := plugin.Parse(paramName, constraintStr)
		if err == nil && validatorFunc != nil {
			// Success - cache and return
			if rp.cache != nil {
				rp.cache.Put(plugin.GetName(), paramName, constraintStr, validatorFunc)
			}
			return validatorFunc, true, nil
		}

		if err != nil {
			// Если это ошибка "не для этого плагина", продолжаем поиск
			if isNotForPluginError(err) {
				pluginErrors = append(pluginErrors, fmt.Sprintf("%s: %v", plugin.GetName(), err))
				continue
			}
			// Любая другая ошибка - это синтаксическая ошибка, возвращаем её
			return nil, false, fmt.Errorf("plugin %s: %w", plugin.GetName(), err)
		}
	}

	// Если все плагины вернули "не для этого плагина", это нормально - используем стандартные правила
	if len(pluginErrors) > 0 {
		// Логируем для отладки, но не возвращаем ошибку
		// fmt.Printf("All plugins rejected constraint '%s': %v\n", constraintStr, pluginErrors)
		return nil, false, nil
	}

	// No plugin could handle this constraint, but that's OK - use standard rules
	return nil, false, nil
}

// isNotForPluginError checks if error indicates that constraint is not for this plugin
func isNotForPluginError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	return strings.Contains(errStr, "not for this plugin") ||
		strings.Contains(errStr, "unknown constraint format") ||
		strings.Contains(errStr, "constraint too short")
}

// createStandardParamRule creates parameter rule using standard patterns
func (rp *RuleParser) createStandardParamRule(paramName, constraintStr string) (*ParamRule, error) {
	rule := &ParamRule{Name: paramName}

	switch {
	case constraintStr == "":
		rule.Pattern = PatternKeyOnly
	case constraintStr == PatternAll:
		rule.Pattern = PatternAny
	case constraintStr == "?":
		rule.Pattern = PatternCallback
	case strings.Contains(constraintStr, ","):
		if err := rp.parseEnumConstraint(rule, constraintStr); err != nil {
			return nil, err
		}
	default:
		rule.Pattern = PatternEnum
		rule.Values = []string{constraintStr}
	}

	return rule, nil
}

// parseEnumConstraint parses enum constraint with allowed values
func (rp *RuleParser) parseEnumConstraint(rule *ParamRule, constraintStr string) error {
	values := strings.Split(constraintStr, ",")
	if len(values) == 0 {
		return fmt.Errorf("empty enum constraint")
	}

	rule.Pattern = PatternEnum
	rule.Values = make([]string, 0, len(values))

	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			rule.Values = append(rule.Values, trimmedValue)
		}
	}

	if len(rule.Values) == 0 {
		return fmt.Errorf("no valid values in enum constraint")
	}

	sort.Strings(rule.Values)
	return nil
}

// ClearCache clears the validation cache
func (rp *RuleParser) ClearCache() {
	if rp.cache != nil {
		rp.cache.Clear()
	}
}

// CheckRulesSyntax validates rules syntax with full plugin validation
func (rp *RuleParser) CheckRulesSyntax(rulesStr string) error {
	if rulesStr == "" {
		return nil
	}

	if err := rp.validateRulesString(rulesStr); err != nil {
		return err
	}

	globalParams, urlRules, err := rp.parseRulesUnsafe(rulesStr)
	if err != nil {
		return err
	}

	return rp.testPluginValidation(globalParams, urlRules)
}

// testPluginValidation tests plugin validation functions for all constraints
func (rp *RuleParser) testPluginValidation(globalParams map[string]*ParamRule, urlRules map[string]*URLRule) error {
	checkConstraint := func(paramName, constraintStr string) error {
		// Try plugins first (same logic as in createParamRule)
		_, pluginUsed, err := rp.tryPlugins(paramName, constraintStr)
		if err != nil {
			return fmt.Errorf("parameter '%s': plugin error for constraint '%s': %w", paramName, constraintStr, err)
		}

		// If no plugin handled it, check if it's a valid standard constraint
		if !pluginUsed {
			testRule := &ParamRule{Name: paramName}
			if strings.Contains(constraintStr, ",") {
				// Test enum constraint
				if err := rp.parseEnumConstraint(testRule, constraintStr); err != nil {
					return fmt.Errorf("parameter '%s': invalid enum constraint '%s': %w", paramName, constraintStr, err)
				}
			}
			// Other standard patterns (PatternKeyOnly, PatternAny, PatternCallback, PatternEnum)
			// are always valid, so no need to test them
		}

		return nil
	}

	for paramName, rule := range globalParams {
		if rule.ConstraintStr != "" {
			if err := checkConstraint(paramName, rule.ConstraintStr); err != nil {
				return err
			}
		}
	}

	for _, urlRule := range urlRules {
		for paramName, rule := range urlRule.Params {
			if rule.ConstraintStr != "" {
				if err := checkConstraint(paramName, rule.ConstraintStr); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
