// Package integration runs godyno against real drivers, because sqlmock does not
// return the types real drivers do. SQLite always runs; Postgres and MySQL run
// when GODYNO_POSTGRES_DSN or GODYNO_MYSQL_DSN is set (CI sets both).
package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"

	"github.com/mstgnz/godyno"
)

var created = time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)

// dialect holds the SQL that differs between databases.
type dialect struct {
	setup       []string
	placeholder string // first bind parameter: "?" or "$1"
	countColumn string // the name the database gives to count(*)
	quote       func(string) string
}

func open(t *testing.T, driver, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open(driver, dsn)
	if err != nil {
		t.Fatalf("open %s: %v", driver, err)
	}
	// Temporary tables live on one connection.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("ping %s: %v", driver, err)
	}
	return db
}

func TestSQLite(t *testing.T) {
	db := open(t, "sqlite", "file:"+t.TempDir()+"/godyno.db")
	runScenario(t, db, dialect{
		setup: []string{
			`CREATE TABLE items (id INTEGER PRIMARY KEY, code TEXT, zip TEXT, amount NUMERIC,
				active INTEGER, created_at DATETIME, note TEXT)`,
			`INSERT INTO items VALUES (1, 'abc', '007', 100, 1, '2026-01-02 15:04:05', NULL),
				(2, '42', '34000', 12.5, 0, '2026-01-02 15:04:05', 'x')`,
		},
		placeholder: "?",
		countColumn: "count(*)",
		quote:       func(s string) string { return `"` + s + `"` },
	})
}

func TestPostgresPgx(t *testing.T) { testPostgres(t, "pgx") }

func TestPostgresLibPQ(t *testing.T) { testPostgres(t, "postgres") }

func testPostgres(t *testing.T, driver string) {
	dsn := os.Getenv("GODYNO_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GODYNO_POSTGRES_DSN not set")
	}
	db := open(t, driver, dsn)
	runScenario(t, db, dialect{
		setup: []string{
			`CREATE TEMP TABLE items (id int PRIMARY KEY, code text, zip text, amount numeric,
				active boolean, created_at timestamptz, note text)`,
			`INSERT INTO items VALUES (1, 'abc', '007', 100, true, '2026-01-02 15:04:05+00', NULL),
				(2, '42', '34000', 12.5, false, '2026-01-02 15:04:05+00', 'x')`,
		},
		placeholder: "$1",
		countColumn: "count",
		quote:       func(s string) string { return `"` + s + `"` },
	})

	t.Run("unnamed expression column", func(t *testing.T) {
		rows, err := godyno.QueryToStruct(db, "SELECT 1+1")
		if err != nil {
			t.Fatal(err)
		}
		if got := rows[0].GetInt("?column?"); got != 2 {
			t.Errorf("?column? = %d, want 2", got)
		}
	})
}

