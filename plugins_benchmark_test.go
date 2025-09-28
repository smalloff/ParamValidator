// plugins_benchmark_test.go
package paramvalidator

import (
	"strings"
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func BenchmarkMixedPlugins(b *testing.B) {
	parser := NewRuleParser(
		plugins.NewComparisonPlugin(),
		plugins.NewLengthPlugin(),
		plugins.NewRangePlugin(),
		plugins.NewPatternPlugin(),
	)

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
			for _, plugin := range parser.plugins {
				_ = plugin.CanParse(constraint)
			}
		}
	}
}

func BenchmarkAllPluginsCanParse(b *testing.B) {
	plugins := []struct {
		name   string
		plugin interface {
			CanParse(constraintStr string) bool
		}
		constraints []string
	}{
		{
			name:        "comparison",
			plugin:      plugins.NewComparisonPlugin(),
			constraints: []string{">100", "<50", ">=10", "<=200", ">=-50"},
		},
		{
			name:        "length",
			plugin:      plugins.NewLengthPlugin(),
			constraints: []string{"len>5", "len<20", "len>=10", "len<=15", "len5..10"},
		},
		{
			name:        "range",
			plugin:      plugins.NewRangePlugin(),
			constraints: []string{"1-100", "18..65", "-10..10", "0-1000", "5..5"},
		},
		{
			name:        "pattern",
			plugin:      plugins.NewPatternPlugin(),
			constraints: []string{"*test*", "prefix*", "*suffix", "*a*b*c*", "start*end"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pl := range plugins {
			for _, constraint := range pl.constraints {
				_ = pl.plugin.CanParse(constraint)
			}
		}
	}
}

func BenchmarkAllPluginsParse(b *testing.B) {
	plugins := []struct {
		name   string
		plugin interface {
			Parse(paramName, constraintStr string) (func(string) bool, error)
		}
		constraints []string
	}{
		{
			name:        "comparison",
			plugin:      plugins.NewComparisonPlugin(),
			constraints: []string{">100", "<50", ">=10"},
		},
		{
			name:        "length",
			plugin:      plugins.NewLengthPlugin(),
			constraints: []string{"len>5", "len<20", "len5..10"},
		},
		{
			name:        "range",
			plugin:      plugins.NewRangePlugin(),
			constraints: []string{"1-100", "18..65", "-10..10"},
		},
		{
			name:        "pattern",
			plugin:      plugins.NewPatternPlugin(),
			constraints: []string{"*test*", "prefix*", "*suffix"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pl := range plugins {
			for _, constraint := range pl.constraints {
				validator, err := pl.plugin.Parse("test_param", constraint)
				if err == nil && validator != nil {
					_ = validator
				}
			}
		}
	}
}

func BenchmarkAllPluginsValidation(b *testing.B) {
	plugins := []struct {
		name   string
		plugin interface {
			Parse(paramName, constraintStr string) (func(string) bool, error)
		}
		constraint string
		testValues []string
	}{
		{
			name:       "comparison",
			plugin:     plugins.NewComparisonPlugin(),
			constraint: ">50",
			testValues: []string{"25", "50", "75", "100"},
		},
		{
			name:       "length",
			plugin:     plugins.NewLengthPlugin(),
			constraint: "len>5",
			testValues: []string{"hi", "hello", "hello!", "this is long"},
		},
		{
			name:       "range",
			plugin:     plugins.NewRangePlugin(),
			constraint: "1-100",
			testValues: []string{"0", "1", "50", "100", "101"},
		},
		{
			name:       "pattern",
			plugin:     plugins.NewPatternPlugin(),
			constraint: "*test*",
			testValues: []string{"hello", "test", "testing", "contest"},
		},
	}

	// Создаем валидаторы один раз
	validators := make([]func(string) bool, 0, len(plugins))
	for _, pl := range plugins {
		validator, err := pl.plugin.Parse("test_param", pl.constraint)
		if err == nil && validator != nil {
			validators = append(validators, validator)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, validator := range validators {
			for _, value := range plugins[0].testValues { // используем общие тестовые значения
				result := validator(value)
				_ = result
			}
		}
	}
}

func BenchmarkPluginIntegration(b *testing.B) {
	// Интеграционные тесты с полным парсером
	parser := NewRuleParser(
		plugins.NewComparisonPlugin(),
		plugins.NewLengthPlugin(),
		plugins.NewRangePlugin(),
		plugins.NewPatternPlugin(),
	)

	// Комплексные правила, использующие все плагины
	rules := []string{
		"/api?age=[18-65]&score=[>50]&username=[len>5]&file=[img_*]",
		"/users?level=[1-10]&status=[active,inactive]&name=[len3..20]&email=[*@*]",
		"/products?price=[<1000]&quantity=[1-100]&code=[len=6]&category=[*_*]",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, rule := range rules {
			_, _, err := parser.parseRulesUnsafe(rule)
			if err != nil {
				b.Logf("Rule parsing failed: %v", err)
			}
		}
	}
}

func BenchmarkPluginConcurrentUsage(b *testing.B) {
	parser := NewRuleParser(
		plugins.NewComparisonPlugin(),
		plugins.NewLengthPlugin(),
		plugins.NewRangePlugin(),
		plugins.NewPatternPlugin(),
	)

	rules := "/api?age=[18-65]&score=[>50]&username=[len>5]&file=[img_*]"

	// Парсим правила один раз
	globalParams, urlRules, err := parser.parseRulesUnsafe(rules)
	if err != nil {
		b.Fatalf("Failed to parse rules: %v", err)
	}

	// Собираем все валидаторы
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
	plugins := []struct {
		name   string
		plugin interface {
			Parse(paramName, constraintStr string) (func(string) bool, error)
		}
		constraints []string
	}{
		{
			name:        "comparison",
			plugin:      plugins.NewComparisonPlugin(),
			constraints: []string{">10", ">50", ">100", "<10", "<50", "<100"},
		},
		{
			name:        "length",
			plugin:      plugins.NewLengthPlugin(),
			constraints: []string{"len>5", "len>10", "len<20", "len5..10", "len10..20"},
		},
		{
			name:        "range",
			plugin:      plugins.NewRangePlugin(),
			constraints: []string{"1-10", "10-100", "100-1000", "-10..10", "0..100"},
		},
		{
			name:        "pattern",
			plugin:      plugins.NewPatternPlugin(),
			constraints: []string{"*test*", "prefix*", "*suffix", "*a*b*", "start*end*"},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Создаем множество валидаторов для измерения памяти
		allValidators := make([]func(string) bool, 0)

		for _, pl := range plugins {
			for _, constraint := range pl.constraints {
				validator, err := pl.plugin.Parse("test_param", constraint)
				if err == nil && validator != nil {
					allValidators = append(allValidators, validator)

					// Используем валидатор
					testValues := []string{"test", "25", "hello world", "prefix_value"}
					for _, value := range testValues {
						result := validator(value)
						_ = result
					}
				}
			}
		}

		_ = allValidators // предотвращаем оптимизацию
	}
}

func BenchmarkPluginEdgeCases(b *testing.B) {
	plugins := []struct {
		name   string
		plugin interface {
			CanParse(constraintStr string) bool
			Parse(paramName, constraintStr string) (func(string) bool, error)
		}
		constraints []string
	}{
		{
			name:   "comparison_edge",
			plugin: plugins.NewComparisonPlugin(),
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
			plugin: plugins.NewLengthPlugin(),
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
			plugin: plugins.NewRangePlugin(),
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
			plugin: plugins.NewPatternPlugin(),
			constraints: []string{
				"*",       // только wildcard
				"**",      // multiple wildcards
				"",        // пустая строка
				"*.*+?[]", // специальные символы
				"*🎉*🚀*",   // unicode
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pl := range plugins {
			for _, constraint := range pl.constraints {
				// Тестируем CanParse
				canParse := pl.plugin.CanParse(constraint)

				// Если может парситься, пробуем распарсить
				if canParse {
					validator, err := pl.plugin.Parse("test_param", constraint)
					if err == nil && validator != nil {
						// Тестируем на граничных значениях
						testValues := []string{"", "test", "123", strings.Repeat("x", 100)}
						for _, value := range testValues {
							result := validator(value)
							_ = result
						}
					}
				}
			}
		}
	}
}

func BenchmarkPluginRealWorldScenario(b *testing.B) {
	pv, err := NewParamValidator("", WithPlugins(plugins.NewComparisonPlugin(), plugins.NewLengthPlugin(), plugins.NewRangePlugin(), plugins.NewPatternPlugin()))
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
		pv.ValidateURL(url)
		pv.NormalizeURL(url + "&invalid=param&extra=value")
	}
}
