package godyno

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
)

// DBResult holds one row of a query result.
//
// Each column keeps the value the driver returned (int64, float64, bool,
// string, time.Time or nil); []byte is stored as string and never guessed into
// another type. Field names use dot notation for nested columns, and the first
// letter of each segment is case-insensitive ("id" and "Id" are the same field).
//
// The plain getters are lenient: they return the zero value for a missing field
// or NULL and truncate 12.9 to 12. The Ok variants are strict and report false
// for a missing field, NULL, or a value that does not convert without loss.
// A zero DBResult, New() and a nil *DBResult behave as a row with no columns.
type DBResult struct {
	// A pointer keeps DBResult comparable, as it was in v1.1.
	r *row
}

type row struct {
	meta *columnMeta
	vals []any
}

func (dr *DBResult) row() *row {
	if dr == nil {
		return nil
	}
	return dr.r
}

func (dr *DBResult) lookup(field string) (any, bool) {
	r := dr.row()
	if r == nil {
		return nil, false
	}
	i, ok := r.meta.index[field]
	if !ok {
		if i, ok = r.meta.index[normalize(field)]; !ok {
			return nil, false
		}
	}
	return r.vals[i], true
}

func (dr *DBResult) isParent(field string) bool {
	r := dr.row()
	if r == nil {
		return false
	}
	_, ok := r.meta.parents[normalize(field)]
	return ok
}

// HasField reports whether field is a column or a nested group ("category" for
// "category.name"). A NULL column still exists; use IsNull to tell them apart.
func (dr *DBResult) HasField(field string) bool {
	_, ok := dr.lookup(field)
	return ok || dr.isParent(field)
}

// IsNull reports whether field exists and is NULL.
func (dr *DBResult) IsNull(field string) bool {
	v, ok := dr.lookup(field)
	return ok && v == nil
}

// Get returns the stored value of field, nil for NULL or a missing field, and a
// map[string]any for a nested group such as "category".
func (dr *DBResult) Get(field string) any {
	if v, ok := dr.lookup(field); ok {
		return v
	}
	if dr.isParent(field) {
		return dr.row().nested(normalize(field), false)
	}
	return nil
}

// Columns returns the column names in query order.
func (dr *DBResult) Columns() []string {
	r := dr.row()
	if r == nil {
		return nil
	}
	return append([]string(nil), r.meta.columns...)
}

// ToMap returns the row as a map, with dotted columns nested:
// "category.name" becomes m["category"].(map[string]any)["name"].
func (dr *DBResult) ToMap() map[string]any {
	r := dr.row()
	if r == nil {
		return map[string]any{}
	}
	return r.nested("", false)
}

// MarshalJSON encodes the row as ToMap does, so a row or a slice of rows can be
// returned from an HTTP handler directly. time.Time values use RFC 3339, and
// NaN and ±Inf, which JSON cannot hold, become null instead of failing the
// whole response.
func (dr *DBResult) MarshalJSON() ([]byte, error) {
	r := dr.row()
	if r == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(r.nested("", true))
}

// nested builds the map of every column under a normalized prefix; "" means all.
func (r *row) nested(prefix string, forJSON bool) map[string]any {
	depth := 0
	if prefix != "" {
		depth = strings.Count(prefix, ".") + 1
		prefix += "."
	}
	m := make(map[string]any)
	for i, key := range r.meta.keys {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		v := r.vals[i]
		if forJSON && !finite(v) {
			v = nil
		}
		setPath(m, r.meta.paths[i][depth:], v)
	}
	return m
}

func finite(v any) bool {
	switch f := v.(type) {
	case float64:
		return !math.IsNaN(f) && !math.IsInf(f, 0)
	case float32:
		return !math.IsNaN(float64(f)) && !math.IsInf(float64(f), 0)
	}
	return true
}

// setPath stores v under path. newColumnMeta guarantees that no value and group
// share a path, so the type assertion below only fails for a missing entry.
func setPath(m map[string]any, path []string, v any) {
	for _, seg := range path[:len(path)-1] {
		next, ok := m[seg].(map[string]any)
		if !ok {
			next = make(map[string]any)
			m[seg] = next
		}
		m = next
	}
	m[path[len(path)-1]] = v
}

// GetString returns field as text. NULL and a missing field give "".
func (dr *DBResult) GetString(field string) string {
	v, _ := dr.lookup(field)
	return toString(v)
}

// GetStringOk returns field as text, and false when it is missing or NULL.
func (dr *DBResult) GetStringOk(field string) (string, bool) {
	v, ok := dr.lookup(field)
	if !ok || v == nil {
		return "", false
	}
	return toString(v), true
}

// GetInt returns field as an int, truncating fractions. It returns 0 when the
// field is missing, NULL or not numeric.
func (dr *DBResult) GetInt(field string) int {
	v, _ := dr.lookup(field)
	return int(toInt64Lenient(v))
}

// GetIntOk returns field as an int, and false when it is missing, NULL, not a
// whole number, or outside the int range.
func (dr *DBResult) GetIntOk(field string) (int, bool) {
	i, ok := dr.GetInt64Ok(field)
	if !ok || (strconv.IntSize == 32 && (i < math.MinInt32 || i > math.MaxInt32)) {
		return 0, false
	}
	return int(i), true
}

// GetInt64 is GetInt for values that need 64 bits on every platform.
func (dr *DBResult) GetInt64(field string) int64 {
	v, _ := dr.lookup(field)
	return toInt64Lenient(v)
}

// GetInt64Ok returns field as an int64, and false when it is missing, NULL or
// not a whole number.
func (dr *DBResult) GetInt64Ok(field string) (int64, bool) {
	v, _ := dr.lookup(field)
	return toInt64(v)
}

// GetFloat returns field as a float64, or 0 when it is missing, NULL or not numeric.
func (dr *DBResult) GetFloat(field string) float64 {
	f, _ := dr.GetFloatOk(field)
	return f
}

// GetFloatOk returns field as a float64, and false when it is missing, NULL or not numeric.
func (dr *DBResult) GetFloatOk(field string) (float64, bool) {
	v, _ := dr.lookup(field)
	return toFloat64(v)
}

// GetBool returns field as a bool. Integers are true when non-zero; text
// accepts the strconv.ParseBool forms ("1", "t", "true", ...).
func (dr *DBResult) GetBool(field string) bool {
	b, _ := dr.GetBoolOk(field)
	return b
}

// GetBoolOk returns field as a bool, and false when it is missing, NULL or not boolean.
func (dr *DBResult) GetBoolOk(field string) (bool, bool) {
	v, _ := dr.lookup(field)
	return toBool(v)
}

// GetTime returns field as a time.Time, or the zero time when it is missing,
// NULL or not a date. Text without a zone is read as UTC; for MySQL, prefer
// parseTime=true and loc in the DSN so the driver returns time.Time itself.
func (dr *DBResult) GetTime(field string) time.Time {
	t, _ := dr.GetTimeOk(field)
	return t
}

// GetTimeOk returns field as a time.Time, and false when it is missing, NULL or not a date.
func (dr *DBResult) GetTimeOk(field string) (time.Time, bool) {
	v, _ := dr.lookup(field)
	return toTime(v)
}
