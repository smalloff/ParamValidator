package paramvalidator

import (
	"strings"
	"testing"

	"github.com/smalloff/paramvalidator/plugins"
)

func TestCheckRules(t *testing.T) {
	tests := []struct {
		name      string
		rulesStr  string
		wantError bool
	}{
		{
			name:      "empty rules",
			rulesStr:  "",
			wantError: false,
		},
		{
			name:      "valid global rules",
			rulesStr:  "page=[1]&limit=[5,10,20]",
			wantError: false,
		},
		{
			name:      "valid URL rules",
			rulesStr:  "/products?page=[1];/users?limit=[5,10,20]",
			wantError: false,
		},
		{
			name:      "valid enum with multiple values",
			rulesStr:  "sort=[name,date,price]&status=[active,inactive]",
			wantError: false,
		},
		{
			name:      "valid key-only parameter",
			rulesStr:  "active=[]&debug=[]",
			wantError: false,
		},
		{
			name:      "valid callback parameter",
			rulesStr:  "token=[?]&signature=[?]",
			wantError: false,
		},
		{
			name:      "valid wildcard parameters - allow all",
			rulesStr:  "/api/*?*",
			wantError: false,
		},
		{
			name:      "valid comparison plugin rules",
			rulesStr:  "age=[cmp:>18]&score=[cmp:>=50]&price=[cmp:<1000]", // Добавлен cmp:
			wantError: false,
		},
		{
			name:      "valid length plugin rules",
			rulesStr:  "username=[len:>5]&password=[len:8..20]&token=[len:32]",
			wantError: false,
		},
		{
			name:      "valid range plugin rules",
			rulesStr:  "level=[range:1-10]&percentage=[range:0-100]&temp=[range:-20..40]",
			wantError: false,
		},
		{
			name:      "valid pattern plugin rules",
			rulesStr:  "file=[in:*test*]&email=[in:*@*]&category=[in:*_*]",
			wantError: false,
		},
		{
			name:      "mixed plugin and standard rules",
			rulesStr:  "page=[1]&age=[cmp:>18]&username=[len:>5]&category=[in:*_*]&status=[active]", // Добавлен cmp:
			wantError: false,
		},
		{
			name:      "complex URL rules with plugins",
			rulesStr:  "/api/*?age=[range:18-65]&score=[cmp:>50];/users?username=[len:3..20]&email=[in:*@*]", // Добавлен cmp:
			wantError: false,
		},
		{
			name:      "invalid rules - unclosed bracket",
			rulesStr:  "page=[1&limit=[5]",
			wantError: true,
		},
		{
			name:      "invalid rules - empty parameter name",
			rulesStr:  "=[1]&limit=[5]",
			wantError: true,
		},
		{
			name:      "rules too large",
			rulesStr:  strings.Repeat("param=[value]&", 1000),
			wantError: true,
		},
		// Тесты с некорректными плагинными правилами
		{
			name:      "invalid comparison plugin - missing number",
			rulesStr:  "age=[cmp:>]",
			wantError: true, // Теперь должна быть ошибка синтаксиса
		},
		{
			name:      "invalid length plugin - missing number",
			rulesStr:  "username=[len:>]",
			wantError: true, // Теперь должна быть ошибка синтаксиса
		},
		{
			name:      "invalid range plugin - missing max value",
			rulesStr:  "level=[range:10-]",
			wantError: true, // Теперь должна быть ошибка синтаксиса
		},
		{
			name:      "invalid pattern plugin - missing wildcard",
			rulesStr:  "file=[in:test]",
			wantError: true, // Ошибка синтаксиса
		},
		// Тесты с невалидными плагинными операторами - обрабатываются как enum
		{
			name:      "double comparison operator - treated as enum",
			rulesStr:  "age=[cmp:>>18]",
			wantError: true, // "not for this plugin" → обрабатывается как enum
		},
		{
			name:      "double length operator - treated as enum",
			rulesStr:  "username=[len:>>5]",
			wantError: true, // "not for this plugin" → обрабатывается как enum
		},
		{
			name:      "invalid range - min > max",
			rulesStr:  "level=[range:10-5]",
			wantError: true, // Ошибка синтаксиса (валидный префикс, но некорректные значения)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv, err := NewParamValidator("", WithPlugins(
				plugins.NewComparisonPlugin(),
				plugins.NewLengthPlugin(),
				plugins.NewRangePlugin(),
				plugins.NewPatternPlugin(),
			))
			if err != nil {
				t.Fatalf("Failed to create validator: %v", err)
			}

			err = pv.CheckRules(tt.rulesStr)

			if tt.wantError {
				if err == nil {
					t.Errorf("CheckRules() expected error for rules %q, but got nil", tt.rulesStr)
				} else {
					t.Logf("CheckRules() correctly returned error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("CheckRules() unexpected error = %v for rules %q", err, tt.rulesStr)
				}
			}
		})
	}
}

