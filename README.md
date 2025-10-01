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

```
cpu: Intel(R) Core(TM) i5-4670K CPU @ 3.40GHz
BenchmarkValidateURL-4               	 1000000	      1072 ns/op	     144 B/op	       1 allocs/op
BenchmarkFilterURL-4                 	 1000000	      1054 ns/op	     192 B/op	       3 allocs/op
BenchmarkFilterQuery-4               	 1630462	       658.4 ns/op	      16 B/op	       1 allocs/op
BenchmarkValidateQuery-4             	 2363110	       490.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkConcurrentValidation-4      	 1403109	       847.8 ns/op	     432 B/op	       3 allocs/op
BenchmarkConcurrentNormalization-4   	 3611492	       319.6 ns/op	     192 B/op	       3 allocs/op
BenchmarkConcurrentFilterQuery-4     	 1921903	       618.2 ns/op	      32 B/op	       3 allocs/op
BenchmarkConcurrentValidateQuery-4   	 2330829	       491.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterQueryBytes-4          	 2234452	       552.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkValidateQueryBytes-4        	 2167021	       538.4 ns/op	       0 B/op	       0 allocs/op
```

Automatic priority determination based on path specificity
Rule combination - parameters from different rules are merged
Override - specific rules overwrite general ones
Support for wildcards and exact matches


## Basic Rule Syntax
```
Single value "page=[5]"

Value enumeration "sort=[name,date,price]"

Key-only parameter (no value) "active=[]"

Any value "query=[*]"

Callback parameter "token=[?]"
```

## Plugins Rule Syntax
```
Comparison Plugin "age=[cmp:>18]&score=[cmp:>=50]&price=[cmp:<1000]"

Length Plugin "username=[len:>5]&password=[len:8..20]&token=[len:32]"

Range Plugin "level=[range:1..10]&percentage=[range:0..100]&temp=[range:-20..40]"

Pattern Plugin "file=[in:*_test.go]&email=[in:*@*]&id=[in:user_*]"
```

## Urls Syntax
```
Allow all parameters for URL "/api/*?\*"

URL-Specific and global Rules "count[cmp:<100];/api/*/products?page=[5]&category=[electronics,books];/users?role=[admin,user]"
```

## Quick Start

```go
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
}
```