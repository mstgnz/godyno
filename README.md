# DynamiGo

DynamiGo is a lightweight library that automatically converts Go database query results into dynamic structs with matching real types. It provides an experience similar to Laravel's `stdClass` in Go, without compromising Go's static type safety.

## 🌟 Key Features

- **Automatic Type Detection**: Creates structs with correct data types based on database query results
- **Nested Field Support**: Access nested fields with dot notation like `address.city`
- **Direct Usage**: Use values directly in conditional expressions and operations
- **Type-Safe Getters**: Type-safe getters like `GetBool()`, `GetInt()`, `GetFloat()`, `GetString()`
- **Pure Go Implementation**: Requires no external dependencies, uses only standard library

## 🤔 Why DynamiGo?

### Problem

When running database queries in Go, there are typically two ways to get results:

1. **Predefined structs**: Not flexible, requires different struct structures for each query
2. **map[string]interface{}**: Very flexible, but creates type conversion difficulties

```go
// Challenge with map[string]interface{}:
data := map[string]interface{}{"active": true}

// This doesn't work directly - requires type conversion
if data["active"] {  // Compilation error!
    // ...
}

// Instead, you always need to perform type conversion
if active, ok := data["active"].(bool); ok && active {
    // ...
}
```

### Solution

DynamiGo automatically converts database queries to Go values and provides a type-safe API:

```go
// With DynamiGo:
results, _ := dynamigo.QueryToStruct(db, "SELECT id, active, count FROM products")

// Direct usage in if conditions:
if results[0].GetBool("active") {
    fmt.Println("Product is active!")
}

// Numeric operations:
if results[0].GetInt("count") > 10 {
    fmt.Println("High stock!")
}
```

## 📦 Installation

```bash
go get github.com/yourusername/dynamigo
```

## 🚀 Usage

### Basic Query

```go
package main

import (
    "database/sql"
    "fmt"
    "log"

    "github.com/yourusername/dynamigo"
    _ "github.com/lib/pq"
)

func main() {
    db, err := sql.Open("postgres", "connection-string")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    query := "SELECT id, title, active, price FROM products WHERE category_id = $1"

    results, err := dynamigo.QueryToStruct(db, query, 5)
    if err != nil {
        log.Fatal(err)
    }

    for _, product := range results {
        // Boolean values
        if product.GetBool("active") {
            // Numeric values
            id := product.GetInt("id")
            price := product.GetFloat("price")

            // String values
            title := product.GetString("title")

            fmt.Printf("Product #%d: %s - %.2f USD\n", id, title, price)
        }
    }
}
```

### Using Nested Fields

```go
query := `SELECT
    p.id,
    p.title,
    p.price,
    c.name AS category.name,
    s.stock AS stock.quantity,
    s.status AS stock.status
FROM products p
JOIN categories c ON c.id = p.category_id
JOIN stock s ON s.product_id = p.id
WHERE p.id = $1`

result, err := dynamigo.QueryToStruct(db, query, 42)
if err != nil {
    log.Fatal(err)
}

if len(result) > 0 {
    product := result[0]

    fmt.Println("Product:", product.GetString("title"))
    fmt.Println("Category:", product.GetString("category.name"))

    // Access to nested fields
    if product.GetInt("stock.quantity") > 0 && product.GetString("stock.status") == "available" {
        fmt.Println("In stock!")
    }
}
```

## 💡 For Those Transitioning from Laravel to Go

If you're using the following structure in Laravel:

```php
$products = DB::select('select * from products');
foreach ($products as $product) {
    if ($product->active) {
        echo $product->title;
    }
}
```

With DynamiGo, you can write similarly in Go:

```go
products, _ := dynamigo.QueryToStruct(db, "SELECT * FROM products")
for _, product := range products {
    if product.GetBool("active") {
        fmt.Println(product.GetString("title"))
    }
}
```

## 🧪 Unit Tests

To run all tests:

```bash
go test -v ./...
```

## 📄 License

Distributed under the MIT License. See the `LICENSE` file for more information.

## 🤝 Contributing

Your contributions are welcome! Please add your tests before submitting a pull request and ensure the code complies with Go standards.

## 📊 Performance

Since DynamiGo works with dynamic types and uses reflection, it may show a slight performance difference compared to predefined structs. However, for many applications, this difference is negligible, and the flexibility provided by DynamiGo more than compensates for this small performance cost.

## 🙏 Inspiration

This library was developed to safely use features like Laravel's `stdClass` object and PHP's dynamic typing in Go. It aims to preserve the advantages of Go's static type system while offering the flexibility found in dynamic languages.
