// pattern_plugin.go
package plugins

import (
	"fmt"
	"strings"
)

const maxPatternLength = 1000

type PatternPlugin struct {
	name string
}

func NewPatternPlugin() *PatternPlugin {
	return &PatternPlugin{name: "in"}
}

func (pp *PatternPlugin) GetName() string {
	return pp.name
}

func (pp *PatternPlugin) Parse(paramName, constraintStr string) (func(string) bool, error) {
	prefix := pp.name + ":"
	if len(constraintStr) < len(prefix) || !strings.HasPrefix(constraintStr, prefix) {
		return nil, fmt.Errorf("not for this plugin: pattern constraint must start with '%s:'", pp.name)
	}

	pattern := strings.TrimSpace(constraintStr[3:])
	if pattern == "" {
		return nil, fmt.Errorf("not for this plugin: empty pattern")
	}

	if len(pattern) > maxPatternLength {
		return nil, fmt.Errorf("pattern too long: %d characters", len(pattern))
	}

	if !isValidUTF8(pattern) {
		return nil, fmt.Errorf("invalid UTF-8 in pattern")
	}

	// Check for wildcard presence
	hasWildcard := false
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '*' {
			hasWildcard = true
			break
		}
	}
	if !hasWildcard {
		return nil, fmt.Errorf("pattern must contain at least one wildcard '*'")
	}

	hasLeadingStar := pattern[0] == '*'
	hasTrailingStar := pattern[len(pattern)-1] == '*'

	// Optimize common patterns
	if hasLeadingStar && hasTrailingStar && len(pattern) == 2 {
		return func(value string) bool {
			return len(value) <= maxPatternLength*10
		}, nil
	}

	if hasLeadingStar && !hasTrailingStar && strings.Count(pattern, "*") == 1 {
		suffix := pattern[1:]
		return func(value string) bool {
			if len(value) > maxPatternLength*10 {
				return false
			}
			return strings.HasSuffix(value, suffix)
		}, nil
	}

	if !hasLeadingStar && hasTrailingStar && strings.Count(pattern, "*") == 1 {
		prefix := pattern[:len(pattern)-1]
		return func(value string) bool {
			if len(value) > maxPatternLength*10 {
				return false
			}
			return strings.HasPrefix(value, prefix)
		}, nil
	}

	parts := strings.Split(pattern, "*")
	return pp.createValidator(parts), nil
}

func (pp *PatternPlugin) createValidator(parts []string) func(string) bool {
	return func(value string) bool {
		if len(value) > maxPatternLength*10 {
			return false
		}

		if len(parts) == 2 && parts[0] == "" && parts[1] == "" {
			return true
		}

		start := 0
		for i, part := range parts {
			if part == "" {
				continue
			}

			if i == 0 {
				if !strings.HasPrefix(value, part) {
					return false
				}
				start = len(part)
			} else if i == len(parts)-1 {
				if !strings.HasSuffix(value[start:], part) {
					return false
				}
			} else {
				pos := strings.Index(value[start:], part)
				if pos == -1 {
					return false
				}
				start += pos + len(part)
			}
		}
		return true
	}
}

func (pp *PatternPlugin) Close() error {
	return nil
}
