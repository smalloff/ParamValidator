package paramvalidator

import (
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func TestLengthPlugin(t *testing.T) {
	plugin := plugins.NewLengthPlugin()

	tests := []struct {
		name        string
		constraint  string
		value       string
		shouldParse bool
		expected    bool
		shouldError bool
	}{
		// Операторы с префиксом len
		{
			name:        "len greater than valid",
			constraint:  "len>5",
			value:       "hello!",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "len greater than invalid",
			constraint:  "len>5",
			value:       "hello",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "len greater than or equal valid",
			constraint:  "len>=5",
			value:       "hello",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "len greater than or equal invalid",
			constraint:  "len>=5",
			value:       "test",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "len less than valid",
			constraint:  "len<10",
			value:       "short",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "len less than invalid",
			constraint:  "len<10",
			value:       "this is too long",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "len less than or equal valid",
			constraint:  "len<=5",
			value:       "hello",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "len less than or equal invalid",
			constraint:  "len<=5",
			value:       "hello!",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "len equal valid",
			constraint:  "len=5",
			value:       "hello",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "len equal invalid",
			constraint:  "len=5",
			value:       "hi",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "len not equal valid",
			constraint:  "len!=5",
			value:       "hi",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "len not equal invalid",
			constraint:  "len!=5",
			value:       "hello",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},

		// Диапазоны
		{
			name:        "len range valid",
			constraint:  "len5..10",
			value:       "hello!",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "len range invalid",
			constraint:  "len5..10",
			value:       "hi",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "len range exact min",
			constraint:  "len5..10",
			value:       "hello",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "len range exact max",
			constraint:  "len5..10",
			value:       "hello worl",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},

		// Unicode строки
		{
			name:        "unicode string valid",
			constraint:  "len=3",
			value:       "при", // 3 символа
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "unicode string invalid",
			constraint:  "len=3",
			value:       "привет", // 6 символов ≠ 3
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "unicode string range valid",
			constraint:  "len2..4",
			value:       "世界", // 2 символа
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "unicode string range invalid",
			constraint:  "len2..4",
			value:       "世界你好", // 4 символа
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},

		// Граничные случаи
		{
			name:        "empty string with min length",
			constraint:  "len>=1",
			value:       "",
			shouldParse: true,
			expected:    false,
			shouldError: false,
		},
		{
			name:        "empty string with zero length",
			constraint:  "len=0",
			value:       "",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "empty string with range",
			constraint:  "len0..5",
			value:       "",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "very long string",
			constraint:  "len<1000",
			value:       "a",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "string with spaces",
			constraint:  "len=11",
			value:       "hello world",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},
		{
			name:        "string with special characters",
			constraint:  "len=5",
			value:       "a+b=c",
			shouldParse: true,
			expected:    true,
			shouldError: false,
		},

		// Ошибочные форматы
		{
			name:        "invalid format no number",
			constraint:  "len>",
			shouldParse: false, // CanParse returns false теперь
			shouldError: true,
		},
		{
			name:        "invalid format text",
			constraint:  "len>abc",
			shouldParse: false, // CanParse returns false теперь
			shouldError: true,
		},
		{
			name:        "invalid range format",
			constraint:  "len5..",
			shouldParse: false, // CanParse returns false теперь
			shouldError: true,
		},
		{
			name:        "invalid range min greater than max",
			constraint:  "len10..5",
			shouldParse: false, // CanParse returns false теперь
			shouldError: true,
		},
		{
			name:        "negative length",
			constraint:  "len>-5",
			shouldParse: false, // CanParse returns false теперь
			shouldError: true,
		},
		{
			name:        "empty constraint",
			constraint:  "",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "unsupported prefix",
			constraint:  "width>5",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "simple operator without len",
			constraint:  ">5",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "range without len",
			constraint:  "5..10",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "alternative prefix",
			constraint:  "length>5",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "double operator",
			constraint:  "len>>5",
			shouldParse: false, // CanParse returns false теперь
			shouldError: true,
		},
		{
			name:        "very large number",
			constraint:  "len>9999999999",
			shouldParse: true, // CanParse returns true (формат правильный)
			shouldError: true, // Но Parse должен упасть из-за диапазона
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test CanParse
			canParse := plugin.CanParse(tt.constraint)
			if canParse != tt.shouldParse {
				t.Errorf("CanParse(%q) = %v, expected %v",
					tt.constraint, canParse, tt.shouldParse)
				return
			}

			if !tt.shouldParse {
				// Если не должен парситься, проверяем что Parse возвращает ошибку
				_, err := plugin.Parse("test_param", tt.constraint)
				if err == nil {
					t.Errorf("Parse(%q) should fail for non-parsable constraint", tt.constraint)
				}
				return
			}

			// Test Parse
			validator, err := plugin.Parse("test_param", tt.constraint)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Parse(%q) should have failed but succeeded", tt.constraint)
				} else {
					t.Logf("Correctly got error for %q: %v", tt.constraint, err)
				}
				return
			} else {
				if err != nil {
					t.Errorf("Parse(%q) failed: %v", tt.constraint, err)
					return
				}
			}

			// Test validation
			result := validator(tt.value)
			if result != tt.expected {
				t.Errorf("Validator(%q) for constraint %q = %v, expected %v",
					tt.value, tt.constraint, result, tt.expected)
			}
		})
	}
}

