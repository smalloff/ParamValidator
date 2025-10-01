package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

var (
	comparisonPlugin = plugins.NewComparisonPlugin()
	lengthPlugin     = plugins.NewLengthPlugin()
	rangePlugin      = plugins.NewRangePlugin()
	patternPlugin    = plugins.NewPatternPlugin()

	allPlugins = []struct {
		name   string
		plugin interface {
			Parse(paramName, constraintStr string) (func(string) bool, error)
		}
	}{
		{"comparison", comparisonPlugin},
		{"length", lengthPlugin},
		{"range", rangePlugin},
		{"pattern", patternPlugin},
	}
)

func BenchmarkMixedPlugins(b *testing.B) {
	constraints := []string{
		"cmp:>100",
		"len:>5",
		"range:1-100",
		"in:*test*",
		"cmp:<50",
		"len:5..15",
		"len:>=8",
		"range:18-65",
		"in:prefix*",
		"invalid",
		"len:gth>=10",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, constraint := range constraints {
			for _, pl := range allPlugins {
				_, _ = pl.plugin.Parse("test_param", constraint)
			}
		}
	}
}

func BenchmarkAllPluginsParse(b *testing.B) {
	pluginConstraints := []struct {
		plugin interface {
			Parse(string, string) (func(string) bool, error)
		}
		constraints []string
	}{
		{
			plugin:      comparisonPlugin,
			constraints: []string{"cmp:>100", "cmp:<50", "cmp:>=10", "cmp:<=200", "cmp:>=-50"},
		},
		{
			plugin:      lengthPlugin,
			constraints: []string{"len:>5", "len:<20", "len:>=10", "len:<=15", "len:5..10"},
		},
		{
			plugin:      rangePlugin,
			constraints: []string{"range:1-100", "range:18..65", "range:-10..10", "range:0-1000", "range:5..5"},
		},
		{
			plugin:      patternPlugin,
			constraints: []string{"in:*test*", "in:prefix*", "in:*suffix", "in:*a*b*c*", "in:start*end"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pc := range pluginConstraints {
			for _, constraint := range pc.constraints {
				_, _ = pc.plugin.Parse("test_param", constraint)
			}
		}
	}
}

func BenchmarkAllPluginsValidation(b *testing.B) {
	validators := []func(string) bool{}

	if v, err := comparisonPlugin.Parse("age", "cmp:>50"); err == nil {
		validators = append(validators, v)
	}

	if v, err := lengthPlugin.Parse("username", "len:>5"); err == nil {
		validators = append(validators, v)
	}

	if v, err := rangePlugin.Parse("score", "range:1-100"); err == nil {
		validators = append(validators, v)
	}

	if v, err := patternPlugin.Parse("file", "in:*test*"); err == nil {
		validators = append(validators, v)
	}

	testValues := []string{"25", "50", "75", "hi", "hello", "test", "testing", "contest"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, validator := range validators {
			for _, value := range testValues {
				result := validator(value)
				_ = result
			}
		}
	}
}

func BenchmarkPluginIntegration(b *testing.B) {
	parser := NewRuleParser(
		comparisonPlugin,
		lengthPlugin,
		rangePlugin,
		patternPlugin,
	)

	rules := []string{
		"/api?age=[range:18-65]&score=[cmp:>50]&username=[len:>5]&file=[in:img_*]",
		"/users?level=[range:1-10]&status=[active,inactive]&name=[len:3..20]&email=[in:*@*]",
		"/products?price=[cmp:<1000]&quantity=[range:1-100]&code=[len:6]&category=[in:*_*]",
	}

	parsedResults := make([]struct {
		globalParams map[string]*ParamRule
		urlRules     map[string]*URLRule
	}, len(rules))

	for i, rule := range rules {
		global, url, _ := parser.parseRulesUnsafe(rule)
		parsedResults[i] = struct {
			globalParams map[string]*ParamRule
			urlRules     map[string]*URLRule
		}{global, url}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, result := range parsedResults {
			_ = result.globalParams
			_ = result.urlRules
		}
	}
}

func BenchmarkPluginConcurrentUsage(b *testing.B) {
	parser := NewRuleParser(
		comparisonPlugin,
		lengthPlugin,
		rangePlugin,
		patternPlugin,
	)

	rules := "/api?age=[range:18-65]&score=[cmp:>50]&username=[len:>5]&file=[in:img_*]"

	globalParams, urlRules, err := parser.parseRulesUnsafe(rules)
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}

	validators := make([]func(string) bool, 0)
	for _, param := range globalParams {
		if param != nil && param.CustomValidator != nil {
			validators = append(validators, param.CustomValidator)
		}
	}
	for _, urlRule := range urlRules {
		for _, param := range urlRule.Params {
			if param != nil && param.CustomValidator != nil {
				validators = append(validators, param.CustomValidator)
			}
		}
	}

	testValues := []string{"18", "25", "50", "75", "john_doe", "img_photo.jpg", "test", "hello world"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, validator := range validators {
				for _, value := range testValues {
					result := validator(value)
					_ = result
				}
			}
		}
	})
}

