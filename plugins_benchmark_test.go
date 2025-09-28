// plugins_benchmark_test.go
package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func BenchmarkComparisonPlugin(b *testing.B) {
	plugin := plugins.NewComparisonPlugin()
	validator, err := plugin.Parse("test", ">100")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator("150")
		validator("50")
	}
}

func BenchmarkLengthPlugin(b *testing.B) {
	plugin := plugins.NewLengthPlugin()

	// Тестируем разные типы констрейнтов с префиксом len
	benchmarks := []struct {
		name       string
		constraint string
		values     []string
	}{
		{
			name:       "len_operator_gt",
			constraint: "len>5",
			values:     []string{"hello!", "test", "very long string"},
		},
		{
			name:       "len_operator_gte",
			constraint: "len>=10",
			values:     []string{"short", "exactly ten", "this is definitely longer than ten"},
		},
		{
			name:       "len_operator_lt",
			constraint: "len<20",
			values:     []string{"short", "this is longer", "x"},
		},
		{
			name:       "len_operator_eq",
			constraint: "len=8",
			values:     []string{"12345678", "123", "1234567890"},
		},
		{
			name:       "len_operator_neq",
			constraint: "len!=5",
			values:     []string{"hi", "hello", "hello!"},
		},
		{
			name:       "len_range",
			constraint: "len5..15",
			values:     []string{"hello", "hello world!", "hi"},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			validator, err := plugin.Parse("test", bm.constraint)
			if err != nil {
				b.Fatalf("Failed to create validator for %s: %v", bm.constraint, err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, value := range bm.values {
					validator(value)
				}
			}
		})
	}
}

func BenchmarkLengthPluginParseOnly(b *testing.B) {
	plugin := plugins.NewLengthPlugin()
	constraints := []string{
		"len>5", "len>=10", "len<100", "len5..15", "len=8",
		"len<=20", "len!=3", "len>0", "len<255", "len1..10",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		constraint := constraints[i%len(constraints)]
		_, _ = plugin.Parse("test_param", constraint)
	}
}

func BenchmarkLengthPluginCanParse(b *testing.B) {
	plugin := plugins.NewLengthPlugin()

	inputs := []string{
		"len>5",      // valid
		"len>=10",    // valid
		"len5..15",   // valid
		"len=8",      // valid
		"length>=10", // invalid (alternative prefix)
		"size<100",   // invalid (alternative prefix)
		">=20",       // invalid (no len prefix)
		"5..15",      // invalid (no len prefix)
		"invalid",    // invalid
		"width>5",    // invalid
		"",           // invalid
		"len=",       // invalid (incomplete)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			_ = plugin.CanParse(input)
		}
	}
}

func BenchmarkRegexPlugin(b *testing.B) {
	plugin := plugins.NewRegexPlugin()
	validator, err := plugin.Parse("email", "/^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$/")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator("test@example.com")
		validator("invalid-email")
	}
}

func BenchmarkMixedPlugins(b *testing.B) {
	parser := NewRuleParser(
		plugins.NewComparisonPlugin(),
		plugins.NewLengthPlugin(),
		plugins.NewRegexPlugin(),
	)

	// Смешанные констрейнты для разных плагинов
	constraints := []string{
		">100",       // comparison
		"len>5",      // length
		"/^test$/",   // regex
		"<50",        // comparison
		"len5..15",   // length
		"len>=8",     // length
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

func BenchmarkPluginCanParse(b *testing.B) {
	parser := NewRuleParser(
		plugins.NewComparisonPlugin(),
		plugins.NewRegexPlugin(),
	)

	constraints := []string{
		">100",
		"<50",
		"/^test$/",
		"simple",
		"1-10",
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
