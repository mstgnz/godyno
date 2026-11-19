package godyno

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Strict conversions report false for NULL, for a type that does not convert,
// and for a value that would lose information (12.9 to an integer).

func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case int32:
		return int64(x), true
	case int16:
		return int64(x), true
	case int8:
		return int64(x), true
	case uint64:
		if x > math.MaxInt64 {
			return 0, false
		}
		return int64(x), true
	case uint:
		if uint64(x) > math.MaxInt64 {
			return 0, false
		}
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint8:
		return int64(x), true
	case float64:
		return floatToInt64(x)
	case float32:
		return floatToInt64(float64(x))
	case string:
		return parseInt64(x)
	case []byte:
		return parseInt64(string(x))
	}
	return 0, false
}

// decimal is numeric text split into parts. Integer conversions read intPart
// with ParseInt, never through float64, which would round above 2^53.
type decimal struct {
	text    string // trimmed input
	sign    string
	intPart string
	frac    string
	hasExp  bool
}

// parseDecimal accepts [+-]digits[.digits][e[+-]digits], the form databases
// send. It refuses what strconv also accepts: underscores, hex, Inf and NaN.
func parseDecimal(s string) (decimal, bool) {
	d := decimal{text: strings.TrimSpace(s)}
	s = d.text
	if s != "" && (s[0] == '+' || s[0] == '-') {
		d.sign, s = s[:1], s[1:]
	}
	i := digitsEnd(s, 0)
	d.intPart = s[:i]
	if i < len(s) && s[i] == '.' {
		j := digitsEnd(s, i+1)
		d.frac, i = s[i+1:j], j
	}
	if d.intPart == "" && d.frac == "" {
		return d, false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		k := digitsEnd(s, j)
		if k == j {
			return d, false
		}
		d.hasExp, i = true, k
	}
	return d, i == len(s)
}

func digitsEnd(s string, i int) int {
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}

// wholePart returns the integer part with its sign, truncating any fraction.
func (d decimal) wholePart() (int64, bool) {
	if d.intPart == "" {
		return 0, true
	}
	i, err := strconv.ParseInt(d.sign+d.intPart, 10, 64)
	if err != nil {
		return 0, false // out of int64 range
	}
	return i, true
}

func parseInt64(s string) (int64, bool) {
	d, ok := parseDecimal(s)
	// "12.00" from a Postgres NUMERIC is whole; "12.5" and "1e3" are refused.
	if !ok || d.hasExp || strings.Trim(d.frac, "0") != "" {
		return 0, false
	}
	return d.wholePart()
}

// lenientInt64 truncates "12.9" to 12 without going through float64.
func lenientInt64(s string) int64 {
	d, ok := parseDecimal(s)
	if !ok {
		return 0
	}
	if !d.hasExp {
		i, _ := d.wholePart()
		return i
	}
	f, err := strconv.ParseFloat(d.text, 64)
	if err != nil {
		return 0
	}
	i, _ := floatToInt64(math.Trunc(f))
	return i
}

// 2^63 is exactly representable as float64; every float below it fits in int64.
const twoPow63 = float64(1 << 63)

func floatToInt64(f float64) (int64, bool) {
	if f != math.Trunc(f) || f < -twoPow63 || f >= twoPow63 {
		return 0, false // also rejects NaN and ±Inf
	}
	return int64(f), true
}

// toInt64Lenient truncates fractions instead of refusing them.
func toInt64Lenient(v any) int64 {
	if i, ok := toInt64(v); ok {
		return i
	}
	switch x := v.(type) {
	case string:
		return lenientInt64(x)
	case []byte:
		return lenientInt64(string(x))
	}
	f, ok := toFloat64(v)
	if !ok {
		return 0
	}
	i, _ := floatToInt64(math.Trunc(f))
	return i
}

func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case string:
		return parseFloat64(x)
	case []byte:
		return parseFloat64(string(x))
	case bool, nil:
		return 0, false
	}
	if i, ok := toInt64(v); ok {
		return float64(i), true
	}
	if u, ok := v.(uint64); ok {
		return float64(u), true
	}
	if u, ok := v.(uint); ok {
		return float64(u), true
	}
	return 0, false
}

// parseFloat64 refuses out-of-range text such as "1e999" instead of returning ±Inf.
func parseFloat64(s string) (float64, bool) {
	d, ok := parseDecimal(s)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(d.text, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func toBool(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		return parseBool(x)
	case []byte:
		return parseBool(string(x))
	case float64, float32, nil:
		return false, false
	}
	if i, ok := toInt64(v); ok {
		return i != 0, true
	}
	if _, ok := v.(uint64); ok {
		return true, true // only values above MaxInt64 reach here, all non-zero
	}
	if _, ok := v.(uint); ok {
		return true, true
	}
	return false, false
}

func parseBool(s string) (bool, bool) {
	// MySQL BIT(1) arrives as a single raw byte.
	if len(s) == 1 && s[0] <= 1 {
		return s[0] == 1, true
	}
	b, err := strconv.ParseBool(strings.TrimSpace(s))
	return b, err == nil
}

// toString formats any non-NULL value; it never fails.
func toString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	}
	if i, ok := toInt64(v); ok {
		return strconv.FormatInt(i, 10)
	}
	return fmt.Sprint(v)
}

// timeLayouts covers the text forms Postgres, MySQL (without parseTime=true)
// and SQLite use. A layout without a zone is read as UTC.
var timeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999Z07",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	time.DateOnly,
}

func toTime(v any) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x, true
	case string:
		return parseTime(x)
	case []byte:
		return parseTime(string(x))
	}
	return time.Time{}, false
}

func parseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
