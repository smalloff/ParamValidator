package paramvalidator

import (
	"strings"
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

// BenchmarkPatternPluginSecurity бенчмарки для паттерн-плагина
func BenchmarkPatternPluginSecurity(b *testing.B) {
	plugin := plugins.NewPatternPlugin()

	benchmarks := []struct {
		name    string
		pattern string
		value   string
	}{
		{
			name:    "Simple wildcard",
			pattern: "*test*",
			value:   "this is a test value",
		},
		{
			name:    "Multiple wildcards",
			pattern: "*a*b*c*d*",
			value:   strings.Repeat("x", 1000),
		},
		{
			name:    "Prefix suffix",
			pattern: "prefix*suffix",
			value:   "prefix_middle_suffix",
		},
		{
			name:    "Complex pattern",
			pattern: "*abc*def*ghi*",
			value:   strings.Repeat("abc def ghi ", 100),
		},
		{
			name:    "Long pattern",
			pattern: strings.Repeat("a", 100) + "*",
			value:   strings.Repeat("a", 100) + "suffix",
		},
		{
			name:    "Unicode pattern",
			pattern: "*🎉*🚀*",
			value:   "start🎉middle🚀end",
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			validator, err := plugin.Parse("test", bm.pattern)
			if err != nil {
				b.Skipf("Failed to create validator: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result := validator(bm.value)
				_ = result
			}
		})
	}
}

// BenchmarkPatternPluginReDoSProtection бенчмарки для защиты от ReDoS
func BenchmarkPatternPluginReDoSProtection(b *testing.B) {
	plugin := plugins.NewPatternPlugin()

	redosPatterns := []struct {
		name    string
		pattern string
		value   string
	}{
		{
			name:    "Exponential backtracking",
			pattern: "*a*b*c*d*e*f*g*h*i*j*",
			value:   strings.Repeat("x", 500),
		},
		{
			name:    "Many wildcards",
			pattern: strings.Repeat("*", 50),
			value:   strings.Repeat("test", 100),
		},
		{
			name:    "Complex overlaps",
			pattern: "*abc*abc*abc*abc*abc*",
			value:   strings.Repeat("abc", 300),
		},
		{
			name:    "Long prefix many wildcards",
			pattern: strings.Repeat("a", 50) + strings.Repeat("*", 10),
			value:   strings.Repeat("a", 50) + strings.Repeat("b", 500),
		},
	}

	for _, bp := range redosPatterns {
		b.Run(bp.name, func(b *testing.B) {
			validator, err := plugin.Parse("test", bp.pattern)
			if err != nil {
				b.Skipf("Failed to create validator: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result := validator(bp.value)
				_ = result
			}
		})
	}
}

// BenchmarkLengthPluginSecurity бенчмарки для length-плагина
func BenchmarkLengthPluginSecurity(b *testing.B) {
	plugin := plugins.NewLengthPlugin()

	benchmarks := []struct {
		name       string
		constraint string
		values     []string
	}{
		{
			name:       "Simple greater than",
			constraint: "len>10",
			values:     []string{"short", "this is long enough", strings.Repeat("x", 1000)},
		},
		{
			name:       "Range constraint",
			constraint: "len5..50",
			values:     []string{"short", "perfect length string", strings.Repeat("x", 100)},
		},
		{
			name:       "Complex operator",
			constraint: "len>=100",
			values:     []string{strings.Repeat("x", 50), strings.Repeat("x", 100), strings.Repeat("x", 150)},
		},
		{
			name:       "Not equal",
			constraint: "len!=0",
			values:     []string{"", "not empty", strings.Repeat("x", 100)},
		},
		{
			name:       "Exact length",
			constraint: "len=25",
			values:     []string{strings.Repeat("x", 24), strings.Repeat("x", 25), strings.Repeat("x", 26)},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			validator, err := plugin.Parse("test", bm.constraint)
			if err != nil {
				b.Skipf("Failed to create validator: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				for _, value := range bm.values {
					result := validator(value)
					_ = result
				}
			}
		})
	}
}

// BenchmarkComparisonPluginSecurity бенчмарки для comparison-плагина
func BenchmarkComparisonPluginSecurity(b *testing.B) {
	plugin := plugins.NewComparisonPlugin()

	benchmarks := []struct {
		name       string
		constraint string
		values     []string
	}{
		{
			name:       "Greater than",
			constraint: ">100",
			values:     []string{"50", "100", "150", "999999"},
		},
		{
			name:       "Less or equal",
			constraint: "<=50",
			values:     []string{"0", "50", "51", "-10"},
		},
		{
			name:       "Negative numbers",
			constraint: ">=-100",
			values:     []string{"-200", "-100", "0", "100"},
		},
		{
			name:       "Large numbers",
			constraint: "<1000000",
			values:     []string{"999999", "1000000", "1000001"},
		},
		{
			name:       "Equal boundary",
			constraint: ">=0",
			values:     []string{"-1", "0", "1"},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			validator, err := plugin.Parse("test", bm.constraint)
			if err != nil {
				b.Skipf("Failed to create validator: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				for _, value := range bm.values {
					result := validator(value)
					_ = result
				}
			}
		})
	}
}

