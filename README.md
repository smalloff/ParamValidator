# ParamValidator

A Go library for validating and normalizing URL parameters. Supports various pattern types. High performance. More on [Wiki](smalloff/paramvalidator/wiki/Documentation).

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

Inversion "page=![5]"

## comment
line breaks
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


### ValidateURL
```go
package main

import (
    "github.com/smalloff/paramvalidator"
    "github.com/smalloff/paramvalidator/plugins"
)

func main() {
rangePlugin := plugins.NewRangePlugin()
    pv, _ := paramvalidator.NewParamValidator("/api?user_id=[range:1-100]&role=[moderator,admin]",
         paramvalidator.WithPlugins(rangePlugin))

    valid := pv.ValidateURL("/api?user_id=42&role=admin")
    println(valid) // Output: true
}
```

### FilterURL
```go
package main

import (
	"github.com/smalloff/paramvalidator"
	"github.com/smalloff/paramvalidator/plugins"
)

func main() {
	lengthPlugin := plugins.NewLengthPlugin()
	pv, _ := paramvalidator.NewParamValidator("/api?name=[len:>5]&role=[moderator,admin]",
		paramvalidator.WithPlugins(lengthPlugin))

	valid := pv.FilterURL("/api?name=small&role=admin")
	println(valid) // /api?role=admin
}
```

### FilterQuery with callback
```go
package main

import (
	"github.com/smalloff/paramvalidator"
)

func main() {
	pv, _ := paramvalidator.NewParamValidator("/api/*/users?name=[?]&role=[moderator,admin]",
		paramvalidator.WithCallback(func(paramName string, paramValue string) bool {
			if paramName == "name" {
				return len(paramValue) > 5
			}
			return true
		}))

	valid := pv.FilterQuery("/api/v2/users", "name=small&role=admin")
	println(valid) // /api?role=admin
}
```
