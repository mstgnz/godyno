# godyno

[![ci](https://github.com/mstgnz/godyno/actions/workflows/ci.yml/badge.svg)](https://github.com/mstgnz/godyno/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/mstgnz/godyno.svg)](https://pkg.go.dev/github.com/mstgnz/godyno)

GoDyno reads `database/sql` query results by column name, without a predefined struct for every query. It gives Go an experience close to Laravel's `DB::select` and `stdClass`: run a query, read `row.GetInt("id")`, return the rows as JSON.

## 🌟 Key Features

- **No struct per query**: read any column by name with `GetString`, `GetInt`, `GetInt64`, `GetFloat`, `GetBool`, `GetTime`
- **Nothing guessed, nothing lost**: the driver's value is kept as is, so `"007"` stays `"007"` and a mixed-content column never breaks the query
- **NULL aware**: `IsNull` and the `...Ok` getters tell a NULL from a real zero
- **Nested fields**: alias a column as `"category.name"` and read it with dot notation, or get the whole group as a map
- **JSON ready**: a row, or a slice of rows, marshals to nested JSON
- **Context, transactions, streaming**: works with `*sql.DB`, `*sql.Tx` and `*sql.Conn`; `QueryIter` reads large results one row at a time
- **No dependencies**: standard library only; tested against SQLite, Postgres (pgx and lib/pq) and MySQL

## 🤔 Why GoDyno?

### Problem

When running database queries in Go, there are typically two ways to get results:

1. **Predefined structs**: Not flexible, requires different struct structures for each query
2. **map[string]any**: Very flexible, but creates type conversion difficulties

```go
// Challenge with map[string]any:
data := map[string]any{"active": true}

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

GoDyno keeps each row and converts on read, so a value can go straight into an `if`:

```go
// With GoDyno:
results, _ := godyno.QueryToStruct(db, "SELECT id, active, count FROM products")

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
go get github.com/mstgnz/godyno
```

GoDyno does not import any database driver. Add the driver for your database yourself:

```go
import _ "github.com/lib/pq"           // PostgreSQL
import _ "github.com/go-sql-driver/mysql" // MySQL
import _ "modernc.org/sqlite"             // SQLite (pure Go; the one the tests use)
```

## 🚀 Usage

### Basic Query

```go
package main

import (
    "database/sql"
    "fmt"
    "log"

    "github.com/mstgnz/godyno"
    _ "github.com/lib/pq"
)

func main() {
    db, err := sql.Open("postgres", "connection-string")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    query := "SELECT id, title, active, price FROM products WHERE category_id = $1"

    results, err := godyno.QueryToStruct(db, query, 5)
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

A dotted alias must be quoted: double quotes in Postgres and SQLite, backticks in MySQL.

```go
query := `SELECT
    p.id,
    p.title,
    p.price,
    c.name AS "category.name",
    s.stock AS "stock.quantity",
    s.status AS "stock.status"
FROM products p
JOIN categories c ON c.id = p.category_id
JOIN stock s ON s.product_id = p.id
WHERE p.id = $1`

result, err := godyno.QueryToStruct(db, query, 42)
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

    // The whole group as a map: map[quantity:10 status:available]
    fmt.Println(product.Get("stock"))
}
```

Any depth works: `"a.b.c"` is read with `GetString("a.b.c")` and `Get("a")` returns the nested maps.

### Returning Rows as JSON

```go
func listProducts(w http.ResponseWriter, r *http.Request) {
    rows, err := godyno.QueryToStructContext(r.Context(), db,
        `SELECT p.id, p.title, c.name AS "category.name" FROM products p JOIN categories c ON c.id = p.category_id`)
    if err != nil {
        http.Error(w, "query failed", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(rows)
    // [{"category":{"name":"Books"},"id":1,"title":"Go Book"}]
}
```

Keys are sorted (that is how `encoding/json` writes maps), NULL becomes `null` and `time.Time` uses RFC 3339. NaN and ±Infinity, which JSON cannot represent, are written as `null`; `ToMap` keeps them.

### Transactions and Context

```go
tx, _ := db.BeginTx(ctx, nil)
defer tx.Rollback()

rows, err := godyno.QueryToStructContext(ctx, tx, "SELECT id, balance FROM accounts WHERE id = $1 FOR UPDATE", 7)
```

`QueryToStructContext` accepts anything with a `QueryContext` method: `*sql.DB`, `*sql.Tx` and `*sql.Conn`. If you already have `*sql.Rows` (from sqlx or a query builder), pass them to `ScanRows`.

### Large Results

`QueryToStruct` holds every row in memory. For an export or a batch job, iterate instead:

```go
for row, err := range godyno.QueryIter(ctx, db, "SELECT id, email FROM users") {
    if err != nil {
        return err
    }
    send(row.GetString("email"))
}
```

Leaving the loop early closes the rows.

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

With GoDyno, you can write similarly in Go:

```go
products, _ := godyno.QueryToStruct(db, "SELECT * FROM products")
for _, product := range products {
    if product.GetBool("active") {
        fmt.Println(product.GetString("title"))
    }
}
```

## API Reference

### Running queries

| Function | Use |
|----------|-----|
| `QueryToStruct(db, query, args...)` | All rows from a `*sql.DB` |
| `QueryToStructContext(ctx, q, query, args...)` | All rows, with a context, from `*sql.DB`, `*sql.Tx` or `*sql.Conn` |
| `QueryIter(ctx, q, query, args...)` | One row at a time (`iter.Seq2[*DBResult, error]`) |
| `ScanRows(rows)` | All rows from existing `*sql.Rows`; closes them |

When nothing matches, the slice functions return `nil` and no error, and `QueryIter` yields nothing.

### Plain getters (lenient)

They return the zero value when the field is missing, NULL or not convertible, and `GetInt` truncates `12.9` to `12`.

| Method | Return type | Reads |
|--------|-------------|-------|
| `GetString(field)` | `string` | any value; numbers in plain decimal, times in RFC 3339 |
| `GetInt(field)` / `GetInt64(field)` | `int` / `int64` | integers, floats, numeric text |
| `GetFloat(field)` | `float64` | integers, floats, numeric text such as Postgres `NUMERIC` |
| `GetBool(field)` | `bool` | bool, integers (MySQL `tinyint(1)`), `BIT(1)`, text `1/0/t/f/true/false` |
| `GetTime(field)` | `time.Time` | `time.Time`, or text dates such as `2026-01-02 15:04:05` (read as UTC) |
| `Get(field)` | `any` | the stored value, `nil` for NULL, a `map[string]any` for a nested group |

### Ok getters (strict)

They return `false` when the field is missing, NULL, or does not convert without loss (`GetIntOk` on `12.9` is `false`).

```go
if score, ok := row.GetIntOk("score"); ok {
    fmt.Println("score:", score)
} else if row.IsNull("score") {
    fmt.Println("no score yet")
}
```

`GetStringOk`, `GetIntOk`, `GetInt64Ok`, `GetFloatOk`, `GetBoolOk`, `GetTimeOk`.

Numeric text must be a plain decimal such as `-12.50` (and `1e5` for floats). Integers are read digit by digit, so `9007199254740993.00` stays exact. Text like `1_000`, `0x10`, `Infinity` or `NaN` is not a number here.

### Row helpers

| Method | Returns |
|--------|---------|
| `HasField(field)` | `true` for a column (even NULL) or a nested group |
| `IsNull(field)` | `true` only for an existing NULL column |
| `Columns()` | column names in query order |
| `ToMap()` | the row as nested `map[string]any` |
| `MarshalJSON()` | the row as nested JSON |

### Field names

- A dot means nesting: `"category.name"`.
- The first letter of each segment is case-insensitive: `"id"` and `"Id"` are the same field, `"ID"` is not.
- Any column name works, including `count(*)` and Postgres `?column?`, but an alias is easier to read. Two unnamed Postgres expressions (`SELECT 1, 2`) are both `?column?`, which is a duplicate.

## Behavior to Know

### Duplicate column names are an error

`SELECT a.id, b.id ...` returns `godyno: duplicate column "id" (same as "id"), give one of them an alias`, because one of the two values would otherwise be lost. Give one of them an alias. The same applies to `category` together with `category.name`.

### Values keep the driver's type

godyno does not guess types from text. What `Get` returns depends on the driver (measured with pgx v5.13, lib/pq v1.12, go-sql-driver/mysql v1.10 and modernc.org/sqlite v1.62):

| Database | Integer | Decimal | Text | Date and time |
|----------|---------|---------|------|---------------|
| Postgres, pgx or lib/pq | `int64` | `string` (`NUMERIC`) | `string` | `time.Time` |
| MySQL | `int64` | `string` (`DECIMAL`) | `string` | `string`, or `time.Time` with `parseTime=true` |
| SQLite (modernc) | `int64` | `float64`, or `int64` for a whole value in a `NUMERIC` column | `string` | `time.Time` for `DATE`/`DATETIME` columns, `string` for expressions |

Older drivers can differ (older MySQL drivers return integers as text). The typed getters read all of these forms. For MySQL, add `parseTime=true&loc=Local` (or your zone) to the DSN so dates arrive as `time.Time` in the right zone.

### Binary columns

`BLOB` and `bytea` values are stored as `string`. Use `[]byte(row.GetString("data"))` to get the bytes back; JSON output of binary data is not meaningful.

## 🧪 Tests

```bash
go test -race ./...
```

Integration tests run against real drivers in a separate module, so the library itself keeps no dependencies. SQLite always runs; Postgres and MySQL run when their DSN is set:

```bash
cd integration
GODYNO_POSTGRES_DSN='postgres://postgres:test@127.0.0.1:5432/godyno?sslmode=disable' \
GODYNO_MYSQL_DSN='root:test@tcp(127.0.0.1:3306)/godyno' \
go test -race ./...
```

CI runs both suites on every push and pull request.

## 📄 License

Distributed under the MIT License. See the `LICENSE` file for more information.

## 🤝 Contributing

Your contributions are welcome! Please add your tests before submitting a pull request and ensure the code complies with Go standards.

## 📊 Performance

Benchmarks on Apple M2 Max (`go test -bench . -benchmem`):

| Operation | Time | Allocs |
|-----------|------|--------|
| `GetInt`, `GetString`, `GetFloat`, `GetBool` | ~9-10 ns | 0 |
| `GetString` (nested) | ~10 ns | 0 |
| `map[string]any` lookup (baseline) | ~7.6 ns | 0 |
| `ScanRows`, 1000 rows of 6 columns | ~375 µs | 14.8k |
| Plain `rows.Scan` into `[]any`, same rows (baseline) | ~337 µs | 10.8k |

Reading 1000 rows costs about 12% more than scanning them by hand, and a getter costs about as much as a map lookup. Both scan benchmarks include the sqlmock driver.

## 🙏 Inspiration

This library was developed to safely use features like Laravel's `stdClass` object and PHP's dynamic typing in Go. It aims to preserve the advantages of Go's static type system while offering the flexibility found in dynamic languages.