func TestLengthPluginIntegration(t *testing.T) {
	// Создаем парсер и явно регистрируем плагин
	lengthPlugin := plugins.NewLengthPlugin()
	parser := NewRuleParser(lengthPlugin)

	tests := []struct {
		name     string
		rule     string
		value    string
		expected bool
	}{
		{
			name:     "len greater than in rule",
			rule:     "username=[len>5]",
			value:    "john_doe",
			expected: true,
		},
		{
			name:     "len greater than in rule too short",
			rule:     "username=[len>5]",
			value:    "john",
			expected: false,
		},
		{
			name:     "len less than in rule",
			rule:     "password=[len<20]",
			value:    "shortpass",
			expected: true,
		},
		{
			name:     "len less than in rule too long",
			rule:     "password=[len<20]",
			value:    "this_is_a_very_long_password",
			expected: false,
		},
		{
			name:     "len range in rule",
			rule:     "code=[len5..10]",
			value:    "123456",
			expected: true,
		},
		{
			name:     "len range in rule too short",
			rule:     "code=[len5..10]",
			value:    "123",
			expected: false,
		},
		{
			name:     "len range in rule too long",
			rule:     "code=[len5..10]",
			value:    "12345678901",
			expected: false,
		},
		{
			name:     "exact length in URL rule",
			rule:     "/api?token=[len=32]",
			value:    "abc123def456ghi789jkl012mno345pq",
			expected: true,
		},
		{
			name:     "exact length in URL rule wrong length",
			rule:     "/api?token=[len=32]",
			value:    "short",
			expected: false,
		},
		{
			name:     "not equal length in rule",
			rule:     "id=[len!=0]",
			value:    "123",
			expected: true,
		},
		{
			name:     "not equal length in rule empty",
			rule:     "id=[len!=0]",
			value:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Парсим полные правила
			globalParams, urlRules, err := parser.parseRulesUnsafe(tt.rule)
			if err != nil {
				t.Fatalf("Failed to parse rule %q: %v", tt.rule, err)
			}

			// Ищем правило параметра
			var paramRule *ParamRule
			for _, rule := range globalParams {
				if rule != nil {
					paramRule = rule
					break
				}
			}

			if paramRule == nil {
				// Проверяем URL rules
				for _, urlRule := range urlRules {
					for _, rule := range urlRule.Params {
						if rule != nil {
							paramRule = rule
							break
						}
					}
					if paramRule != nil {
						break
					}
				}
			}

			if paramRule == nil {
				t.Fatalf("No parameter rule found for %q", tt.rule)
			}

			if paramRule.Pattern != "plugin" {
				t.Errorf("Expected pattern 'plugin', got %q", paramRule.Pattern)
			}

			if paramRule.CustomValidator == nil {
				t.Fatal("CustomValidator should not be nil")
			}

			result := paramRule.CustomValidator(tt.value)
			if result != tt.expected {
				t.Errorf("Validation failed for value %q: got %v, expected %v",
					tt.value, result, tt.expected)
			}
		})
	}
}

