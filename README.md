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
BenchmarkValidateURL-4                   	  496651	      2463 ns/op	    1344 B/op	      14 allocs/op
BenchmarkNormalizeURL-4                  	  462272	      2686 ns/op	    1512 B/op	      22 allocs/op
BenchmarkFilterQueryParamsParallel-4     	 2113588	       509.4 ns/op	     432 B/op	      12 allocs/op
BenchmarkFilterQueryParams-4             	  693978	      1687 ns/op	     432 B/op	      12 allocs/op
BenchmarkConcurrentValidation-4          	 1271839	      1036 ns/op	    1584 B/op	      15 allocs/op
BenchmarkConcurrentNormalization-4       	 1241494	       934.1 ns/op	    1496 B/op	      21 allocs/op
BenchmarkValidateQueryParams-4           	  406701	      2734 ns/op	    1424 B/op	      17 allocs/op
BenchmarkValidateQueryParamsParallel-4   	 1555990	       783.3 ns/op	    1328 B/op	      13 allocs/op
PASS
ok 
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


	urlPath := "/api/v1/users"
	queryString := "page=5&limit=10&invalid=param"

	// Fast filter
	filteredQuery := pv.FilterQueryParams(urlPath, queryString) // page=5&limit=10
}
```
