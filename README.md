# ParamValidator

Библиотека на Go для валидации и нормализации параметров URL. Поддерживает различные типы паттернов. Высокая производительность.

## Основные возможности

- ✅ Валидация параметров URL по заданным правилам
- 🔄 Нормализация URL с удалением невалидных параметров
- 🛡️ Потокобезопасная реализация
- 📊 Поддержка диапазонов, перечислений, key-only параметров
- 🎯 Глобальные и URL-специфичные правила
- 🔀 **Поддержка множественных правил с приоритетами**

Автоматическое определение приоритета по специфичности пути
Комбинирование правил - параметры из разных правил объединяются
Переопределение - специфичные правила перезаписывают общие
Поддержка wildcards и точных совпадений


## Синтаксис правил

Диапазон значений "page=[1-10]"

Перечисление значений  "sort=[name,date,price]"

Key-only параметр (без значения) "active=[]"

Любое значение "query=[*]"

Разрешить все параметры для URL "/api/*?*"

## Быстрый старт

```go
// Создание валидатора
rules := "/products?page=[1-10]&category=[electronics,books]"
pv := paramvalidator.NewParamValidator(rules)

// Проверка URL
isValid := pv.ValidateURL("/products?page=5&category=electronics")
fmt.Println("URL valid:", isValid) // true

// Нормализация URL
normalized := pv.NormalizeURL("/products?page=15&category=electronics&invalid=param")
fmt.Println("Normalized URL:", normalized) // /products?category=electronics

// Несколько URL-правил
rules := "/products?page=[1-10];/users?sort=[name,date];/search?q=[]"
pv := paramvalidator.NewParamValidator(rules)

fmt.Println(pv.ValidateURL("/products?page=5"))    // true
fmt.Println(pv.ValidateURL("/users?sort=name"))    // true
fmt.Println(pv.ValidateURL("/search?q"))          // true (key-only параметр)

// Глобальные правила + URL-специфичные
rules := "page=[1-100];/products?page=[1-10];/admin/*?access=[admin,superuser]"
pv := paramvalidator.NewParamValidator(rules)

// Глобальное правило работает для любого URL
fmt.Println(pv.ValidateURL("/any/path?page=50"))     // true (глобальное правило)

// URL-специфичное правило имеет приоритет
fmt.Println(pv.ValidateURL("/products?page=5"))      // true (специфичное правило)
fmt.Println(pv.ValidateURL("/products?page=50"))     // false (ограничение 1-10)

// Wildcard правила
fmt.Println(pv.ValidateURL("/admin/users?access=admin"))    // true
fmt.Println(pv.ValidateURL("/admin/settings?access=admin")) // true
