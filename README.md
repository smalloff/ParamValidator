# ParamValidator

A Go library for validating and normalizing URL parameters. Supports various pattern types. High performance.

## Installation
go get github.com/smalloff/paramvalidator

## Key Features

- ✅ URL parameter validation according to defined rules  
- 🔄 URL normalization with removal of invalid parameters  
- 🛡️ Thread-safe implementation  
- 📊 Support for ranges, enumerations, key-only parameters  
- 🎯 Global and URL-specific rules  
- 🔀 **Support for multiple rules with priorities**

Automatic priority determination based on path specificity
Rule combination - parameters from different rules are merged
Override - specific rules overwrite general ones
Support for wildcards and exact matches
```
cpu: Intel(R) Core(TM) i5-4670K CPU @ 3.40GHz
BenchmarkValidateURL-4                   	  906200	      1160 ns/op	     400 B/op	       3 allocs/op
BenchmarkNormalizeURL-4                  	  771000	      1400 ns/op	     480 B/op	       7 allocs/op
BenchmarkFilterQueryParamsParallel-4     	 3850118	       317.2 ns/op	     288 B/op	       5 allocs/op
BenchmarkFilterQueryParams-4             	 1000000	      1033 ns/op	     288 B/op	       5 allocs/op
BenchmarkConcurrentValidation-4          	 2223439	       493.8 ns/op	     656 B/op	       5 allocs/op
BenchmarkConcurrentNormalization-4       	 3019664	       417.0 ns/op	     480 B/op	       7 allocs/op
BenchmarkValidateQueryParams-4           	 1384642	       864.0 ns/op	     256 B/op	       2 allocs/op
BenchmarkValidateQueryParamsParallel-4   	 5045278	       233.9 ns/op	     256 B/op	       2 allocs/op
```

## Rule Syntax

```
Value range: "page=[1-10]"
Value enumeration: "sort=[name,date,price]"
Key-only parameter (no value): "active=[]"
Any value: "query=[*]"
Allow all parameters for URL: "/api/*?\*"
Allow with callback: "query=[?]"
```

## Quick Start

```go
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
	normalized := pv.NormalizeURL("/products?page=15&category=electronics&invalid=param")
	fmt.Println("Normalized URL:", normalized) // /products?category=electronics

	
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
}	
}
```