// BenchmarkRangePluginSecurity бенчмарки для range-плагина
func BenchmarkRangePluginSecurity(b *testing.B) {
	plugin := plugins.NewRangePlugin()

	benchmarks := []struct {
		name       string
		constraint string
		values     []string
	}{
		{
			name:       "Simple range",
			constraint: "1..100",
			values:     []string{"0", "1", "50", "100", "101"},
		},
		{
			name:       "Negative range",
			constraint: "-50..50",
			values:     []string{"-51", "-50", "0", "50", "51"},
		},
		{
			name:       "Large range",
			constraint: "1000..10000",
			values:     []string{"999", "1000", "5000", "10000", "10001"},
		},
		{
			name:       "Single value range",
			constraint: "42..42",
			values:     []string{"41", "42", "43"},
		},
		{
			name:       "Dash separator",
			constraint: "1-100",
			values:     []string{"0", "1", "50", "100", "101"},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			validator, err := plugin.Parse("test", bm.constraint)
			if err != nil {
				b.Skipf("Failed to create validator: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				for _, value := range bm.values {
					result := validator(value)
					_ = result
				}
			}
		})
	}
}

// BenchmarkPluginInputValidation бенчмарки валидации входных данных
func BenchmarkPluginInputValidation(b *testing.B) {
	plugins := []struct {
		name   string
		plugin interface {
			CanParse(constraintStr string) bool
			Parse(paramName, constraintStr string) (func(string) bool, error)
			GetName() string
		}
		constraints []string
	}{
		{
			name:        "pattern",
			plugin:      plugins.NewPatternPlugin(),
			constraints: []string{"*test*", "prefix*", "*suffix", "*a*b*c*", strings.Repeat("x", 100) + "*"},
		},
		{
			name:        "length",
			plugin:      plugins.NewLengthPlugin(),
			constraints: []string{"len>5", "len10..50", "len!=0", "len=25", "len>=100"},
		},
		{
			name:        "comparison",
			plugin:      plugins.NewComparisonPlugin(),
			constraints: []string{">10", "<=50", ">=-100", "<1000", ">=0"},
		},
		{
			name:        "range",
			plugin:      plugins.NewRangePlugin(),
			constraints: []string{"1..100", "-50..50", "1000..10000", "42..42", "1-100"},
		},
	}

	testValues := []string{"", "test", "12345", strings.Repeat("x", 100), "invalid-value"}

	for _, pl := range plugins {
		b.Run(pl.name+"_creation", func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				for _, constraint := range pl.constraints {
					validator, err := pl.plugin.Parse("test_param", constraint)
					if err == nil && validator != nil {
						_ = validator
					}
				}
			}
		})

		b.Run(pl.name+"_validation", func(b *testing.B) {
			validators := make([]func(string) bool, 0, len(pl.constraints))

			// Создаем валидаторы один раз
			for _, constraint := range pl.constraints {
				validator, err := pl.plugin.Parse("test_param", constraint)
				if err == nil && validator != nil {
					validators = append(validators, validator)
				}
			}

			if len(validators) == 0 {
				b.Skip("No valid validators created")
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				for _, validator := range validators {
					for _, value := range testValues {
						result := validator(value)
						_ = result
					}
				}
			}
		})
	}
}