func TestLengthEdgeCases(t *testing.T) {
	plugin := plugins.NewLengthPlugin()

	tests := []struct {
		name        string
		constraint  string
		shouldParse bool
		shouldError bool
	}{
		{
			name:        "valid len greater than",
			constraint:  "len>5",
			shouldParse: true,
			shouldError: false,
		},
		{
			name:        "valid len range",
			constraint:  "len5..10",
			shouldParse: true,
			shouldError: false,
		},
		{
			name:        "double operator should fail",
			constraint:  "len>>5",
			shouldParse: false, // CanParse returns false теперь
			shouldError: true,
		},
		{
			name:        "invalid range min greater than max",
			constraint:  "len10..5",
			shouldParse: false, // CanParse returns false теперь
			shouldError: true,
		},
		{
			name:        "negative length should fail",
			constraint:  "len>-5",
			shouldParse: false, // CanParse returns false теперь
			shouldError: true,
		},
		{
			name:        "empty after len should fail",
			constraint:  "len",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "very large number should fail",
			constraint:  "len>9999999999",
			shouldParse: true, // CanParse returns true (формат правильный)
			shouldError: true, // Но Parse должен упасть из-за диапазона
		},
		{
			name:        "invalid characters should fail",
			constraint:  "len>5abc",
			shouldParse: false, // CanParse returns false теперь
			shouldError: true,
		},
		{
			name:        "unsupported prefix should not parse",
			constraint:  "length>5",
			shouldParse: false,
			shouldError: true,
		},
		{
			name:        "simple operator without len should not parse",
			constraint:  ">5",
			shouldParse: false,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canParse := plugin.CanParse(tt.constraint)
			if canParse != tt.shouldParse {
				t.Errorf("CanParse(%q) = %v, expected %v",
					tt.constraint, canParse, tt.shouldParse)
			}

			_, err := plugin.Parse("test", tt.constraint)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Parse(%q) should fail but succeeded", tt.constraint)
				}
			} else {
				if err != nil {
					t.Errorf("Parse(%q) failed but should succeed: %v", tt.constraint, err)
				}
			}
		})
	}
}

// Бенчмарки остаются без изменений...
func BenchmarkLengthPlugin(b *testing.B) {
	plugin := plugins.NewLengthPlugin()
	validator, err := plugin.Parse("test", "len>5")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator("hello!")
	}
}

func BenchmarkLengthPluginCanParse(b *testing.B) {
	plugin := plugins.NewLengthPlugin()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.CanParse("len>5")
	}
}

func BenchmarkLengthPluginParse(b *testing.B) {
	plugin := plugins.NewLengthPlugin()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.Parse("test", "len>5")
	}
}

func BenchmarkLengthPluginNormalization(b *testing.B) {
	lengthPlugin := plugins.NewLengthPlugin()
	pv, err := NewParamValidator("", WithPlugins(lengthPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?username=[len>5]&code=[len5..10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterURL("/api?username=john_doe&code=123456&invalid=value")
	}
}

func BenchmarkLengthPluginFilterQuery(b *testing.B) {
	lengthPlugin := plugins.NewLengthPlugin()
	pv, err := NewParamValidator("", WithPlugins(lengthPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?username=[len>5]&code=[len5..10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.FilterQuery("/api", "username=john_doe&code=123456&invalid=value")
	}
}

func BenchmarkLengthPluginValidateQuery(b *testing.B) {
	lengthPlugin := plugins.NewLengthPlugin()
	pv, err := NewParamValidator("", WithPlugins(lengthPlugin))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}
	err = pv.ParseRules("/api?username=[len>5]&code=[len5..10]")
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.ValidateQuery("/api", "username=john_doe&code=123456&invalid=value")
	}
}
