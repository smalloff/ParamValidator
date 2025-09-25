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
```

## Quick Start

```go
package main


import (
	"fmt"
	"github.com/smalloff/paramvalidator"
)

func main() {
	// Create validator
	rules := "/products?page=[1-10]&category=[electronics,books]"
	pv := paramvalidator.NewParamValidator(rules)

	// Validate URL
	isValid := pv.ValidateURL("/products?page=5&category=electronics")
	fmt.Println("URL valid:", isValid) // true

	// Normalize URL
	normalized := pv.NormalizeURL("/products?page=15&category=electronics&invalid=param")
	fmt.Println("Normalized URL:", normalized) // /products?category=electronics

	// Multiple URL rules
	rules = "/products?page=[1-10];/users?sort=[name,date];/search?q=[]"
	pv = paramvalidator.NewParamValidator(rules)

	fmt.Println(pv.ValidateURL("/products?page=5")) // true
	fmt.Println(pv.ValidateURL("/users?sort=name")) // true
	fmt.Println(pv.ValidateURL("/search?q"))        // true (key-only parameter)

	// Global rules + URL-specific
	rules = "page=[1-100];/products?page=[1-10];/admin/*?access=[admin,superuser]"
	pv = paramvalidator.NewParamValidator(rules)

	// Global rule works for any URL
	fmt.Println(pv.ValidateURL("/any/path?page=50")) // true (global rule)

	// URL-specific rule has priority
	fmt.Println(pv.ValidateURL("/products?page=5"))  // true (specific rule)
	fmt.Println(pv.ValidateURL("/products?page=50")) // false (1-10 restriction)

	// Wildcard rules
	fmt.Println(pv.ValidateURL("/admin/users?access=admin"))    // true
	fmt.Println(pv.ValidateURL("/admin/settings?access=admin")) // true


	urlPath := "/products"
	queryString := "page=5&limit=10&invalid=param"

	// Fast filter
	filteredQuery := pv.FilterQueryParams(urlPath, queryString) // page=5
}
```
