package main

import (
	"fmt"

	"github.com/smalloff/paramvalidator"
)

func main() {
	// Define validation rules
	rules := "/products?page=[1-10]&category=[electronics,books]"

	// Create validator
	pv, err := paramvalidator.NewParamValidator(rules)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Validate URL parameters
	isValid := pv.ValidateURL("/products?page=5&category=electronics")
	fmt.Println("URL valid:", isValid) // true

	// Normalize invalid URL (removes invalid params)
	normalized := pv.FilterURL("/products?page=15&category=electronics&invalid=param")
	fmt.Println("Normalized URL:", normalized) // /products?category=electronics

	// Validate query parameters string
	validQuery := pv.ValidateQuery("/products", "page=5&category=electronics")
	fmt.Println("Query params valid:", validQuery) // true

	// Filter query parameters (keep only allowed ones)
	filtered := pv.FilterQuery("/products", "page=15&category=books&invalid=value")
	fmt.Println("Filtered query:", filtered) // category=books

	// Rule with callback parameter [?]
	rules = "/api?auth=[?]&page=[1-10]"

	// Create validator with callback function
	pv, err = paramvalidator.NewParamValidator(rules, func(key string, value string) bool {
		if key == "auth" {
			// Custom validation logic
			return value == "secret123"
		}
		return false
	})

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Validate with callback
	valid1 := pv.ValidateURL("/api?auth=secret123&page=5")
	fmt.Println("Valid auth:", valid1) // true

	valid2 := pv.ValidateURL("/api?auth=wrong&page=5")
	fmt.Println("Invalid auth:", valid2) // false

	// Validate and filter query strings separately
	queryValid := pv.ValidateQuery("/api", "auth=secret123&page=3")
	fmt.Println("Query validation:", queryValid) // true

	filteredQuery := pv.FilterQuery("/api", "auth=wrong&page=3&extra=param")
	fmt.Println("Filtered query:", filteredQuery) // page=3
}
