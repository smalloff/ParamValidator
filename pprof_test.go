package paramvalidator

import (
	"os"
	"runtime/pprof"
	"strconv"
	"testing"
)

// TestCPUProfile - профиль для валидации URL
func TestCPUProfile(t *testing.T) {
	f, err := os.Create("cpu.prof")
	if err != nil {
		t.Fatalf("could not create CPU profile: %v", err)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		t.Fatalf("could not start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	pv := createTestValidator(t)

	// Предварительно создаем URL без strconv.Itoa в цикле
	urls := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		urls[i] = "/user/" + strconv.Itoa(i) + "?age=25&score=75&username=testuser"
	}

	// Теперь в горячем цикле только использование
	for i := 0; i < 100000; i++ {
		pv.ValidateURL(urls[i%1000])
	}
}

func TestMemoryProfile(t *testing.T) {
	pv := createTestValidator(t)

	// Предварительно создаем URL
	urls := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		urls[i] = "/user/" + strconv.Itoa(i) + "?age=25&score=75&username=testuser&extra=param"
	}

	// Операции фильтрации без аллокаций в цикле
	for i := 0; i < 50000; i++ {
		pv.FilterURL(urls[i%1000])
	}

	f, err := os.Create("heap.prof")
	if err != nil {
		t.Fatalf("could not create memory profile: %v", err)
	}
	defer f.Close()

	if err := pprof.WriteHeapProfile(f); err != nil {
		t.Fatalf("could not write memory profile: %v", err)
	}
}

func createTestValidator(t *testing.T) *ParamValidator {
	pv, err := NewParamValidator("")
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	rules := `
		age=[18-65]&score=[>50]&username=[len>5];
		/user/*?level=[1-10]&status=[active,inactive];
		/api/*?token=[len=32]&limit=[1-100]&offset=[>=0];
		/products?price=[<1000]&quantity=[1-100];
		/search?q=[len1..100]&page=[1-100];
	`

	err = pv.ParseRules(rules)
	if err != nil {
		t.Fatalf("Failed to parse rules: %v", err)
	}

	return pv
}