func TestMySQL(t *testing.T) {
	dsn := os.Getenv("GODYNO_MYSQL_DSN")
	if dsn == "" {
		t.Skip("GODYNO_MYSQL_DSN not set")
	}
	// No parseTime: DATETIME then arrives as text, which GetTime must still read.
	db := open(t, "mysql", dsn)
	runScenario(t, db, dialect{
		setup: []string{
			`CREATE TEMPORARY TABLE items (id int PRIMARY KEY, code varchar(10), zip varchar(10),
				amount decimal(10,2), active tinyint(1), created_at datetime, note varchar(10))`,
			`INSERT INTO items VALUES (1, 'abc', '007', 100, 1, '2026-01-02 15:04:05', NULL),
				(2, '42', '34000', 12.5, 0, '2026-01-02 15:04:05', 'x')`,
		},
		placeholder: "?",
		countColumn: "count(*)",
		quote:       func(s string) string { return "`" + s + "`" },
	})

	t.Run("BIT(1) column", func(t *testing.T) {
		if _, err := db.Exec(`CREATE TEMPORARY TABLE flags (f bit(1))`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO flags VALUES (1), (0)`); err != nil {
			t.Fatal(err)
		}
		rows, err := godyno.QueryToStruct(db, "SELECT f FROM flags ORDER BY f DESC")
		if err != nil {
			t.Fatal(err)
		}
		if !rows[0].GetBool("f") || rows[1].GetBool("f") {
			t.Errorf("BIT(1) = %v, %v, want true, false", rows[0].GetBool("f"), rows[1].GetBool("f"))
		}
	})
}

func runScenario(t *testing.T, db *sql.DB, d dialect) {
	t.Helper()
	for _, stmt := range d.setup {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup: %v\n%s", err, stmt)
		}
	}
	const cols = "id, code, zip, amount, active, created_at, note"

	// Without args MySQL uses its text protocol, with args the binary protocol;
	// the driver returns different types for each, and both must read the same.
	t.Run("without args", func(t *testing.T) {
		rows, err := godyno.QueryToStruct(db, "SELECT "+cols+" FROM items ORDER BY id")
		if err != nil {
			t.Fatal(err)
		}
		checkItems(t, rows)
	})
	t.Run("with args", func(t *testing.T) {
		rows, err := godyno.QueryToStructContext(context.Background(), db,
			"SELECT "+cols+" FROM items WHERE id > "+d.placeholder+" ORDER BY id", 0)
		if err != nil {
			t.Fatal(err)
		}
		checkItems(t, rows)
	})
	t.Run("iterator", func(t *testing.T) {
		var n int
		for r, err := range godyno.QueryIter(context.Background(), db, "SELECT "+cols+" FROM items ORDER BY id") {
			if err != nil {
				t.Fatal(err)
			}
			if r.GetInt("id") != n+1 {
				t.Errorf("row %d id = %d", n, r.GetInt("id"))
			}
			n++
		}
		if n != 2 {
			t.Errorf("iterated %d rows, want 2", n)
		}
	})
	t.Run("transaction", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		rows, err := godyno.QueryToStructContext(context.Background(), tx, "SELECT "+cols+" FROM items ORDER BY id")
		if err != nil {
			t.Fatal(err)
		}
		checkItems(t, rows)
	})
	t.Run("count without alias", func(t *testing.T) {
		rows, err := godyno.QueryToStruct(db, "SELECT count(*) FROM items")
		if err != nil {
			t.Fatal(err)
		}
		if got := rows[0].GetInt(d.countColumn); got != 2 {
			t.Errorf("%s = %d, want 2 (columns %q)", d.countColumn, got, rows[0].Columns())
		}
	})
	t.Run("nested alias and JSON", func(t *testing.T) {
		q := fmt.Sprintf("SELECT id, code AS %s, zip AS %s FROM items ORDER BY id",
			d.quote("item.code"), d.quote("item.zip"))
		rows, err := godyno.QueryToStruct(db, q)
		if err != nil {
			t.Fatal(err)
		}
		if rows[0].GetString("item.zip") != "007" {
			t.Errorf("item.zip = %q", rows[0].GetString("item.zip"))
		}
		out, err := json.Marshal(rows[0])
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), `"item":{"code":"abc","zip":"007"}`) {
			t.Errorf("json = %s", out)
		}
	})
	t.Run("duplicate columns are an error", func(t *testing.T) {
		_, err := godyno.QueryToStruct(db, "SELECT id, id FROM items")
		if err == nil || !strings.Contains(err.Error(), "duplicate column") {
			t.Errorf("err = %v, want duplicate column", err)
		}
	})
}

func checkItems(t *testing.T, rows []*godyno.DBResult) {
	t.Helper()
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	a, b := rows[0], rows[1]

	if got := a.GetString("code"); got != "abc" {
		t.Errorf("row 1 code = %q, want abc", got)
	}
	if got, ok := b.GetIntOk("code"); !ok || got != 42 {
		t.Errorf("row 2 code GetIntOk = (%d, %v), want (42, true)", got, ok)
	}
	if got := a.GetString("zip"); got != "007" {
		t.Errorf("zip = %q, want 007", got)
	}
	if got := a.GetFloat("amount"); got != 100 {
		t.Errorf("row 1 amount = %v, want 100", got)
	}
	if got := b.GetFloat("amount"); got != 12.5 {
		t.Errorf("row 2 amount = %v, want 12.5", got)
	}
	if !a.GetBool("active") || b.GetBool("active") {
		t.Errorf("active = %v, %v, want true, false", a.GetBool("active"), b.GetBool("active"))
	}
	if ts, ok := a.GetTimeOk("created_at"); !ok || !ts.Equal(created) {
		t.Errorf("created_at = (%v, %v), want %v (raw %#v)", ts, ok, created, a.Get("created_at"))
	}
	if !a.IsNull("note") || b.GetString("note") != "x" {
		t.Errorf("note: IsNull = %v, row 2 = %q", a.IsNull("note"), b.GetString("note"))
	}
	if _, err := json.Marshal(rows); err != nil {
		t.Errorf("json: %v", err)
	}
}