func BenchmarkPluginMemoryUsage(b *testing.B) {
	allValidators := []func(string) bool{}

	for _, constraint := range []string{"cmp:>10", "cmp:>50", "cmp:>100", "cmp:<10", "cmp:<50", "cmp:<100"} {
		if v, err := comparisonPlugin.Parse("test_param", constraint); err == nil {
			allValidators = append(allValidators, v)
		}
	}

	for _, constraint := range []string{"len:>5", "len:>10", "len:<20", "len:5..10", "len:10..20"} {
		if v, err := lengthPlugin.Parse("test_param", constraint); err == nil {
			allValidators = append(allValidators, v)
		}
	}

	for _, constraint := range []string{"range:1-10", "range:10-100", "range:100-1000", "range:-10..10", "range:0..100"} {
		if v, err := rangePlugin.Parse("test_param", constraint); err == nil {
			allValidators = append(allValidators, v)
		}
	}

	for _, constraint := range []string{"in:*test*", "in:prefix*", "in:*suffix", "in:*a*b*", "in:start*end*"} {
		if v, err := patternPlugin.Parse("test_param", constraint); err == nil {
			allValidators = append(allValidators, v)
		}
	}

	testValues := []string{"test", "25", "hello world", "prefix_value"}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, validator := range allValidators {
			for _, value := range testValues {
				result := validator(value)
				_ = result
			}
		}
	}
}

