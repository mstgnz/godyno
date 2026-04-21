package godyno

import (
	"reflect"
	"testing"
)

// buildBenchResult returns a DBResult with a realistic set of mixed-type fields.
func buildBenchResult() *DBResult {
	structFields := []reflect.StructField{
		{Name: "Id", Type: reflect.TypeFor[int](), Tag: `json:"id"`},
		{Name: "Title", Type: reflect.TypeFor[string](), Tag: `json:"title"`},
		{Name: "Price", Type: reflect.TypeFor[float64](), Tag: `json:"price"`},
		{Name: "Active", Type: reflect.TypeFor[bool](), Tag: `json:"active"`},
		{
			Name: "Category",
			Type: reflect.StructOf([]reflect.StructField{
				{Name: "Name", Type: reflect.TypeFor[string](), Tag: `json:"name"`},
				{Name: "Id", Type: reflect.TypeFor[int](), Tag: `json:"id"`},
			}),
			Tag: `json:"category"`,
		},
	}

	st := reflect.StructOf(structFields)
	sv := reflect.New(st).Elem()
	sv.FieldByName("Id").Set(reflect.ValueOf(42))
	sv.FieldByName("Title").Set(reflect.ValueOf("Benchmark Product"))
	sv.FieldByName("Price").Set(reflect.ValueOf(99.99))
	sv.FieldByName("Active").Set(reflect.ValueOf(true))
	cat := sv.FieldByName("Category")
	cat.FieldByName("Name").Set(reflect.ValueOf("Electronics"))
	cat.FieldByName("Id").Set(reflect.ValueOf(7))

	return &DBResult{value: sv.Interface(), typ: st}
}

func BenchmarkGetInt(b *testing.B) {
	r := buildBenchResult()
	b.ResetTimer()
	for b.Loop() {
		_ = r.GetInt("id")
	}
}

func BenchmarkGetString(b *testing.B) {
	r := buildBenchResult()
	b.ResetTimer()
	for b.Loop() {
		_ = r.GetString("title")
	}
}

func BenchmarkGetFloat(b *testing.B) {
	r := buildBenchResult()
	b.ResetTimer()
	for b.Loop() {
		_ = r.GetFloat("price")
	}
}

func BenchmarkGetBool(b *testing.B) {
	r := buildBenchResult()
	b.ResetTimer()
	for b.Loop() {
		_ = r.GetBool("active")
	}
}

func BenchmarkGetNestedString(b *testing.B) {
	r := buildBenchResult()
	b.ResetTimer()
	for b.Loop() {
		_ = r.GetString("category.name")
	}
}

func BenchmarkHasField(b *testing.B) {
	r := buildBenchResult()
	b.ResetTimer()
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
	b.ResetTimer()
	for b.Loop() {
		_ = m["id"].(int)
	}
}

func BenchmarkQueryToStruct(b *testing.B) {
	columns := []string{"id", "title", "price", "active", "category.name", "category.id"}
	values := []any{1, "Bench Product", 49.99, true, "Electronics", 3}

	fieldTypes := map[string]reflect.Type{
		"id":     reflect.TypeFor[int](),
		"title":  reflect.TypeFor[string](),
		"price":  reflect.TypeFor[float64](),
		"active": reflect.TypeFor[bool](),
	}
	fieldMap := map[string][]FieldInfo{
		"category": {
			{Name: "name", Type: reflect.TypeFor[string]()},
			{Name: "id", Type: reflect.TypeFor[int]()},
		},
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = createStruct(columns, values, fieldTypes, fieldMap)
	}
}
