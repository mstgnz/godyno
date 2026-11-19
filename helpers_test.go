package godyno

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// newTestResult builds a row without a database, for getter tests.
func newTestResult(tb testing.TB, columns []string, vals ...any) *DBResult {
	tb.Helper()
	meta, err := newColumnMeta(columns)
	if err != nil {
		tb.Fatalf("newColumnMeta(%q): %v", columns, err)
	}
	if len(vals) != len(columns) {
		tb.Fatalf("got %d values for %d columns", len(vals), len(columns))
	}
	return &DBResult{r: &row{meta: meta, vals: vals}}
}

// queryMock runs QueryToStruct against sqlmock rows built from the given values.
func queryMock(t *testing.T, columns []string, rows ...[]any) ([]*DBResult, error) {
	t.Helper()
	db, mock := newMock(t)
	mock.ExpectQuery("SELECT").WillReturnRows(mockRows(columns, rows...))
	return QueryToStruct(db, "SELECT")
}

func newMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

func mockRows(columns []string, rows ...[]any) *sqlmock.Rows {
	r := sqlmock.NewRows(columns)
	for _, row := range rows {
		vals := make([]driver.Value, len(row))
		for i, v := range row {
			vals[i] = v
		}
		r.AddRow(vals...)
	}
	return r
}
