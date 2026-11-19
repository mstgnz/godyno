# Changelog

## v1.2.0

Rows no longer build a struct with reflection. Each row keeps the value the driver returned, and the getters convert on read. Every exported name from v1.1.0 still exists.

### Fixed

- `QueryToStruct` panicked when a column held different kinds of values across rows, for example `"abc"` then `"42"` in a MySQL `varchar`, `100` then `12.5` in a Postgres `NUMERIC`, or an integer then a float in SQLite.
- `QueryToStruct` panicked on column names that are not Go identifiers: `count(*)`, Postgres `?column?`, `_id`, names with spaces.
- `QueryToStruct` panicked on three-level names (`a.b.c` with `a.b.d`) and on `category` together with `category.name`.
- Text that looks like a number lost data: `"007"` became `7`, `"05321234567"` lost its leading zero, and a 21-digit ID turned into a float.
- `GetBool` returned `false` for MySQL `tinyint(1)` (driver type `int64`).
- `New().GetString(...)`, a zero `DBResult`, and a path through a plain value (`"title.x"`) panicked.
- Getters cost about 72-182 ns and 2-3 allocations; they now cost about 10 ns and none.
- README: dotted aliases must be quoted (`AS "category.name"`), and the `GoDyno.QueryToStruct` typo.

### Added

- `QueryToStructContext(ctx, q, ...)` for a context, a transaction (`*sql.Tx`) or a `*sql.Conn`, through the new `Querier` interface.
- `QueryIter(ctx, q, ...)` reads rows one at a time (`iter.Seq2`).
- `ScanRows(rows)` reads `*sql.Rows` you already have.
- `GetInt64`, `GetInt64Ok`, `GetTime`, `GetTimeOk`, `IsNull`, `Columns`, `ToMap`, `MarshalJSON`.
- `GetBool` reads MySQL `BIT(1)`; numeric getters accept space-padded text (Postgres `char(n)`).
- Integration tests against SQLite, Postgres (pgx and lib/pq) and MySQL, and CI.

### Changed

- Duplicate column names (`SELECT a.id, b.id`) and a column that is also a nested prefix now return an error instead of losing a value. Add an alias.
- `Get` returns the driver's value instead of a guessed one: a numeric text column comes back as `string`, Postgres `NUMERIC` as `string`, integers as `int64`. Use the typed getters for a fixed type.
- `Get` returns `nil` for NULL. A nested group (`Get("category")`) returns a `map[string]any`.
- The `...Ok` getters are strict: they return `false` for NULL and for a lossy conversion (`GetIntOk` on `12.9`). The plain getters still truncate.
- Plain getters read more inputs than before: `GetInt` on the text `"12.9"` gives `12` (was `0`), text with surrounding spaces is read (`" 42 "` gives `42`, was `0`), and `GetString` on a NULL number gives `""` (was `"0"`).
- Numeric text must be a plain decimal; `1_000`, hex, `Infinity` and `NaN` text are not read as numbers.
- `GetString` writes floats in plain decimal (`12.5`) and times in RFC 3339.
- In JSON, NaN and ±Infinity are written as `null`, so one such value no longer fails the whole response.
- `FieldInfo` is deprecated and unused.
- The `examples/` program is replaced by runnable examples on pkg.go.dev; `github.com/lib/pq` is no longer in `go.mod`.

## v1.1.0

- Type inference scans every row, `HasField`, the `...Ok` getters, and benchmarks.

## v1.0.0

- First release.
