package paramvalidator

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// RuleParser handles parsing of validation rules
type RuleParser struct{}

// NewRuleParser creates a new rule parser
func NewRuleParser() *RuleParser {
	return &RuleParser{}
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

	if strings.HasSuffix(ruleStr, "=[]") {
		paramName := strings.TrimSuffix(ruleStr, "=[]")
		paramName, err := rp.sanitizeParamName(paramName)
		if err != nil {
			return nil, fmt.Errorf("invalid parameter name in key-only rule: %w", err)
		}
		return &ParamRule{
			Name:    paramName,
			Pattern: PatternKeyOnly,
		}, nil
	}

	if strings.HasSuffix(ruleStr, "=[?]") {
		paramName := strings.TrimSuffix(ruleStr, "=[?]")
		paramName, err := rp.sanitizeParamName(paramName)
		if err != nil {
			return nil, fmt.Errorf("invalid parameter name in callback rule: %w", err)
		}
		return &ParamRule{
			Name:    paramName,
			Pattern: PatternCallback,
		}, nil
	}

	startBracket := strings.Index(ruleStr, "[")
	if startBracket == -1 {
		return rp.parseSimpleParamRule(ruleStr)
	}

	return rp.parseComplexParamRule(ruleStr, startBracket)
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
func (rp *RuleParser) parseComplexParamRule(ruleStr string, startBracket int) (*ParamRule, error) {
	paramName := strings.TrimSpace(ruleStr[:startBracket])
	if strings.HasSuffix(paramName, "=") {
		paramName = strings.TrimSuffix(paramName, "=")
		paramName = strings.TrimSpace(paramName)
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
			Name:    paramName,
			Pattern: PatternAny,
		}, nil
	}

	return rp.createParamRule(paramName, constraintStr)
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

// createParamRule creates ParamRule from name and constraint
func (rp *RuleParser) createParamRule(paramName, constraintStr string) (*ParamRule, error) {
	rule := &ParamRule{Name: paramName}

	switch {
	case constraintStr == "":
		rule.Pattern = PatternKeyOnly
	case constraintStr == PatternAll:
		rule.Pattern = PatternAny
	case constraintStr == "?":
		rule.Pattern = PatternCallback
	case rp.isRangeConstraint(constraintStr):
		if err := rp.parseRangeConstraint(rule, constraintStr); err != nil {
			return nil, err
		}
	case strings.Contains(constraintStr, ","):
		if err := rp.parseEnumConstraint(rule, constraintStr); err != nil {
			return nil, err
		}
	default:
		// Single value enum
		rule.Pattern = PatternEnum
		rule.Values = []string{constraintStr}
	}

	return rule, nil
}

// isRangeConstraint checks if constraint is a numeric range
func (rp *RuleParser) isRangeConstraint(constraintStr string) bool {
	// Support both formats: "1-10" and "1..10"
	return (strings.Contains(constraintStr, "-") || strings.Contains(constraintStr, "..")) &&
		!strings.Contains(constraintStr, ",")
}

// parseRangeConstraint parses numeric range constraint
func (rp *RuleParser) parseRangeConstraint(rule *ParamRule, constraintStr string) error {
	// Normalize to use hyphen as separator
	normalized := strings.Replace(constraintStr, "..", "-", -1)
	parts := strings.Split(normalized, "-")
	if len(parts) != 2 {
		return fmt.Errorf("invalid range format: %s", constraintStr)
	}

	minStr := strings.TrimSpace(parts[0])
	maxStr := strings.TrimSpace(parts[1])

	min, err := strconv.ParseInt(minStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid min value in range: %s", minStr)
	}

	max, err := strconv.ParseInt(maxStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid max value in range: %s", maxStr)
	}

	if min > max {
		return fmt.Errorf("min value cannot be greater than max value: %d..%d", min, max)
	}

	rule.Pattern = PatternRange
	rule.Min = min
	rule.Max = max
	return nil
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

	// Sort for consistent behavior (optional)
	sort.Strings(rule.Values)
	return nil
}
