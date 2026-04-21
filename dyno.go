// Package godyno converts database query results into dynamic structs with
// automatic type detection, providing type-safe access without predefined structs.
package godyno

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// DBResult holds a single row from a database query as a dynamic struct.
// Field types are inferred from the first row of the result set.
// Getter methods return safe zero values when a field does not exist or cannot be converted.
type DBResult struct {
	value any
	typ   reflect.Type
}

// New returns an empty DBResult. Prefer QueryToStruct for normal usage.
func New() *DBResult {
	return &DBResult{}
}

// FieldInfo describes a single field in a nested struct built from a dot-notation column name.
type FieldInfo struct {
	Name string
	Type reflect.Type
}

// toTitle capitalizes the first letter of a string
func toTitle(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// QueryToStruct executes query with optional args and converts each result row
// into a DBResult. Column types are inferred from the first non-nil value found
// in each column across all rows, so NULL-only columns fall back to string.
func QueryToStruct(db *sql.DB, query string, args ...any) ([]*DBResult, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %w", err)
	}

	// Pass 1: read every row into memory so we can scan all values before
	// locking column types.
	var allValues [][]any
	for rows.Next() {
		vals := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		allValues = append(allValues, vals)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	if len(allValues) == 0 {
		return nil, nil
	}

	// Pass 2: determine the type for each column using the first non-nil value
	// found anywhere in that column.
	fieldTypes := make(map[string]reflect.Type)
	fieldMap := make(map[string][]FieldInfo)

	for _, col := range columns {
		parts := strings.Split(col, ".")
		if len(parts) > 1 {
			parent, child := parts[0], parts[1]
			if _, exists := fieldMap[parent]; !exists {
				fieldMap[parent] = []FieldInfo{}
			}
			fieldMap[parent] = append(fieldMap[parent], FieldInfo{
				Name: child,
				Type: reflect.TypeFor[string](), // default, overwritten below
			})
		} else {
			fieldTypes[col] = reflect.TypeFor[string]() // default
		}
	}

	for _, vals := range allValues {
		for i, col := range columns {
			val := vals[i]
			if val == nil {
				continue // keep looking for a non-nil value
			}

			parts := strings.Split(col, ".")
			valueType := inferType(val)

			if len(parts) > 1 {
				parent, child := parts[0], parts[1]
				for j, field := range fieldMap[parent] {
					if field.Name == child && field.Type == reflect.TypeFor[string]() {
						fieldMap[parent][j].Type = valueType
					}
				}
			} else {
				if fieldTypes[col] == reflect.TypeFor[string]() {
					fieldTypes[col] = valueType
				}
			}
		}
	}

	// Pass 3: build a DBResult for every row using the now-stable types.
	results := make([]*DBResult, 0, len(allValues))
	for _, vals := range allValues {
		result, err := createStruct(columns, vals, fieldTypes, fieldMap)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

// inferType returns the reflect.Type for a scanned database value.
// []byte columns are parsed to detect int, float64, or bool before falling back to string.
func inferType(val any) reflect.Type {
	switch v := val.(type) {
	case []byte:
		str := string(v)
		if _, err := strconv.Atoi(str); err == nil {
			return reflect.TypeFor[int]()
		}
		if _, err := strconv.ParseFloat(str, 64); err == nil {
			return reflect.TypeFor[float64]()
		}
		if _, err := strconv.ParseBool(str); err == nil {
			return reflect.TypeFor[bool]()
		}
		return reflect.TypeFor[string]()
	case nil:
		return reflect.TypeFor[string]()
	default:
		return reflect.TypeOf(v)
	}
}

// createStruct - creates a dynamic struct with field types and values
func createStruct(columns []string, values []any, fieldTypes map[string]reflect.Type, fieldMap map[string][]FieldInfo) (*DBResult, error) {
	// Create struct types for the parent fields
	structFields := []reflect.StructField{}
	parentStructs := make(map[string]reflect.Type)

	// First create struct types for nested fields
	for parent, fields := range fieldMap {
		nestedFields := []reflect.StructField{}

		for _, field := range fields {
			nestedFields = append(nestedFields, reflect.StructField{
				Name: toTitle(field.Name),
				Type: field.Type,
				Tag:  reflect.StructTag(fmt.Sprintf(`json:"%s"`, field.Name)),
			})
		}

		// Create struct type for the nested fields
		parentStructs[parent] = reflect.StructOf(nestedFields)
	}

	// Prepare all fields for the parent struct
	for col, typ := range fieldTypes {
		structFields = append(structFields, reflect.StructField{
			Name: toTitle(col), // First letter uppercase
			Type: typ,
			Tag:  reflect.StructTag(fmt.Sprintf(`json:"%s"`, col)),
		})
	}

	// Add nested structs to the parent struct
	for parent, typ := range parentStructs {
		structFields = append(structFields, reflect.StructField{
			Name: toTitle(parent), // First letter uppercase
			Type: typ,
			Tag:  reflect.StructTag(fmt.Sprintf(`json:"%s"`, parent)),
		})
	}

	// Create the parent struct
	structType := reflect.StructOf(structFields)
	structValue := reflect.New(structType).Elem()

	// Place values in the struct
	for i, col := range columns {
		val := values[i]
		parts := strings.Split(col, ".")

		// Convert byte array to the correct type
		if byteArray, ok := val.([]byte); ok {
			str := string(byteArray)
			fieldType := fieldTypes[col]

			if len(parts) > 1 {
				// Check type for nested field
				parent := parts[0]
				child := parts[1]

				for _, field := range fieldMap[parent] {
					if field.Name == child {
						fieldType = field.Type
						break
					}
				}
			}

			switch fieldType.Kind() {
			case reflect.Int:
				if num, err := strconv.Atoi(str); err == nil {
					val = num
				}
			case reflect.Float64:
				if num, err := strconv.ParseFloat(str, 64); err == nil {
					val = num
				}
			case reflect.Bool:
				if b, err := strconv.ParseBool(str); err == nil {
					val = b
				}
			default:
				val = str
			}
		}

		if len(parts) > 1 {
			// Assign value to nested field
			parent := parts[0]
			child := parts[1]

			parentField := structValue.FieldByName(toTitle(parent))
			childField := parentField.FieldByName(toTitle(child))
			if childField.IsValid() && childField.CanSet() && val != nil {
				childField.Set(reflect.ValueOf(val))
			}
		} else {
			field := structValue.FieldByName(toTitle(col))
			if field.IsValid() && field.CanSet() && val != nil {
				field.Set(reflect.ValueOf(val))
			}
		}
	}

	return &DBResult{
		value: structValue.Interface(),
		typ:   structType,
	}, nil
}

// HasField reports whether the given field name (dot notation supported) exists
// in the result. Use this to distinguish a missing field from a legitimate zero value.
func (dr *DBResult) HasField(fieldName string) bool {
	parts := strings.Split(fieldName, ".")
	val := reflect.ValueOf(dr.value)

	for _, part := range parts {
		field := val.FieldByName(toTitle(part))
		if !field.IsValid() {
			return false
		}
		val = field
	}
	return true
}

// Get returns the raw value of fieldName (dot notation supported), or nil if the
// field does not exist. Use HasField to distinguish nil-from-missing vs a stored nil.
func (dr *DBResult) Get(fieldName string) any {
	parts := strings.Split(fieldName, ".")
	val := reflect.ValueOf(dr.value)

	for _, part := range parts {
		field := val.FieldByName(toTitle(part))
		if !field.IsValid() {
			return nil
		}
		val = field
	}

	return val.Interface()
}

// GetString returns the field value as a string. Returns "" if the field does not exist.
func (dr *DBResult) GetString(fieldName string) string {
	val := dr.Get(fieldName)
	if val == nil {
		return ""
	}

	if str, ok := val.(string); ok {
		return str
	}

	return fmt.Sprintf("%v", val)
}

// GetInt returns the field value as an int. Returns 0 if the field does not exist or cannot be converted.
func (dr *DBResult) GetInt(fieldName string) int {
	val := dr.Get(fieldName)
	if val == nil {
		return 0
	}

	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}

	return 0
}

// GetFloat returns the field value as a float64. Returns 0 if the field does not exist or cannot be converted.
func (dr *DBResult) GetFloat(fieldName string) float64 {
	val := dr.Get(fieldName)
	if val == nil {
		return 0
	}

	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}

	return 0
}

// GetBool returns the field value as a bool. Returns false if the field does not exist or cannot be converted.
func (dr *DBResult) GetBool(fieldName string) bool {
	val := dr.Get(fieldName)
	if val == nil {
		return false
	}

	switch v := val.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case string:
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}

	return false
}

