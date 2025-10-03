package main

import (
	"fmt"

	"github.com/smalloff/paramvalidator"
	"github.com/smalloff/paramvalidator/plugins"
)

func main() {
	// Create callback function
	callbackFunc := func(key string, value string) bool {
		switch key {
		case "year":
			return value == "2025"
		default:
			return false
		}
	}

	// Create plugins
	rangePlugin := plugins.NewRangePlugin()
	lengthPlugin := plugins.NewLengthPlugin()
	comparisonPlugin := plugins.NewComparisonPlugin()
	patternPlugin := plugins.NewPatternPlugin()

	// Unified rules combining all plugin types, user callback and inversion
	rules := "/*/data?page=[range:1-100]&username=[len:3..20]&score=[cmp:>50]&file=[in:*.jpg]&status=![pending,rejected]&year=[?]"

	// Create validator with all plugins and callback
	pv, err := paramvalidator.NewParamValidator(
		rules,
		paramvalidator.WithPlugins(rangePlugin, lengthPlugin, comparisonPlugin, patternPlugin),
		paramvalidator.WithCallback(callbackFunc),
	)
	if err != nil {
		fmt.Println("Error creating validator:", err)
		return
	}

	fmt.Printf("Rules: %s\n\n", rules)

	// 1. ValidateURL -> true
	url := "/api/data?page=5&username=john_doe&score=75&file=photo.jpg&status=approved"
	valid := pv.ValidateURL(url)
	fmt.Printf("1. ValidateURL:\nURL: %s\nValid: %t\n\n", url, valid)

	// 2. ValidateURL -> false
	url = "/api/data?page=1000&username=john_doe&score=75&file=photo.jpg&status=approved"
	valid = pv.ValidateURL(url)
	fmt.Printf("2. ValidateURL:\nURL: %s\nValid: %t\n\n", url, valid)

	// 3. ValidateURL with callback -> true
	url = "/api/data?year=2025&username=john_doe&score=75&file=photo.jpg&status=approved"
	valid = pv.ValidateURL(url)
	fmt.Printf("3. ValidateURL with callback:\nURL: %s\nValid: %t\n\n", url, valid)

	// 4. FilterURL -> /users/data?page=5&username=john_doe&file=photo.jpg
	urlWithExtra := "/users/data?page=5&username=john_doe&score=30&file=photo.jpg&status=pending&invalid=param"
	filtered := pv.FilterURL(urlWithExtra)
	fmt.Printf("4. FilterURL:\nOriginal: %s\nFiltered: %s\n\n", urlWithExtra, filtered)

	// 5. ValidateQuery -> false
	query := "page=5&username=john_doe&score=75&file=photo.png&status=approved"
	queryValid := pv.ValidateQuery("/pages/data", query)
	fmt.Printf("5. ValidateQuery:\nQuery: %s\nValid: %t\n", query, queryValid)

	// 6. Check rules -> Expected error: failed to parse params for URL /api: plugin len: invalid length value: 'invalid'
	fmt.Println("\n=== CheckRules ===")
	invalidRules := "/api?page=[len:invalid]"
	err = pv.CheckRules(invalidRules)
	if err != nil {
		fmt.Println("Expected error:", err)
	}

	// 6. Check rules with comments and multi lines -> true
	fmt.Println("\n=== CheckRules with comments and multi lines ===")
	rulesMulti := `## comment
sort=[date_desc,date_asc,updated_desc,updated_asc,price_desc,price_asc,votings_desc,votings_asc,downloads_desc,downloads_asc,views_desc,views_asc]&tags=[*]&s=[]
## allowed for main
/?*`
	err = pv.CheckRules(rulesMulti)
	if err != nil {
		fmt.Println("Expected error:", err)
	}

	println(err == nil)
}
