package paramvalidator

import (
	"fmt"
	"sort"
	"strings"
)

// PluginConstraintParser interface for custom constraint parsers
type PluginConstraintParser interface {
	// CanParse checks if plugin can handle this constraint
	CanParse(constraintStr string) bool

	// Parse creates validation function for parameter
	Parse(paramName, constraintStr string) (func(string) bool, error)

	// GetName returns plugin name for debugging
	GetName() string
}

// PluginResourceManager interface for plugin resource management
type PluginResourceManager interface {
	// Close releases plugin resources
	Close() error
}

// RuleParser handles parsing of validation rules with plugin support
type RuleParser struct {
	plugins []PluginConstraintParser
	cache   *ValidationCache
}

// NewRuleParser creates a new rule parser
func NewRuleParser(plugins ...PluginConstraintParser) *RuleParser {
	return &RuleParser{
		plugins: plugins,
		cache:   NewValidationCache(),
	}
}

// RegisterPlugin registers a new plugin
func (rp *RuleParser) RegisterPlugin(plugin PluginConstraintParser) {
	rp.plugins = append(rp.plugins, plugin)
}

// Close releases parser resources
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

// parseRulesUnsafe parses rules string without locking
func (rp *RuleParser) parseRulesUnsafe(rulesStr string) (map[string]*ParamRule, map[string]*URLRule, error) {
	if rulesStr == "" {
		return make(map[string]*ParamRule), make(map[string]*URLRule), nil
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
		// Merge results
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
	globalParams := make(map[string]*ParamRule)
	rules := rp.splitRules(rulesStr, '&')

	for _, ruleStr := range rules {
		if ruleStr == "" {
			continue
		}

		rule, err := rp.parseSingleParamRuleUnsafe(ruleStr)
		if err != nil {
			return nil, err
		}
		if rule != nil {
			globalParams[rule.Name] = rule
		}
	}

	return globalParams, nil
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

		urlPattern = NormalizeURLPattern(urlPattern)
		if urlPattern == "" {
			continue
		}

		params, err := rp.parseParamsStringUnsafe(paramsStr)
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

	// Remove whitespace for cleaner parsing
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

	if strings.HasPrefix(cleanStr, "/") || strings.HasPrefix(cleanStr, "*") {
		bracketDepth := 0
		questionMarkPos := -1
		breakMe := false

		// Find the question mark outside of brackets
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
			// URL with parameters: /path?param=value
			urlPattern := strings.TrimSpace(cleanStr[:questionMarkPos])
			paramsStr := strings.TrimSpace(cleanStr[questionMarkPos+1:])
			return urlPattern, paramsStr
		} else {
			// No question mark, check for brackets directly after URL
			bracketPos := strings.Index(cleanStr, "[")
			if bracketPos != -1 {
				// URL pattern with inline parameters: /path[param=value]
				urlPattern := strings.TrimSpace(cleanStr[:bracketPos])
				paramsStr := strings.TrimSpace(cleanStr[bracketPos:])
				return urlPattern, paramsStr
			} else {
				// Just a URL pattern without parameters
				return strings.TrimSpace(cleanStr), ""
			}
		}
	}

	return "", urlRuleStr
}

// parseParamsStringUnsafe parses parameters string into map of rules
func (rp *RuleParser) parseParamsStringUnsafe(paramsStr string) (map[string]*ParamRule, error) {
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

	paramStrings := rp.splitRules(paramsStr, '&')

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

// parseSingleParamRuleUnsafe parses single parameter rule
func (rp *RuleParser) parseSingleParamRuleUnsafe(ruleStr string) (*ParamRule, error) {
	ruleStr = strings.TrimSpace(ruleStr)
	if ruleStr == "" {
		return nil, nil
	}

	ruleStr = strings.ReplaceAll(ruleStr, "![*]", "[]")

	inverted := false
	if strings.Contains(ruleStr, "![") {
		inverted = true
		ruleStr = strings.Replace(ruleStr, "![", "[", 1)
	}

	// Handle special patterns first
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

	// Remove trailing "=" if present (for URL rules like /path?param=[value])
	if strings.HasSuffix(paramName, "=") {
		paramName = strings.TrimSuffix(paramName, "=")
		paramName = strings.TrimSpace(paramName)
	}

	// For URL rules, extract only the actual parameter name (after last ? or &)
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

// createParamRule создает ParamRule с кэшированием функций плагинов
func (rp *RuleParser) createParamRule(paramName, constraintStr string) (*ParamRule, error) {
	rule := &ParamRule{Name: paramName}

	// First check plugins with caching
	for _, plugin := range rp.plugins {
		if plugin.CanParse(constraintStr) {
			if rp.cache != nil {
				if validatorFunc, found := rp.cache.Get(plugin.GetName(), paramName, constraintStr); found {
					rule.Pattern = "plugin"
					rule.CustomValidator = validatorFunc
					return rule, nil
				}
			}

			validatorFunc, err := plugin.Parse(paramName, constraintStr)
			if err != nil {
				return nil, fmt.Errorf("plugin %s failed to parse constraint '%s': %w",
					plugin.GetName(), constraintStr, err)
			}

			if rp.cache != nil {
				rp.cache.Put(plugin.GetName(), paramName, constraintStr, validatorFunc)
			}

			rule.Pattern = "plugin"
			rule.CustomValidator = validatorFunc
			return rule, nil
		}
	}

	return rp.createStandardParamRule(paramName, constraintStr)
}

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

	// Sort for consistent behavior
	sort.Strings(rule.Values)
	return nil
}

func (rp *RuleParser) ClearCache() {
	if rp.cache != nil {
		rp.cache.Clear()
	}
}
