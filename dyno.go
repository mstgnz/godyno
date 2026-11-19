// Package godyno reads database/sql query results by column name, with typed
// getters, nested dot-notation fields and JSON output, without predefined structs.
package godyno

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"reflect"
)

// Querier runs a query. *sql.DB, *sql.Tx and *sql.Conn all satisfy it.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// New returns an empty DBResult. Every getter on it reports a missing field.
func New() *DBResult {
	return &DBResult{}
}

// FieldInfo described a nested field of the struct built by v1.1.
//
// Deprecated: unused since v1.2.0, which no longer builds structs. Kept so
// existing code still compiles.
type FieldInfo struct {
	Name string
	Type reflect.Type
}

// QueryToStruct runs query on db and returns every row. It returns nil and no
// error when nothing matches. Use QueryToStructContext for a context or a
// transaction, and QueryIter for large results.
func QueryToStruct(db *sql.DB, query string, args ...any) ([]*DBResult, error) {
	return QueryToStructContext(context.Background(), db, query, args...)
}

// QueryToStructContext runs query on q and returns every row. It returns nil and
// no error when nothing matches.
func QueryToStructContext(ctx context.Context, q Querier, query string, args ...any) ([]*DBResult, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	return ScanRows(rows)
}

// ScanRows reads all remaining rows and closes rows. Use it with *sql.Rows
// obtained elsewhere, for example from sqlx or a query builder.
func ScanRows(rows *sql.Rows) ([]*DBResult, error) {
	defer rows.Close()
	sc, err := newScanner(rows)
	if err != nil {
		return nil, err
	}
	var results []*DBResult
	for rows.Next() {
		r, err := sc.scan(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}
	return results, nil
}

// QueryIter runs query and yields one row at a time, so a large result is never
// held in memory. An error is yielded once with a nil row and ends the
// sequence. Leaving the loop early closes the rows.
func QueryIter(ctx context.Context, q Querier, query string, args ...any) iter.Seq2[*DBResult, error] {
	return func(yield func(*DBResult, error) bool) {
		rows, err := q.QueryContext(ctx, query, args...)
		if err != nil {
			yield(nil, fmt.Errorf("query error: %w", err))
			return
		}
		defer rows.Close()
		sc, err := newScanner(rows)
		if err != nil {
			yield(nil, err)
			return
		}
		for rows.Next() {
			r, err := sc.scan(rows)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(r, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(nil, fmt.Errorf("row iteration error: %w", err))
		}
	}
}

type scanner struct {
	meta *columnMeta
	ptrs []any
}

func newScanner(rows *sql.Rows) (*scanner, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %w", err)
	}
	meta, err := newColumnMeta(columns)
	if err != nil {
		return nil, err
	}
	return &scanner{meta: meta, ptrs: make([]any, len(columns))}, nil
}

func (s *scanner) scan(rows *sql.Rows) (*DBResult, error) {
	vals := make([]any, len(s.meta.columns))
	for i := range vals {
		s.ptrs[i] = &vals[i]
	}
	if err := rows.Scan(s.ptrs...); err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}
	// Text arrives as []byte from many drivers. Keeping it as text, never
	// guessing a number, is what preserves "007" and mixed-content columns.
	for i, v := range vals {
		if b, ok := v.([]byte); ok {
			vals[i] = string(b)
		}
	}
	return &DBResult{r: &row{meta: s.meta, vals: vals}}, nil
}
