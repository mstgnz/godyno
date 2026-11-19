package godyno

import (
	"reflect"
	"strings"
	"testing"
)

// These cases crashed or lost data in v1.1.0, which guessed column types from
// text and built a struct per row with reflection.

func TestRegressionMixedContentColumns(t *testing.T) {
	b := func(s string) []byte { return []byte(s) }

	t.Run("MySQL text protocol varchar: text then digits", func(t *testing.T) {
		res, err := queryMock(t, []string{"code"}, []any{b("abc")}, []any{b("42")})
		if err != nil {
			t.Fatal(err)
		}
		if got := res[0].GetString("code"); got != "abc" {
			t.Errorf("row 0 = %q, want abc", got)
		}
		if got, ok := res[1].GetIntOk("code"); !ok || got != 42 {
			t.Errorf("row 1 GetIntOk = (%d, %v), want (42, true)", got, ok)
		}
	})

	t.Run("Postgres NUMERIC: whole then fraction", func(t *testing.T) {
		res, err := queryMock(t, []string{"amount"}, []any{b("100")}, []any{b("12.5")})
		if err != nil {
			t.Fatal(err)
		}
		if got := res[0].GetFloat("amount"); got != 100 {
			t.Errorf("row 0 = %v, want 100", got)
		}
		if got := res[1].GetFloat("amount"); got != 12.5 {
			t.Errorf("row 1 = %v, want 12.5", got)
		}
		if _, ok := res[1].GetIntOk("amount"); ok {
			t.Error("GetIntOk(12.5) ok = true, want false")
		}
		if got := res[1].GetInt("amount"); got != 12 {
			t.Errorf("GetInt(12.5) = %d, want 12", got)
		}
	})

	t.Run("SQLite dynamic typing: int then float", func(t *testing.T) {
		res, err := queryMock(t, []string{"v"}, []any{int64(1)}, []any{2.5})
		if err != nil {
			t.Fatal(err)
		}
		if res[0].GetInt("v") != 1 || res[1].GetFloat("v") != 2.5 {
			t.Errorf("got %d and %v, want 1 and 2.5", res[0].GetInt("v"), res[1].GetFloat("v"))
		}
	})

	t.Run("NaN in a name column stays text", func(t *testing.T) {
		res, err := queryMock(t, []string{"name"}, []any{b("NaN")}, []any{b("Ali")})
		if err != nil {
			t.Fatal(err)
		}
		if res[0].GetString("name") != "NaN" || res[1].GetString("name") != "Ali" {
			t.Errorf("got %q and %q", res[0].GetString("name"), res[1].GetString("name"))
		}
	})

	t.Run("t/f text then other text", func(t *testing.T) {
		res, err := queryMock(t, []string{"flag"}, []any{b("x")}, []any{b("t")})
		if err != nil {
			t.Fatal(err)
		}
		if res[0].GetString("flag") != "x" || !res[1].GetBool("flag") {
			t.Errorf("got %q and %v", res[0].GetString("flag"), res[1].GetBool("flag"))
		}
	})
}

func TestRegressionTextKeepsItsDigits(t *testing.T) {
	cases := map[string]string{
		"zip":   "007",
		"phone": "05321234567",
		"id":    "123456789012345678901",
		"code":  "1e5",
	}
	for col, want := range cases {
		res, err := queryMock(t, []string{col}, []any{[]byte(want)})
		if err != nil {
			t.Fatal(err)
		}
		if got := res[0].GetString(col); got != want {
			t.Errorf("%s = %q, want %q", col, got, want)
		}
		if got := res[0].Get(col); got != want {
			t.Errorf("Get(%s) = %#v, want the string %q", col, got, want)
		}
	}
}

func TestRegressionColumnNames(t *testing.T) {
	cases := []struct {
		column string
		field  string
	}{
		{"count(*)", "count(*)"},     // MySQL aggregate without alias
		{"?column?", "?column?"},     // Postgres expression without alias
		{"_id", "_id"},               // leading underscore
		{"1st", "1st"},               // leading digit
		{"first name", "first name"}, // space
		{"şehir", "şehir"},           // non-ASCII
		{"şehir", "Şehir"},           // first letter is case-insensitive, as in v1.1
		{"", ""},                     // some drivers report empty names
		{"a.b.c", "a.b.c"},           // three levels
		{"Category.Name", "category.name"},
	}
	for _, c := range cases {
		res, err := queryMock(t, []string{c.column}, []any{int64(3)})
		if err != nil {
			t.Errorf("column %q: %v", c.column, err)
			continue
		}
		if got := res[0].GetInt(c.field); got != 3 {
			t.Errorf("column %q, field %q = %d, want 3", c.column, c.field, got)
		}
	}
}

