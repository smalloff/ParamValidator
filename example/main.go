package main

import (
	"fmt"

	"github.com/smalloff/paramvalidator"
	"github.com/smalloff/paramvalidator/plugins"
)

func main() {
	// Create plugins
	rangePlugin := plugins.NewRangePlugin()
	lengthPlugin := plugins.NewLengthPlugin()
	comparisonPlugin := plugins.NewComparisonPlugin()
	patternPlugin := plugins.NewPatternPlugin()

	// Example 1: Range plugin
	fmt.Println("=== Range Plugin ===")
	rules1 := "/products?page=[range:1-10]"
	pv1, err := paramvalidator.NewParamValidator(rules1, paramvalidator.WithPlugins(rangePlugin))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Valid:", pv1.ValidateURL("/products?page=5"))    // true
	fmt.Println("Invalid:", pv1.ValidateURL("/products?page=15")) // false

	// Example 2: Length plugin
	fmt.Println("\n=== Length Plugin ===")
	rules2 := "/api?username=[len:3..10]"
	pv2, err := paramvalidator.NewParamValidator(rules2, paramvalidator.WithPlugins(lengthPlugin))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Valid:", pv2.ValidateURL("/api?username=john")) // true
	fmt.Println("Invalid:", pv2.ValidateURL("/api?username=jo")) // false

	// Example 3: Comparison plugin
	fmt.Println("\n=== Comparison Plugin ===")
	rules3 := "/data?score=[cmp:>50]"
	pv3, err := paramvalidator.NewParamValidator(rules3, paramvalidator.WithPlugins(comparisonPlugin))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Valid:", pv3.ValidateURL("/data?score=75"))   // true
	fmt.Println("Invalid:", pv3.ValidateURL("/data?score=25")) // false

	// Example 4: Pattern plugin
	fmt.Println("\n=== Pattern Plugin ===")
	rules4 := "/files?name=[in:*.jpg]"
	pv4, err := paramvalidator.NewParamValidator(rules4, paramvalidator.WithPlugins(patternPlugin))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Valid:", pv4.ValidateURL("/files?name=photo.jpg")) // true
	fmt.Println("Invalid:", pv4.ValidateURL("/files?name=doc.pdf")) // false

	// Example 5: URL filtering
	fmt.Println("\n=== URL Filtering ===")
	rules5 := "/api?page=[range:1-10]&name=[len:3..10]"
	pv5, err := paramvalidator.NewParamValidator(rules5, paramvalidator.WithPlugins(rangePlugin, lengthPlugin))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	filtered := pv5.FilterURL("/api?page=5&name=john&invalid=param")
	fmt.Println("Filtered URL:", filtered) // /api?page=5&name=john

	// Example 6: Query validation
	fmt.Println("\n=== Query Validation ===")
	rules6 := "/search?q=[len:2..50]&limit=[range:1-100]"
	pv6, err := paramvalidator.NewParamValidator(rules6, paramvalidator.WithPlugins(lengthPlugin, rangePlugin))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	validQuery := pv6.ValidateQuery("/search", "q=test&limit=10")
	fmt.Println("Valid query:", validQuery) // true

	invalidQuery := pv6.ValidateQuery("/search", "q=a&limit=150")
	fmt.Println("Invalid query:", invalidQuery) // false

	// Example 7: Error handling
	fmt.Println("\n=== Error Handling ===")
	invalidRules := "/api?page=[len:constraint]"
	_, err = paramvalidator.NewParamValidator(invalidRules, paramvalidator.WithPlugins(lengthPlugin))
	if err != nil {
		fmt.Println("Expected error:", err)
	}
}