func TestCheckRulesWithRealPlugins(t *testing.T) {
	comparisonPlugin := plugins.NewComparisonPlugin()
	lengthPlugin := plugins.NewLengthPlugin()
	rangePlugin := plugins.NewRangePlugin()
	patternPlugin := plugins.NewPatternPlugin()

	allPlugins := []PluginConstraintParser{
		comparisonPlugin,
		lengthPlugin,
		rangePlugin,
		patternPlugin,
	}

	tests := []struct {
		name      string
		rulesStr  string
		wantError bool
	}{
		{
			name:      "valid comparison plugin constraints",
			rulesStr:  "age=[cmp:>18]&score=[cmp:>=50]&price=[cmp:<1000]", // Добавлен cmp:
			wantError: false,
		},
		{
			name:      "valid length plugin constraints",
			rulesStr:  "username=[len:>5]&password=[len:8..20]&token=[len:32]",
			wantError: false,
		},
		{
			name:      "valid range plugin constraints",
			rulesStr:  "level=[range:1-10]&percentage=[range:0-100]",
			wantError: false,
		},
		{
			name:      "valid pattern plugin constraints",
			rulesStr:  "file=[in:*test*]&email=[in:*@*]",
			wantError: false,
		},
		{
			name:      "mixed plugins in URL rules",
			rulesStr:  "/api/*?age=[cmp:>18]&username=[len:>5];/users?level=[range:1-10]&email=[in:*@*]", // Добавлен cmp:
			wantError: false,
		},
		{
			name:      "invalid comparison constraint - double operator",
			rulesStr:  "age=[cmp:>>18]", // Добавлен cmp:
			wantError: true,             // "not for this plugin" → обрабатывается как enum
		},
		{
			name:      "invalid length constraint - double operator",
			rulesStr:  "username=[len:>>5]",
			wantError: true, // "not for this plugin" → обрабатывается как enum
		},
		{
			name:      "invalid range constraint - min > max",
			rulesStr:  "level=[range:10-5]",
			wantError: true, // Ошибка синтаксиса (валидный префикс, но некорректные значения)
		},
		{
			name:      "unknown plugin constraint - handled as enum",
			rulesStr:  "param=[unknown:format]",
			wantError: false, // Обрабатывается как enum
		},
		{
			name:      "out of range comparison",
			rulesStr:  "age=[cmp:>9999999999]", // Добавлен cmp:
			wantError: true,                    // Ошибка синтаксиса (число вне диапазона)
		},
		{
			name:      "out of range length",
			rulesStr:  "username=[len:>9999999999]",
			wantError: true, // Ошибка синтаксиса (число вне диапазона)
		},
		{
			name:      "out of range range",
			rulesStr:  "level=[range:1-9999999999]",
			wantError: true, // Ошибка синтаксиса (число вне диапазона)
		},
		// Тесты, которые ДОЛЖНЫ вызывать ошибки
		{
			name:      "invalid syntax - unclosed bracket",
			rulesStr:  "age=[cmp:>18", // Добавлен cmp:
			wantError: true,           // Должна быть ошибка синтаксиса
		},
		{
			name:      "invalid parameter name",
			rulesStr:  "=[]",
			wantError: true, // Должна быть ошибка имени параметра
		},
		{
			name:      "valid enum with invalid plugin format",
			rulesStr:  "param=[>>18]", // Это enum, а не плагин
			wantError: false,          // Валидный enum
		},
		{
			name:      "valid enum with unknown plugin prefix",
			rulesStr:  "param=[unknown:value]",
			wantError: false, // Валидный enum
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckRulesStaticWithPlugins(tt.rulesStr, allPlugins)

			if tt.wantError {
				if err == nil {
					t.Errorf("CheckRulesStaticWithPlugins() expected error for rules %q, but got nil", tt.rulesStr)
				} else {
					t.Logf("CheckRulesStaticWithPlugins() correctly returned error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("CheckRulesStaticWithPlugins() unexpected error = %v for rules %q", err, tt.rulesStr)
				}
			}
		})
	}
}