// BenchmarkPluginMemorySafety бенчмарки безопасности памяти
func BenchmarkPluginMemorySafety(b *testing.B) {
	plugin := plugins.NewPatternPlugin()
	validator, err := plugin.Parse("test", "*test*")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	// Предотвращаем кэширование
	memoryTests := []struct {
		name  string
		value func(int) string // Генератор значений
	}{
		{
			"Large",
			func(i int) string {
				return strings.Repeat("x", 10000) + string(rune(i%26+97))
			},
		},
		{
			"ManyMatches",
			func(i int) string {
				return strings.Repeat("test", 1000) + string(rune(i%26+97))
			},
		},
	}

	for _, mt := range memoryTests {
		b.Run(mt.name, func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				value := mt.value(i) // Уникальное значение каждый раз
				result := validator(value)
				_ = result
			}
		})
	}
}

// BenchmarkPluginBoundaryConditions бенчмарки граничных условий
func BenchmarkPluginBoundaryConditions(b *testing.B) {
	plugins := []struct {
		name   string
		plugin interface {
			Parse(paramName, constraintStr string) (func(string) bool, error)
		}
		constraint string
	}{
		{"pattern", plugins.NewPatternPlugin(), "*test*"},
		{"length", plugins.NewLengthPlugin(), "len>5"},
		{"comparison", plugins.NewComparisonPlugin(), ">10"},
		{"range", plugins.NewRangePlugin(), "1..100"},
	}

	// Только критические boundary values
	boundaryValues := []string{
		"",           // empty
		"0",          // zero
		"test",       // normal
		"1234567890", // max length number
	}

	for _, pl := range plugins {
		b.Run(pl.name, func(b *testing.B) {
			validator, err := pl.plugin.Parse("test", pl.constraint)
			if err != nil {
				b.Skipf("Failed to create validator: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Тестируем только 2 значения за итерацию
				result1 := validator(boundaryValues[i%2])
				result2 := validator(boundaryValues[(i+1)%2])
				_ = result1
				_ = result2
			}
		})
	}
}

// BenchmarkPluginConcurrentSafety бенчмарки конкурентной безопасности
func BenchmarkPluginConcurrentSafety(b *testing.B) {
	plugin := plugins.NewPatternPlugin()
	validator, err := plugin.Parse("test", "*test*")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	testValues := []string{
		"",
		"test",
		"no match here",
		"this has test in it",
		strings.Repeat("test ", 100),
		strings.Repeat("x", 1000),
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, value := range testValues {
				result := validator(value)
				_ = result
			}
		}
	})
}

// BenchmarkPluginResourceCleanup бенчмарки очистки ресурсов
func BenchmarkPluginResourceCleanup(b *testing.B) {
	plugins := []struct {
		name   string
		plugin interface {
			Parse(paramName, constraintStr string) (func(string) bool, error)
		}
		constraints []string
	}{
		{
			name:   "pattern",
			plugin: plugins.NewPatternPlugin(),
			constraints: []string{
				"*test*", "prefix*", "*suffix", "*a*b*c*",
				strings.Repeat("x", 50) + "*", "*" + strings.Repeat("y", 50),
			},
		},
		{
			name:        "length",
			plugin:      plugins.NewLengthPlugin(),
			constraints: []string{"len>5", "len10..50", "len!=0", "len=25"},
		},
	}

	testValue := "this is a test value of moderate length"

	for _, pl := range plugins {
		b.Run(pl.name+"_recreate", func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				for _, constraint := range pl.constraints {
					validator, err := pl.plugin.Parse("test", constraint)
					if err == nil && validator != nil {
						result := validator(testValue)
						_ = result
					}
				}
			}
		})
	}
}
