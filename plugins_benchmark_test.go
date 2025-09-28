// plugins_benchmark_test.go
package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func BenchmarkMixedPlugins(b *testing.B) {
	parser := NewRuleParser(
		plugins.NewComparisonPlugin(),
		plugins.NewLengthPlugin(),
		plugins.NewRangePlugin(),
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
