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
    // Example 1: Basic usage without callback
    rules := "/products?page=[1-10]&category=[electronics,books]"
    pv, err := paramvalidator.NewParamValidator(rules)
    if err != nil {
        fmt.Println("Error creating validator:", err)
        return
    }

    // Validate URL
    isValid := pv.ValidateURL("/products?page=5&category=electronics")
    fmt.Println("URL valid:", isValid) // true

    // Normalize URL
    normalized := pv.NormalizeURL("/products?page=15&category=electronics&invalid=param")
    fmt.Println("Normalized URL:", normalized) // /products?category=electronics

    // Example 2: Multiple URL rules
    rules = "/products?page=[1-10];/users?sort=[name,date];/search?q=[]"
    pv, err = paramvalidator.NewParamValidator(rules)
    if err != nil {
        fmt.Println("Error creating validator:", err)
        return
    }

    fmt.Println("Products page valid:", pv.ValidateURL("/products?page=5")) // true
    fmt.Println("Users sort valid:", pv.ValidateURL("/users?sort=name"))    // true
    fmt.Println("Search query valid:", pv.ValidateURL("/search?q"))         // true (key-only parameter)

    // Example 3: Global rules + URL-specific with callback
    rules = "page=[1-100];/products?page=[1-10];/admin/*?access=[admin,superuser]&token=[?]"
    
    // Create validator with callback function
    callbackFunc := func(key string, value string) bool {
        switch key {
        case "token":
            // Custom validation logic for token
            return len(value) == 32 && strings.HasPrefix(value, "tok_")
        default:
            return false
        }
    }
    
    pv, err = paramvalidator.NewParamValidator(rules, callbackFunc)
    if err != nil {
        fmt.Println("Error creating validator:", err)
        return
    }

    // Global rule works for any URL
    fmt.Println("Global rule valid:", pv.ValidateURL("/any/path?page=50")) // true

    // URL-specific rule has priority
    fmt.Println("Specific rule valid:", pv.ValidateURL("/products?page=5"))  // true
    fmt.Println("Specific rule invalid:", pv.ValidateURL("/products?page=50")) // false

    // Wildcard rules with callback
    fmt.Println("Admin with valid token:", pv.ValidateURL("/admin/users?access=admin&token=tok_123456789012345678901234567890")) // true
    fmt.Println("Admin with invalid token:", pv.ValidateURL("/admin/users?access=admin&token=invalid")) // false

    // Example 4: Query parameter filtering
    urlPath := "/products"
    queryString := "page=5&limit=10&invalid=param"

    // Fast filter
    filteredQuery := pv.FilterQueryParams(urlPath, queryString)
    fmt.Println("Filtered query:", filteredQuery) // page=5

    // Example 5: Dynamic callback setting
    pv, err = paramvalidator.NewParamValidator("/api?auth=[?]")
    if err != nil {
        fmt.Println("Error creating validator:", err)
        return
    }

    // Initially, callback parameters will fail without a callback function
    fmt.Println("Without callback:", pv.ValidateURL("/api?auth=secret")) // false

    // Set callback dynamically
    pv.SetCallback(func(key string, value string) bool {
        return value == "secret123"
    })

    fmt.Println("With callback - valid:", pv.ValidateURL("/api?auth=secret123")) // true
    fmt.Println("With callback - invalid:", pv.ValidateURL("/api?auth=wrong"))   // false

    // Example 6: Mixed callback and regular rules
    rules = "/data?page=[1-10]&auth=[?]&filter=[active,inactive]"
    pv, err = paramvalidator.NewParamValidator(rules, func(key string, value string) bool {
        if key == "auth" {
            return strings.HasPrefix(value, "bearer_")
        }
        return false
    })

    if err != nil {
        fmt.Println("Error creating validator:", err)
        return
    }

    fmt.Println("Mixed rules - valid:", pv.ValidateURL("/data?page=5&auth=bearer_token123&filter=active")) // true
    fmt.Println("Mixed rules - invalid auth:", pv.ValidateURL("/data?page=5&auth=invalid&filter=active")) // false
    fmt.Println("Mixed rules - invalid page:", pv.ValidateURL("/data?page=15&auth=bearer_token123&filter=active")) // false

    // Example 7: Normalization with callback parameters
    normalized = pv.NormalizeURL("/data?page=5&auth=invalid_token&filter=active&extra=param")
    fmt.Println("Normalized with callback:", normalized) // /data?page=5&filter=active
}
```
