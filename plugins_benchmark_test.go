// plugins_benchmark_test.go
package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

// Создаем плагины один раз для всех бенчмарков
var (
	comparisonPlugin = plugins.NewComparisonPlugin()
	lengthPlugin     = plugins.NewLengthPlugin()
	rangePlugin      = plugins.NewRangePlugin()
	patternPlugin    = plugins.NewPatternPlugin()

	allPlugins = []struct {
		name   string
		plugin interface {
			CanParse(constraintStr string) bool
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
	// Смешанные констрейнты для разных плагинов
	constraints := []string{
		">100",       // comparison
		"len>5",      // length
		"1-100",      // range
		"*test*",     // pattern
		"<50",        // comparison
		"len5..15",   // length
		"len>=8",     // length
		"18-65",      // range
		"prefix*",    // pattern
		"invalid",    // none
		"length>=10", // none (invalid for length plugin)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, constraint := range constraints {
			for _, pl := range allPlugins {
				_ = pl.plugin.CanParse(constraint)
			}
		}
	}
}

func BenchmarkAllPluginsCanParse(b *testing.B) {
	pluginConstraints := []struct {
		plugin      interface{ CanParse(string) bool }
		constraints []string
	}{
		{
			plugin:      comparisonPlugin,
			constraints: []string{">100", "<50", ">=10", "<=200", ">=-50"},
		},
		{
			plugin:      lengthPlugin,
			constraints: []string{"len>5", "len<20", "len>=10", "len<=15", "len5..10"},
		},
		{
			plugin:      rangePlugin,
			constraints: []string{"1-100", "18..65", "-10..10", "0-1000", "5..5"},
		},
		{
			plugin:      patternPlugin,
			constraints: []string{"*test*", "prefix*", "*suffix", "*a*b*c*", "start*end"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pc := range pluginConstraints {
			for _, constraint := range pc.constraints {
				_ = pc.plugin.CanParse(constraint)
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
			constraints: []string{">100", "<50", ">=10"},
		},
		{
			plugin:      lengthPlugin,
			constraints: []string{"len>5", "len<20", "len5..10"},
		},
		{
			plugin:      rangePlugin,
			constraints: []string{"1-100", "18..65", "-10..10"},
		},
		{
			plugin:      patternPlugin,
			constraints: []string{"*test*", "prefix*", "*suffix"},
		},
	}

	// Предварительно создаем все валидаторы
	allValidators := make([][]func(string) bool, len(pluginConstraints))
	for i, pc := range pluginConstraints {
		validators := make([]func(string) bool, 0, len(pc.constraints))
		for _, constraint := range pc.constraints {
			validator, err := pc.plugin.Parse("test_param", constraint)
			if err == nil && validator != nil {
				validators = append(validators, validator)
			}
		}
		allValidators[i] = validators
	}

	testValues := []string{"25", "50", "75", "hello", "test", "prefix_value"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, validators := range allValidators {
			for _, validator := range validators {
				for _, value := range testValues {
					result := validator(value)
					_ = result
				}
			}
		}
	}
}

func BenchmarkAllPluginsValidation(b *testing.B) {
	// Создаем валидаторы один раз
	validators := []func(string) bool{}

	// Comparison
	if v, err := comparisonPlugin.Parse("age", ">50"); err == nil {
		validators = append(validators, v)
	}

	// Length
	if v, err := lengthPlugin.Parse("username", "len>5"); err == nil {
		validators = append(validators, v)
	}

	// Range
	if v, err := rangePlugin.Parse("score", "1-100"); err == nil {
		validators = append(validators, v)
	}

	// Pattern
	if v, err := patternPlugin.Parse("file", "*test*"); err == nil {
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

	// Комплексные правила, использующие все плагины
	rules := []string{
		"/api?age=[18-65]&score=[>50]&username=[len>5]&file=[img_*]",
		"/users?level=[1-10]&status=[active,inactive]&name=[len3..20]&email=[*@*]",
		"/products?price=[<1000]&quantity=[1-100]&code=[len=6]&category=[*_*]",
	}

	// Парсим правила один раз
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
		// Используем уже распарсенные правила
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

	rules := "/api?age=[18-65]&score=[>50]&username=[len>5]&file=[img_*]"

	// Парсим правила один раз
	globalParams, urlRules, err := parser.parseRulesUnsafe(rules)
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}

	// Собираем все валидаторы один раз
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
	// Создаем валидаторы один раз
	allValidators := []func(string) bool{}

	// Comparison validators
	for _, constraint := range []string{">10", ">50", ">100", "<10", "<50", "<100"} {
		if v, err := comparisonPlugin.Parse("test_param", constraint); err == nil {
			allValidators = append(allValidators, v)
		}
	}

	// Length validators
	for _, constraint := range []string{"len>5", "len>10", "len<20", "len5..10", "len10..20"} {
		if v, err := lengthPlugin.Parse("test_param", constraint); err == nil {
			allValidators = append(allValidators, v)
		}
	}

	// Range validators
	for _, constraint := range []string{"1-10", "10-100", "100-1000", "-10..10", "0..100"} {
		if v, err := rangePlugin.Parse("test_param", constraint); err == nil {
			allValidators = append(allValidators, v)
		}
	}

	// Pattern validators
	for _, constraint := range []string{"*test*", "prefix*", "*suffix", "*a*b*", "start*end*"} {
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
				">-100",   // отрицательные
				">999999", // большие числа
				">",       // неполные
				">>10",    // двойные операторы
				">abc",    // текст вместо чисел
			},
		},
		{
			name:   "length_edge",
			plugin: lengthPlugin,
			constraints: []string{
				"len=0",     // нулевая длина
				"len>99999", // очень большие числа
				"len",       // неполные
				"len>>5",    // двойные операторы
				"len>abc",   // текст вместо чисел
			},
		},
		{
			name:   "range_edge",
			plugin: rangePlugin,
			constraints: []string{
				"0..0",      // одинаковые границы
				"-100..100", // отрицательные
				"10..5",     // min > max
				"1..999999", // очень большие числа
				"a..z",      // текст вместо чисел
			},
		},
		{
			name:   "pattern_edge",
			plugin: patternPlugin,
			constraints: []string{
				"*",       // только wildcard
				"**",      // multiple wildcards
				"",        // пустая строка
				"*.*+?[]", // специальные символы
				"*🎉*🚀*",   // unicode
			},
		},
	}

	// Создаем валидаторы один раз для всех валидных констрейнтов
	validators := make([]func(string) bool, 0)
	testValues := []string{"", "test", "123", "hello world"}

	for _, ec := range edgeCases {
		for _, constraint := range ec.constraints {
			// Пробуем создать валидатор, игнорируем ошибки
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

	// Реалистичные правила для API
	rules := `
		age=[18-65];
		/user/*?score=[>0]&level=[1-10]&username=[len3..20];
		/api/v1/*?token=[len=32]&limit=[1-100]&offset=[>=0];
		/products?price=[<10000]&category=[*_*]&status=[active,inactive];
		/search?q=[len1..100]&page=[1-100]&sort=[name,date,price];
	`

	err = pv.ParseRules(rules)
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}

	// Реалистичные тестовые URL
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

		// Основные операции валидации
		pv.ValidateURL(url)
		pv.NormalizeURL(url + "&invalid=param&extra=value")
	}
}

// Дополнительные оптимизированные бенчмарки

func BenchmarkPluginCanParseOnly(b *testing.B) {
	constraints := []string{
		">100", "len>5", "1-100", "*test*", "<50", "len5..15",
		"invalid", "length>=10", "prefix*", "18-65",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, constraint := range constraints {
			for _, pl := range allPlugins {
				_ = pl.plugin.CanParse(constraint)
			}
		}
	}
}

func BenchmarkPluginValidationOnly(b *testing.B) {
	// Создаем валидаторы один раз
	validators := make([]func(string) bool, 0, 10)

	// Добавляем по 2-3 валидатора каждого типа
	if v, err := comparisonPlugin.Parse("test", ">50"); err == nil {
		validators = append(validators, v)
	}
	if v, err := lengthPlugin.Parse("test", "len>5"); err == nil {
		validators = append(validators, v)
	}
	if v, err := rangePlugin.Parse("test", "1-100"); err == nil {
		validators = append(validators, v)
	}
	if v, err := patternPlugin.Parse("test", "*test*"); err == nil {
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
