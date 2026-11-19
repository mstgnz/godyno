package godyno

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestGettersOnRow(t *testing.T) {
	created := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	r := newTestResult(t,
		[]string{"id", "price", "active", "created_at", "note", "big"},
		int64(7), 12.9, true, created, nil, int64(1)<<40)

	if v, ok := r.GetInt64Ok("big"); !ok || v != 1<<40 {
		t.Errorf("GetInt64Ok(big) = (%d, %v)", v, ok)
	}
	if r.GetInt64("price") != 12 {
		t.Errorf("GetInt64(12.9) = %d, want 12", r.GetInt64("price"))
	}
	if v, ok := r.GetIntOk("id"); !ok || v != 7 {
		t.Errorf("GetIntOk(id) = (%d, %v)", v, ok)
	}
	if _, ok := r.GetIntOk("missing"); ok {
		t.Error("GetIntOk(missing) ok = true")
	}
	if v, ok := r.GetFloatOk("price"); !ok || v != 12.9 {
		t.Errorf("GetFloatOk(price) = (%v, %v)", v, ok)
	}
	if v, ok := r.GetBoolOk("active"); !ok || !v {
		t.Errorf("GetBoolOk(active) = (%v, %v)", v, ok)
	}
	if !r.GetTime("created_at").Equal(created) {
		t.Errorf("GetTime = %v", r.GetTime("created_at"))
	}
	if _, ok := r.GetTimeOk("note"); ok {
		t.Error("GetTimeOk(NULL) ok = true")
	}
	if got := r.GetString("created_at"); got != "2026-01-02T15:04:05Z" {
		t.Errorf("GetString(time) = %q", got)
	}
	if v, ok := r.GetStringOk("id"); !ok || v != "7" {
		t.Errorf("GetStringOk(id) = (%q, %v)", v, ok)
	}
	if !r.HasField("note") || !r.IsNull("note") || r.IsNull("id") || r.IsNull("missing") {
		t.Error("HasField/IsNull disagree on the NULL column")
	}
}

func TestNestedGroup(t *testing.T) {
	r := newTestResult(t, []string{"id", "category.name", "category.id"}, int64(1), "Books", int64(5))
	if !r.HasField("category") || !r.HasField("Category") {
		t.Error("HasField(category) = false, want true for a nested group")
	}
	want := map[string]any{"name": "Books", "id": int64(5)}
	if got := r.Get("category"); !reflect.DeepEqual(got, want) {
		t.Errorf("Get(category) = %#v, want %#v", got, want)
	}
	if r.GetString("category") != "" {
		t.Error("a group is not a string")
	}
	if r.IsNull("category") {
		t.Error("a group is not NULL")
	}
}

func TestToMapAndJSON(t *testing.T) {
	created := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	r := newTestResult(t,
		[]string{"id", "title", "active", "score", "created_at", "category.name", "category.id"},
		int64(1), "Go", true, nil, created, "Books", int64(5))

	wantMap := map[string]any{
		"id": int64(1), "title": "Go", "active": true, "score": nil, "created_at": created,
		"category": map[string]any{"name": "Books", "id": int64(5)},
	}
	if got := r.ToMap(); !reflect.DeepEqual(got, wantMap) {
		t.Errorf("ToMap() = %#v", got)
	}

	got, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"active":true,"category":{"id":5,"name":"Books"},"created_at":"2026-01-02T15:04:05Z","id":1,"score":null,"title":"Go"}`
	if string(got) != want {
		t.Errorf("json = %s\nwant   %s", got, want)
	}

	list, err := json.Marshal([]*DBResult{r, nil})
	if err != nil {
		t.Fatal(err)
	}
	if string(list) != "["+want+",null]" {
		t.Errorf("slice json = %s", list)
	}
}

func TestColumnsIsACopy(t *testing.T) {
	r := newTestResult(t, []string{"a", "b"}, int64(1), int64(2))
	cols := r.Columns()
	cols[0] = "changed"
	if r.Columns()[0] != "a" {
		t.Error("Columns() exposes internal state")
	}
}

func TestMixedCaseGroupsMatchBetweenGetAndJSON(t *testing.T) {
	r := newTestResult(t, []string{"x.y", "X.z", "x.a.b", "X.A.c"}, int64(1), int64(2), int64(3), int64(4))
	wantGroup := map[string]any{"y": int64(1), "z": int64(2), "a": map[string]any{"b": int64(3), "c": int64(4)}}
	if got := r.Get("x"); !reflect.DeepEqual(got, wantGroup) {
		t.Errorf("Get(x) = %#v, want %#v", got, wantGroup)
	}
	if got := r.ToMap(); !reflect.DeepEqual(got, map[string]any{"x": wantGroup}) {
		t.Errorf("ToMap() = %#v", got)
	}
	out, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"x":{"a":{"b":3,"c":4},"y":1,"z":2}}`; string(out) != want {
		t.Errorf("json = %s, want %s", out, want)
	}
}

func TestJSONNonFiniteFloatsBecomeNull(t *testing.T) {
	r := newTestResult(t, []string{"nan", "inf", "f32", "ok"},
		math.NaN(), math.Inf(-1), float32(math.Inf(1)), 1.5)
	out, err := json.Marshal([]*DBResult{r})
	if err != nil {
		t.Fatalf("one non-finite value failed the whole slice: %v", err)
	}
	if want := `[{"f32":null,"inf":null,"nan":null,"ok":1.5}]`; string(out) != want {
		t.Errorf("json = %s, want %s", out, want)
	}
	if f, ok := r.ToMap()["nan"].(float64); !ok || !math.IsNaN(f) {
		t.Error("ToMap should keep the NaN for Go callers")
	}
}

func TestDBResultIsComparable(t *testing.T) {
	// v1.1 callers could compare DBResult values and use them as map keys.
	a, b := *newTestResult(t, []string{"id"}, int64(1)), *newTestResult(t, []string{"id"}, int64(1))
	seen := map[DBResult]bool{a: true}
	if a == b || seen[b] || !seen[a] {
		t.Error("comparison should be by row identity")
	}
}

func TestDeprecatedFieldInfoStillCompiles(t *testing.T) {
	_ = FieldInfo{Name: "id", Type: reflect.TypeFor[int]()}
}