// GetStringOk returns the field value as a string and whether the field existed.
// Unlike GetString, this lets callers distinguish a missing field from an empty string.
func (dr *DBResult) GetStringOk(fieldName string) (string, bool) {
	if !dr.HasField(fieldName) {
		return "", false
	}
	return dr.GetString(fieldName), true
}

// GetIntOk returns the field value as an int and whether the conversion succeeded.
// Returns (0, false) if the field does not exist or cannot be converted to int.
func (dr *DBResult) GetIntOk(fieldName string) (int, bool) {
	val := dr.Get(fieldName)
	if val == nil {
		return 0, false
	}

	switch v := val.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	}

	return 0, false
}

// GetFloatOk returns the field value as a float64 and whether the conversion succeeded.
// Returns (0, false) if the field does not exist or cannot be converted to float64.
func (dr *DBResult) GetFloatOk(fieldName string) (float64, bool) {
	val := dr.Get(fieldName)
	if val == nil {
		return 0, false
	}

	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}

	return 0, false
}

// GetBoolOk returns the field value as a bool and whether the conversion succeeded.
// Returns (false, false) if the field does not exist or cannot be converted to bool.
func (dr *DBResult) GetBoolOk(fieldName string) (bool, bool) {
	val := dr.Get(fieldName)
	if val == nil {
		return false, false
	}

	switch v := val.(type) {
	case bool:
		return v, true
	case int:
		return v != 0, true
	case string:
		if b, err := strconv.ParseBool(v); err == nil {
			return b, true
		}
	}

	return false, false
}