func BenchmarkPluginEdgeCases(b *testing.B) {
	edgeCases := []struct {
		name   string
		plugin interface {
			Parse(string, string) (func(string) bool, error)
		}
		constraints []string
	}{
		{
			name:   "comparison_edge",
			plugin: comparisonPlugin,
			constraints: []string{
				"cmp:>-100",
				"cmp:>999999",
				"cmp:>",
				"cmp:>>10",
				"cmp:>abc",
			},
		},
		{
			name:   "length_edge",
			plugin: lengthPlugin,
			constraints: []string{
				"len:=0",
				"len:>99999",
				"len:",
				"len:>>5",
				"len:>abc",
			},
		},
		{
			name:   "range_edge",
			plugin: rangePlugin,
			constraints: []string{
				"range:0..0",
				"range:-100..100",
				"range:10..5",
				"range:1..999999",
				"range:a..z",
			},
		},
		{
			name:   "pattern_edge",
			plugin: patternPlugin,
			constraints: []string{
				"in:*",
				"in:**",
				"in:",
				"in:*.*+?[]",
				"in:*🎉*🚀*",
			},
		},
	}

	validators := make([]func(string) bool, 0)
	testValues := []string{"", "test", "123", "hello world"}

	for _, ec := range edgeCases {
		for _, constraint := range ec.constraints {
			if validator, err := ec.plugin.Parse("test_param", constraint); err == nil && validator != nil {
				validators = append(validators, validator)
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, validator := range validators {
			for _, value := range testValues {
				result := validator(value)
				_ = result
			}
		}
	}
}

func BenchmarkPluginRealWorldScenario(b *testing.B) {
	pv, err := NewParamValidator("", WithPlugins(
		comparisonPlugin,
		lengthPlugin,
		rangePlugin,
		patternPlugin,
	))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	rules := `
		age=[range:18-65];
		/user/*?score=[cmp:>0]&level=[range:1-10]&username=[len:3..20];
		/api/v1/*?token=[len:32]&limit=[range:1-100]&offset=[cmp:>=0];
		/products?price=[cmp:<10000]&category=[in:*_*]&status=[active,inactive];
		/search?q=[len:1..100]&page=[range:1-100]&sort=[name,date,price];
	`

	err = pv.ParseRules(rules)
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}

	testURLs := []string{
		"/user/123?score=85&level=5&username=john_doe",
		"/api/v1/data?token=abc123def456ghi789jkl012mno345pq&limit=50&offset=0",
		"/products?price=2500&category=electronics_phones&status=active",
		"/search?q=laptop&page=1&sort=price",
		"/user/profile?score=95&level=8&username=alice_smith",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := testURLs[i%len(testURLs)]
		pv.ValidateURL(url)
		pv.FilterURL(url + "&invalid=param&extra=value")
	}
}

func BenchmarkPluginParseOnly(b *testing.B) {
	constraints := []string{
		"cmp:>100", "len:>5", "range:1-100", "in:*test*", "cmp:<50", "len:5..15",
		"invalid", "len:gth>=10", "in:prefix*", "range:18-65",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, constraint := range constraints {
			for _, pl := range allPlugins {
				_, _ = pl.plugin.Parse("test_param", constraint)
			}
		}
	}
}

func BenchmarkPluginValidationOnly(b *testing.B) {
	validators := make([]func(string) bool, 0, 10)

	if v, err := comparisonPlugin.Parse("test", "cmp:>50"); err == nil {
		validators = append(validators, v)
	}
	if v, err := lengthPlugin.Parse("test", "len:>5"); err == nil {
		validators = append(validators, v)
	}
	if v, err := rangePlugin.Parse("test", "range:1-100"); err == nil {
		validators = append(validators, v)
	}
	if v, err := patternPlugin.Parse("test", "in:*test*"); err == nil {
		validators = append(validators, v)
	}

	testValues := []string{"25", "50", "75", "hello", "test", "testing"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, validator := range validators {
			for _, value := range testValues {
				result := validator(value)
				_ = result
			}
		}
	}
}

func BenchmarkPluginParseAndValidate(b *testing.B) {
	pluginConstraints := []struct {
		plugin interface {
			Parse(string, string) (func(string) bool, error)
		}
		constraints []string
	}{
		{
			plugin:      comparisonPlugin,
			constraints: []string{"cmp:>100", "cmp:<50", "cmp:>=10"},
		},
		{
			plugin:      lengthPlugin,
			constraints: []string{"len:>5", "len:<20", "len:5..10"},
		},
		{
			plugin:      rangePlugin,
			constraints: []string{"range:1-100", "range:18..65", "range:-10..10"},
		},
		{
			plugin:      patternPlugin,
			constraints: []string{"in:*test*", "in:prefix*", "in:*suffix"},
		},
	}

	testValues := []string{"25", "50", "75", "hello", "test", "prefix_value"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pc := range pluginConstraints {
			for _, constraint := range pc.constraints {
				validator, err := pc.plugin.Parse("test_param", constraint)
				if err != nil || validator == nil {
					continue
				}

				for _, value := range testValues {
					result := validator(value)
					_ = result
				}
			}
		}
	}
}
