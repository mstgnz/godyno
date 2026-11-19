package godyno

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// columnMeta describes one result set and is shared by all of its rows.
type columnMeta struct {
	columns []string
	keys    []string // normalized column names
	// paths holds each column's segments for ToMap and Get on a group. A segment
	// shared by several columns takes the spelling of the first one, so "x.y"
	// and "X.z" form one group, as they do for lookups.
	paths [][]string
	// index maps both the original and the normalized column name to its position.
	index map[string]int
	// parents maps every normalized dot prefix ("Category" for "category.name")
	// to the position of the first column under it.
	parents map[string]int
}

// newColumnMeta rejects names that would make a lookup ambiguous: two columns
// that normalize to the same name, or a column that is also a nested prefix.
func newColumnMeta(columns []string) (*columnMeta, error) {
	m := &columnMeta{
		columns: columns,
		keys:    make([]string, len(columns)),
		paths:   make([][]string, len(columns)),
		index:   make(map[string]int, 2*len(columns)),
		parents: make(map[string]int),
	}
	spelling := make(map[string]string) // normalized prefix -> first original segment
	for i, col := range columns {
		key := normalize(col)
		if j, dup := m.index[key]; dup {
			return nil, fmt.Errorf("godyno: duplicate column %q (same as %q), give one of them an alias", col, columns[j])
		}
		m.keys[i] = key
		m.index[key] = i

		// normalize keeps the number of dots, so both splits line up.
		segs, nsegs := strings.Split(col, "."), strings.Split(key, ".")
		for k := range segs {
			prefix := strings.Join(nsegs[:k+1], ".")
			if s, seen := spelling[prefix]; seen {
				segs[k] = s
			} else {
				spelling[prefix] = segs[k]
			}
		}
		m.paths[i] = segs

		for p := 0; ; {
			n := strings.IndexByte(key[p:], '.')
			if n < 0 {
				break
			}
			if _, ok := m.parents[key[:p+n]]; !ok {
				m.parents[key[:p+n]] = i
			}
			p += n + 1
		}
	}
	for i, key := range m.keys {
		if child, ok := m.parents[key]; ok {
			return nil, fmt.Errorf("godyno: column %q conflicts with nested column %q, rename one of them", columns[i], columns[child])
		}
	}
	// The original spelling is a second key for a zero-allocation lookup. It cannot
	// collide with another column's key: normalize is idempotent, so a collision
	// would already have been reported as a duplicate above.
	for i, col := range columns {
		m.index[col] = i
	}
	return m, nil
}

// normalize upper-cases the first rune of every dot-separated segment, so "id",
// "Id", "category.name" and "Category.Name" resolve the same way they did in v1.1.
func normalize(name string) string {
	first := true
	for i, r := range name {
		if first && unicode.ToUpper(r) != r {
			return normalizeFrom(name, i)
		}
		first = r == '.'
	}
	return name
}

func normalizeFrom(name string, start int) string {
	var b strings.Builder
	b.Grow(len(name) + utf8.UTFMax)
	b.WriteString(name[:start])
	first := true
	for _, r := range name[start:] {
		if first {
			r = unicode.ToUpper(r)
		}
		first = r == '.'
		b.WriteRune(r)
	}
	return b.String()
}
