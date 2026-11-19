package godyno

import (
	"math"
	"testing"
	"time"
)

func TestToInt64(t *testing.T) {
	cases := []struct {
		in     any
		want   int64
		wantOk bool
	}{
		{int64(5), 5, true},
		{int(5), 5, true},
		{int32(-5), -5, true},
		{int16(5), 5, true},
		{int8(5), 5, true},
		{uint64(5), 5, true},
		{uint64(math.MaxUint64), 0, false},
		{uint(5), 5, true},
		{uint32(5), 5, true},
		{uint16(5), 5, true},
		{uint8(5), 5, true},
		{3.0, 3, true},
		{3.5, 0, false},
		{float32(2), 2, true},
		{math.NaN(), 0, false},
		{math.Inf(1), 0, false},
		{1e19, 0, false},
		{-9.3e18, 0, false},
		{"42", 42, true},
		{" 42 ", 42, true}, // Postgres char(n) pads with spaces
		{"12.00", 12, true},
		{"12.5", 0, false},
		{"abc", 0, false},
		{"123456789012345678901", 0, false},
		{"9007199254740993.00", 9007199254740993, true}, // above 2^53: must not round
		{"9007199254740993.5", 0, false},
		{"12.00000000000000000001", 0, false},
		{"+42", 42, true},
		{"12.", 12, true},
		{".0", 0, true},
		{"1e3", 0, false},
		{"1_000", 0, false},
		{"0x10", 0, false},
		{"Infinity", 0, false},
		{"", 0, false},
		{"-", 0, false},
		{".", 0, false},
		{"1e", 0, false},
		{[]byte("7"), 7, true},
		{true, 0, false},
		{nil, 0, false},
		{time.Now(), 0, false},
	}
	for _, c := range cases {
		got, ok := toInt64(c.in)
		if got != c.want || ok != c.wantOk {
			t.Errorf("toInt64(%#v) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.wantOk)
		}
	}
}

func TestToInt64Lenient(t *testing.T) {
	cases := []struct {
		in   any
		want int64
	}{
		{12.9, 12},
		{-12.9, -12},
		{"12.9", 12},
		{"-12.9", -12},
		{"9007199254740993.9", 9007199254740993},
		{".5", 0},
		{"1.5e3", 1500},
		{"1e999", 0},
		{"123456789012345678901.5", 0},
		{[]byte("7.5"), 7},
		{"1_000", 0},
		{int64(7), 7},
		{math.NaN(), 0},
		{1e30, 0},
		{true, 0},
		{nil, 0},
		{"abc", 0},
	}
	for _, c := range cases {
		if got := toInt64Lenient(c.in); got != c.want {
			t.Errorf("toInt64Lenient(%#v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestToFloat64(t *testing.T) {
	cases := []struct {
		in     any
		want   float64
		wantOk bool
	}{
		{1.5, 1.5, true},
		{float32(1.5), 1.5, true},
		{int64(3), 3, true},
		{int8(2), 2, true},
		{uint64(math.MaxUint64), float64(math.MaxUint64), true},
		{uint(4), 4, true},
		{"3.25", 3.25, true},
		{" 3.25 ", 3.25, true},
		{[]byte("1.5"), 1.5, true},
		{"1e5", 100000, true},
		{"+1.5E-2", 0.015, true},
		{"-.5", -0.5, true},
		{"1_000", 0, false},
		{"0x1p4", 0, false},
		{"Infinity", 0, false},
		{"NaN", 0, false},
		{"1e999", 0, false},
		{"abc", 0, false},
		{true, 0, false},
		{nil, 0, false},
		{time.Now(), 0, false},
	}
	for _, c := range cases {
		got, ok := toFloat64(c.in)
		if got != c.want || ok != c.wantOk {
			t.Errorf("toFloat64(%#v) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.wantOk)
		}
	}
}

func TestToBool(t *testing.T) {
	cases := []struct {
		in     any
		want   bool
		wantOk bool
	}{
		{true, true, true},
		{false, false, true},
		{int64(0), false, true},
		{int64(1), true, true},
		{int64(2), true, true},
		{uint(0), false, true},
		{uint64(math.MaxUint64), true, true},
		{uint(math.MaxUint), true, true},
		{"1", true, true},
		{"0", false, true},
		{"t", true, true},
		{"f", false, true},
		{"TRUE", true, true},
		{" true ", true, true},
		{"\x01", true, true}, // MySQL BIT(1)
		{"\x00", false, true},
		{[]byte("true"), true, true},
		{"yes", false, false},
		{1.0, false, false},
		{float32(1), false, false},
		{nil, false, false},
		{time.Now(), false, false},
	}
	for _, c := range cases {
		got, ok := toBool(c.in)
		if got != c.want || ok != c.wantOk {
			t.Errorf("toBool(%#v) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.wantOk)
		}
	}
}

func TestToString(t *testing.T) {
	ts := time.Date(2026, 1, 2, 15, 4, 5, 123000000, time.UTC)
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"x", "x"},
		{[]byte("y"), "y"},
		{ts, "2026-01-02T15:04:05.123Z"},
		{true, "true"},
		{12.5, "12.5"},
		{1e21, "1000000000000000000000"},
		{float32(1.5), "1.5"},
		{int64(-3), "-3"},
		{uint64(math.MaxUint64), "18446744073709551615"},
		{int8(4), "4"},
		{struct{ A int }{1}, "{1}"},
	}
	for _, c := range cases {
		if got := toString(c.in); got != c.want {
			t.Errorf("toString(%#v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestToTime(t *testing.T) {
	plus3 := time.FixedZone("", 3*3600)
	plus530 := time.FixedZone("", 5*3600+1800)
	cases := []struct {
		in     any
		want   time.Time
		wantOk bool
	}{
		{time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), true},
		{"2026-01-02T15:04:05Z", time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC), true},
		{"2026-01-02 15:04:05", time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC), true},
		{"2026-01-02 15:04:05.5", time.Date(2026, 1, 2, 15, 4, 5, 5e8, time.UTC), true},
		{"2026-01-02 15:04:05+03", time.Date(2026, 1, 2, 15, 4, 5, 0, plus3), true},
		{"2026-01-02 15:04:05.123+05:30", time.Date(2026, 1, 2, 15, 4, 5, 123e6, plus530), true},
		{"2026-01-02T15:04:05", time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC), true},
		{"2026-01-02", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), true},
		{[]byte("2026-01-02"), time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), true},
		{"0000-00-00 00:00:00", time.Time{}, false}, // MySQL zero date
		{"abc", time.Time{}, false},
		{int64(1700000000), time.Time{}, false},
		{nil, time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := toTime(c.in)
		if !got.Equal(c.want) || ok != c.wantOk {
			t.Errorf("toTime(%#v) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.wantOk)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"":              "",
		"id":            "Id",
		"Id":            "Id",
		"ID":            "ID",
		"category.name": "Category.Name",
		"Category.name": "Category.Name",
		"a..b":          "A..B",
		".x":            ".X",
		"şehir.ılçe":    "Şehir.Ilçe",
		"count(*)":      "Count(*)",
		"_id":           "_id",
	}
	for in, want := range cases {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
