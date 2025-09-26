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