func TestCheckRulesIntegration(t *testing.T) {
	pv, err := NewParamValidator("", WithPlugins(
		plugins.NewComparisonPlugin(),
		plugins.NewLengthPlugin(),
		plugins.NewRangePlugin(),
		plugins.NewPatternPlugin(),
	))
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name            string
		rulesStr        string
		shouldCheckPass bool
		shouldParsePass bool
	}{
		{
			name:            "consistent valid rules",
			rulesStr:        "age=[cmp:>18]&username=[len:>5]", // Добавлен cmp:
			shouldCheckPass: true,
			shouldParsePass: true,
		},
		{
			name:            "invalid plugin constraint - syntax error",
			rulesStr:        "age=[cmp:>]", // Добавлен cmp: и нет числа после оператора
			shouldCheckPass: false,         // Ошибка синтаксиса
			shouldParsePass: false,         // Ошибка синтаксиса
		},
		{
			name:            "valid enum with double operator",
			rulesStr:        "param=[>>18]", // Это enum, а не плагин
			shouldCheckPass: true,           // Валидный enum для CheckRules
			shouldParsePass: true,           // Валидный enum для ParseRules
		},
		{
			name:            "URL rules consistency",
			rulesStr:        "/api/*?age=[cmp:>18]&username=[len:>5]", // Добавлен cmp:
			shouldCheckPass: true,
			shouldParsePass: true,
		},
		{
			name:            "invalid plugin syntax in URL",
			rulesStr:        "/api/*?age=[cmp:>]", // Добавлен cmp: и нет числа после оператора
			shouldCheckPass: false,                // Ошибка синтаксиса
			shouldParsePass: false,                // Ошибка синтаксиса
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Проверяем синтаксис через CheckRules
			checkErr := pv.CheckRules(tt.rulesStr)

			if tt.shouldCheckPass && checkErr != nil {
				t.Errorf("CheckRules() failed but should pass: %v", checkErr)
			}
			if !tt.shouldCheckPass && checkErr == nil {
				t.Errorf("CheckRules() passed but should fail for rules: %q", tt.rulesStr)
			}

			// Проверяем загрузку через ParseRules
			parseErr := pv.ParseRules(tt.rulesStr)

			if tt.shouldParsePass && parseErr != nil {
				t.Errorf("ParseRules() failed but should pass: %v", parseErr)
			}
			if !tt.shouldParsePass && parseErr == nil {
				t.Errorf("ParseRules() passed but should fail for rules: %q", tt.rulesStr)
			}

			t.Logf("Rules: %q, CheckRules error: %v, ParseRules error: %v", tt.rulesStr, checkErr, parseErr)
		})
	}
}

func TestCheckRulesEdgeCases(t *testing.T) {
	pv, err := NewParamValidator("", WithPlugins(
		plugins.NewComparisonPlugin(),
		plugins.NewLengthPlugin(),
		plugins.NewRangePlugin(),
		plugins.NewPatternPlugin(),
	))
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name      string
		rulesStr  string
		wantError bool
	}{
		{
			name:      "empty constraint in brackets",
			rulesStr:  "param=[]",
			wantError: false, // Валидный key-only параметр
		},
		{
			name:      "callback constraint",
			rulesStr:  "param=[?]",
			wantError: false, // Валидный callback
		},
		{
			name:      "wildcard all parameters",
			rulesStr:  "param=[*]",
			wantError: false, // Валидный wildcard
		},
		{
			name:      "mixed valid and invalid plugin rules",
			rulesStr:  "valid=[cmp:>10]&invalid=[cmp:>]", // Добавлен cmp: и один валидный, один с ошибкой
			wantError: true,                              // Одна ошибка ломает всю проверку
		},
		{
			name:      "plugin constraint with spaces",
			rulesStr:  "age=[ cmp:> 18 ]", // Добавлен cmp:
			wantError: false,              // Пробелы тримятся
		},
		{
			name:      "multiple plugins in one rule",
			rulesStr:  "age=[cmp:>18]&name=[len:>5]&file=[in:*test*]&level=[range:1-10]", // Добавлен cmp:
			wantError: false,                                                             // Все плагины валидны
		},
		{
			name:      "mixed valid plugin and enum",
			rulesStr:  "age=[cmp:>18]&status=[active]&file=[in:*test*]", // Добавлен cmp:
			wantError: false,                                            // Плагины + enum работают вместе
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pv.CheckRules(tt.rulesStr)

			if tt.wantError {
				if err == nil {
					t.Errorf("CheckRules() expected error for rules %q, but got nil", tt.rulesStr)
				}
			} else {
				if err != nil {
					t.Errorf("CheckRules() unexpected error = %v for rules %q", err, tt.rulesStr)
				}
			}
		})
	}
}

func BenchmarkCheckRules(b *testing.B) {
	pv, err := NewParamValidator("", WithPlugins(
		plugins.NewComparisonPlugin(),
		plugins.NewLengthPlugin(),
		plugins.NewRangePlugin(),
		plugins.NewPatternPlugin(),
	))
	if err != nil {
		b.Fatalf("Failed to create validator: %v", err)
	}

	testRules := []string{
		"page=[1]&limit=[5]",
		"age=[cmp:>18]&username=[len:>5]&email=[in:*@*]",                              // Добавлен cmp:
		"/api/*?page=[1]&limit=[5];/users?age=[cmp:>18]&username=[len:>5]",            // Добавлен cmp:
		"param1=[range:1-10]&param2=[cmp:<100]&param3=[len:5..15]&param4=[in:*test*]", // Добавлен cmp:
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rules := testRules[i%len(testRules)]
		_ = pv.CheckRules(rules)
	}
}

func BenchmarkCheckRulesStatic(b *testing.B) {
	testRules := []string{
		"page=[1]&limit=[5]",
		"age=[cmp:>18]&username=[len:>5]", // Добавлен cmp:
		"/api/*?page=[1]&limit=[5]",
		"param1=[range:1-10]&param2=[cmp:<100]", // Добавлен cmp:
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rules := testRules[i%len(testRules)]
		_ = CheckRulesStatic(rules)
	}
}
