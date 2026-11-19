package godyno

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// buildBenchResult returns a DBResult with a realistic set of mixed-type fields.
func buildBenchResult(b *testing.B) *DBResult {
	return newTestResult(b,
		[]string{"id", "title", "price", "active", "category.name", "category.id"},
		int64(42), "Benchmark Product", 99.99, true, "Electronics", int64(7))
}

func BenchmarkGetInt(b *testing.B) {
	r := buildBenchResult(b)
	for b.Loop() {
		_ = r.GetInt("id")
	}
}

func BenchmarkGetString(b *testing.B) {
	r := buildBenchResult(b)
	for b.Loop() {
		_ = r.GetString("title")
	}
}

func BenchmarkGetFloat(b *testing.B) {
	r := buildBenchResult(b)
	for b.Loop() {
		_ = r.GetFloat("price")
	}
}

func BenchmarkGetBool(b *testing.B) {
	r := buildBenchResult(b)
	for b.Loop() {
		_ = r.GetBool("active")
	}
}

func BenchmarkGetNestedString(b *testing.B) {
	r := buildBenchResult(b)
	for b.Loop() {
		_ = r.GetString("category.name")
	}
}

func BenchmarkHasField(b *testing.B) {
	r := buildBenchResult(b)
	for b.Loop() {
		_ = r.HasField("category.name")
	}
}

// BenchmarkMapAccess is a baseline: plain map[string]any lookup with type assertion.
func BenchmarkMapAccess(b *testing.B) {
	m := map[string]any{
		"id":     42,
		"title":  "Benchmark Product",
		"price":  99.99,
		"active": true,
	}
	for b.Loop() {
		_ = m["id"].(int)
	}
}

const benchRows = 1000

// BenchmarkScanRows measures godyno over 1000 rows from sqlmock.
func BenchmarkScanRows(b *testing.B) {
	db, mock, err := sqlmock.New()
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	for b.Loop() {
		mock.ExpectQuery("SELECT").WillReturnRows(newBenchRows())
		rows, err := db.Query("SELECT")
		if err != nil {
			b.Fatal(err)
		}
		if _, err := ScanRows(rows); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPlainScan is the baseline: the same rows scanned into []any by hand.
func BenchmarkPlainScan(b *testing.B) {
	db, mock, err := sqlmock.New()
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	for b.Loop() {
		mock.ExpectQuery("SELECT").WillReturnRows(newBenchRows())
		rows, err := db.Query("SELECT")
		if err != nil {
			b.Fatal(err)
		}
		var all [][]any
		for rows.Next() {
			vals := make([]any, 6)
			ptrs := make([]any, 6)
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				b.Fatal(err)
			}
			all = append(all, vals)
		}
		if err := rows.Err(); err != nil {
			b.Fatal(err)
		}
		_ = rows.Close()
	}
}

func newBenchRows() *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "title", "price", "active", "category.name", "category.id"})
	for i := range benchRows {
		rows.AddRow(int64(i), "Bench Product", 49.99, true, []byte("Electronics"), int64(3))
	}
	return rows
}