func TestRegressionThreeLevelNesting(t *testing.T) {
	res, err := queryMock(t, []string{"a.b.c", "a.b.d", "a.e"}, []any{"x", "y", "z"})
	if err != nil {
		t.Fatal(err)
	}
	r := res[0]
	if r.GetString("a.b.c") != "x" || r.GetString("a.b.d") != "y" || r.GetString("a.e") != "z" {
		t.Errorf("leaf values wrong: %v", r.ToMap())
	}
	want := map[string]any{"b": map[string]any{"c": "x", "d": "y"}, "e": "z"}
	if got := r.Get("a"); !reflect.DeepEqual(got, want) {
		t.Errorf("Get(a) = %#v, want %#v", got, want)
	}
	if got := r.Get("A.B"); !reflect.DeepEqual(got, want["b"]) {
		t.Errorf("Get(A.B) = %#v, want %#v", got, want["b"])
	}
}

func TestRegressionAmbiguousColumnsReturnError(t *testing.T) {
	cases := []struct {
		name    string
		columns []string
		want    string
	}{
		{"join without alias", []string{"id", "id"}, "duplicate column"},
		{"case of first letter", []string{"id", "Id"}, "duplicate column"},
		{"nested case", []string{"category.name", "Category.Name"}, "duplicate column"},
		{"value and group", []string{"category", "category.name"}, "conflicts with nested"},
		{"group then value", []string{"a.b.c", "a.b"}, "conflicts with nested"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			row := make([]any, len(c.columns))
			for i := range row {
				row[i] = int64(i)
			}
			_, err := queryMock(t, c.columns, row)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestRegressionNoPanics(t *testing.T) {
	r := newTestResult(t, []string{"title"}, "x")
	if got := r.GetString("title.x"); got != "" {
		t.Errorf("path through a scalar = %q, want empty", got)
	}
	if r.HasField("title.x") {
		t.Error("HasField(title.x) = true, want false")
	}

	var zero DBResult
	var nilResult *DBResult
	for name, dr := range map[string]*DBResult{"New()": New(), "zero value": &zero, "nil pointer": nilResult} {
		if dr.GetString("x") != "" || dr.GetInt("x") != 0 || dr.HasField("x") || dr.Get("x") != nil ||
			dr.IsNull("x") || dr.Columns() != nil || len(dr.ToMap()) != 0 {
			t.Errorf("%s does not behave as an empty row", name)
		}
	}
}

func TestRegressionMySQLTinyintIsBool(t *testing.T) {
	r := newTestResult(t, []string{"active", "inactive"}, int64(1), int64(0))
	if !r.GetBool("active") || r.GetBool("inactive") {
		t.Errorf("GetBool(1) = %v, GetBool(0) = %v", r.GetBool("active"), r.GetBool("inactive"))
	}
}

func TestRegressionNullIsNotZero(t *testing.T) {
	res, err := queryMock(t, []string{"id", "score"}, []any{int64(1), nil}, []any{int64(2), int64(0)})
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := res[0].GetIntOk("score"); ok {
		t.Errorf("NULL GetIntOk = (%d, true), want ok false", v)
	}
	if v, ok := res[1].GetIntOk("score"); !ok || v != 0 {
		t.Errorf("zero GetIntOk = (%d, %v), want (0, true)", v, ok)
	}
	if !res[0].IsNull("score") || res[1].IsNull("score") {
		t.Error("IsNull does not tell NULL from 0")
	}
	if res[0].Get("score") != nil {
		t.Errorf("Get(NULL) = %#v, want nil", res[0].Get("score"))
	}
	if _, ok := res[0].GetStringOk("score"); ok {
		t.Error("GetStringOk(NULL) ok = true, want false")
	}
}

func TestRegressionColumnOrderIsStable(t *testing.T) {
	cols := []string{"c", "a", "b", "category.name"}
	res, err := queryMock(t, cols, []any{"1", "2", "3", "4"}, []any{"5", "6", "7", "8"})
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range res {
		if got := r.Columns(); !reflect.DeepEqual(got, cols) {
			t.Errorf("row %d Columns() = %q, want %q", i, got, cols)
		}
	}
	if res[0].r.meta != res[1].r.meta {
		t.Error("rows of one result set should share column metadata")
	}
}
