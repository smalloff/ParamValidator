package paramvalidator

import (
	"strings"
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func BenchmarkPatternPluginSecurity(b *testing.B) {
	plugin := plugins.NewPatternPlugin()

	benchmarks := []struct {
		name    string
		pattern string
		value   string
	}{
		{
			name:    "Simple wildcard",
			pattern: "in:*test*",
			value:   "this is a test value",
		},
		{
			name:    "Multiple wildcards",
			pattern: "in:*a*b*c*d*",
			value:   strings.Repeat("x", 1000),
		},
		{
			name:    "Prefix suffix",
			pattern: "in:prefix*suffix",
			value:   "prefix_middle_suffix",
		},
		{
			name:    "Complex pattern",
			pattern: "in:*abc*def*ghi*",
			value:   strings.Repeat("abc def ghi ", 100),
		},
		{
			name:    "Long pattern",
			pattern: "in:" + strings.Repeat("a", 100) + "*",
			value:   strings.Repeat("a", 100) + "suffix",
		},
		{
			name:    "Unicode pattern",
			pattern: "in:*🎉*🚀*",
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

func BenchmarkPatternPluginReDoSProtection(b *testing.B) {
	plugin := plugins.NewPatternPlugin()

	redosPatterns := []struct {
		name    string
		pattern string
		value   string
	}{
		{
			name:    "Exponential backtracking",
			pattern: "in:*a*b*c*d*e*f*g*h*i*j*",
			value:   strings.Repeat("x", 500),
		},
		{
			name:    "Many wildcards",
			pattern: "in:" + strings.Repeat("*", 50),
			value:   strings.Repeat("test", 100),
		},
		{
			name:    "Complex overlaps",
			pattern: "in:*abc*abc*abc*abc*abc*",
			value:   strings.Repeat("abc", 300),
		},
		{
			name:    "Long prefix many wildcards",
			pattern: "in:" + strings.Repeat("a", 50) + strings.Repeat("*", 10),
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

func BenchmarkLengthPluginSecurity(b *testing.B) {
	plugin := plugins.NewLengthPlugin()

	benchmarks := []struct {
		name       string
		constraint string
		values     []string
	}{
		{
			name:       "Simple greater than",
			constraint: "len:>10",
			values:     []string{"short", "this is long enough", strings.Repeat("x", 1000)},
		},
		{
			name:       "Range constraint",
			constraint: "len:5..50",
			values:     []string{"short", "perfect length string", strings.Repeat("x", 100)},
		},
		{
			name:       "Complex operator",
			constraint: "len:>=100",
			values:     []string{strings.Repeat("x", 50), strings.Repeat("x", 100), strings.Repeat("x", 150)},
		},
		{
			name:       "Not equal",
			constraint: "len:!=0",
			values:     []string{"", "not empty", strings.Repeat("x", 100)},
		},
		{
			name:       "Exact length",
			constraint: "len:25",
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

func BenchmarkComparisonPluginSecurity(b *testing.B) {
	plugin := plugins.NewComparisonPlugin()

	benchmarks := []struct {
		name       string
		constraint string
		values     []string
	}{
		{
			name:       "Greater than",
			constraint: "cmp:>100",
			values:     []string{"50", "100", "150", "999999"},
		},
		{
			name:       "Less or equal",
			constraint: "cmp:<=50",
			values:     []string{"0", "50", "51", "-10"},
		},
		{
			name:       "Negative numbers",
			constraint: "cmp:>=-100",
			values:     []string{"-200", "-100", "0", "100"},
		},
		{
			name:       "Large numbers",
			constraint: "cmp:<1000000",
			values:     []string{"999999", "1000000", "1000001"},
		},
		{
			name:       "Equal boundary",
			constraint: "cmp:>=0",
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

func BenchmarkRangePluginSecurity(b *testing.B) {
	plugin := plugins.NewRangePlugin()

	benchmarks := []struct {
		name       string
		constraint string
		values     []string
	}{
		{
			name:       "Simple range",
			constraint: "range:1..100",
			values:     []string{"0", "1", "50", "100", "101"},
		},
		{
			name:       "Negative range",
			constraint: "range:-50..50",
			values:     []string{"-51", "-50", "0", "50", "51"},
		},
		{
			name:       "Large range",
			constraint: "range:1000..10000",
			values:     []string{"999", "1000", "5000", "10000", "10001"},
		},
		{
			name:       "Single value range",
			constraint: "range:42..42",
			values:     []string{"41", "42", "43"},
		},
		{
			name:       "Dash separator",
			constraint: "range:1-100",
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

func BenchmarkPluginInputValidation(b *testing.B) {
	plugins := []struct {
		name   string
		plugin interface {
			Parse(paramName, constraintStr string) (func(string) bool, error)
		}
		constraints []string
	}{
		{
			name:        "pattern",
			plugin:      plugins.NewPatternPlugin(),
			constraints: []string{"in:*test*", "in:prefix*", "in:*suffix", "in:*a*b*c*", "in:" + strings.Repeat("x", 100) + "*"},
		},
		{
			name:        "length",
			plugin:      plugins.NewLengthPlugin(),
			constraints: []string{"len:>5", "len:10..50", "len:!=0", "len:25", "len:>=100"},
		},
		{
			name:        "comparison",
			plugin:      plugins.NewComparisonPlugin(),
			constraints: []string{"cmp:>10", "cmp:<=50", "cmp:>=-100", "cmp:<1000", "cmp:>=0"},
		},
		{
			name:        "range",
			plugin:      plugins.NewRangePlugin(),
			constraints: []string{"range:1..100", "range:-50..50", "range:1000..10000", "range:42..42", "range:1-100"},
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

func BenchmarkPluginMemorySafety(b *testing.B) {
	plugin := plugins.NewPatternPlugin()
	validator, err := plugin.Parse("test", "in:*test*")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	memoryTests := []struct {
		name  string
		value func(int) string
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
				value := mt.value(i)
				result := validator(value)
				_ = result
			}
		})
	}
}

func BenchmarkPluginBoundaryConditions(b *testing.B) {
	plugins := []struct {
		name   string
		plugin interface {
			Parse(paramName, constraintStr string) (func(string) bool, error)
		}
		constraint string
	}{
		{"pattern", plugins.NewPatternPlugin(), "in:*test*"},
		{"length", plugins.NewLengthPlugin(), "len:>5"},
		{"comparison", plugins.NewComparisonPlugin(), "cmp:>10"},
		{"range", plugins.NewRangePlugin(), "range:1..100"},
	}

	boundaryValues := []string{
		"",
		"0",
		"test",
		"1234567890",
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
				for _, value := range boundaryValues {
					result := validator(value)
					_ = result
				}
			}
		})
	}
}

func BenchmarkPluginConcurrentSafety(b *testing.B) {
	plugin := plugins.NewPatternPlugin()
	validator, err := plugin.Parse("test", "in:*test*")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	testValues := []string{"", "no", "this is a test value", strings.Repeat("x", 100)}

	b.Run("SingleValidator", func(b *testing.B) {
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
	})

	b.Run("MultipleValidators", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				localValidator, err := plugin.Parse("test", "in:*test*")
				if err != nil {
					b.Fatalf("Failed to create validator: %v", err)
				}

				for _, value := range testValues {
					result := localValidator(value)
					_ = result
				}
			}
		})
	})
}

func BenchmarkPluginSecurityEdgeCases(b *testing.B) {
	plugin := plugins.NewPatternPlugin()

	edgeCases := []struct {
		name    string
		pattern string
		value   string
	}{
		{
			name:    "Empty pattern",
			pattern: "in:",
			value:   "any value",
		},
		{
			name:    "Only wildcard",
			pattern: "in:*",
			value:   strings.Repeat("x", 10000),
		},
		{
			name:    "Multiple consecutive wildcards",
			pattern: "in:***",
			value:   "test",
		},
		{
			name:    "Very long pattern",
			pattern: "in:" + strings.Repeat("abc", 1000) + "*",
			value:   strings.Repeat("abc", 1000) + "suffix",
		},
		{
			name:    "Special regex characters",
			pattern: "in:*.*+?[]{}()|^$\\*",
			value:   "text.*+?[]{}()|^$\\*end",
		},
	}

	for _, ec := range edgeCases {
		b.Run(ec.name, func(b *testing.B) {
			validator, err := plugin.Parse("test", ec.pattern)
			if err != nil {
				b.Skipf("Failed to create validator: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result := validator(ec.value)
				_ = result
			}
		})
	}
}
