package godyno

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

type DBResult struct {
	value any
	typ   reflect.Type
}

func New() *DBResult {
	return &DBResult{}
}

type FieldInfo struct {
	Name string
	Type reflect.Type
}

// QueryToStruct - converts database query results to dynamic struct
func QueryToStruct(db *sql.DB, query string, args ...any) ([]*DBResult, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("sorgu hatası: %w", err)
	}
	defer rows.Close()

	// Get column names and types
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %w", err)
	}

	// Read first row and determine field types
	var results []*DBResult
	fieldTypes := make(map[string]reflect.Type)
	fieldMap := make(map[string][]FieldInfo)

	// Analyze column names and determine nested structures
	for _, col := range columns {
		parts := strings.Split(col, ".")
		if len(parts) > 1 {
			// Nested field (e.g.: address.city)
			parent := parts[0]
			child := parts[1]

			if _, exists := fieldMap[parent]; !exists {
				fieldMap[parent] = []FieldInfo{}
			}

			// Initially assume string type, can be changed later
			fieldMap[parent] = append(fieldMap[parent], FieldInfo{
				Name: child,
				Type: reflect.TypeOf(""),
			})
		} else {
			// Flat field, initially assumed to be string
			fieldTypes[col] = reflect.TypeOf("")
		}
	}

	// Determine field types for the first row
	if rows.Next() {
		// Slice to hold values
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))

		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("satır taranamadı: %w", err)
		}

		// Update type information
		for i, col := range columns {
			val := values[i]
			parts := strings.Split(col, ".")

			// Determine type
			var valueType reflect.Type
			switch v := val.(type) {
			case []byte:
				// Byte array, possible types: string, int, float, bool
				str := string(v)

				// Is it a number?
				if _, err := strconv.Atoi(str); err == nil {
					valueType = reflect.TypeOf(int(0))
				} else if _, err := strconv.ParseFloat(str, 64); err == nil {
					valueType = reflect.TypeOf(float64(0))
				} else if _, err := strconv.ParseBool(str); err == nil {
					valueType = reflect.TypeOf(bool(false))
				} else {
					valueType = reflect.TypeOf("")
				}
			case nil:
				valueType = reflect.TypeOf("")
			default:
				valueType = reflect.TypeOf(val)
			}

			if len(parts) > 1 {
				// Update type for nested field
				parent := parts[0]
				child := parts[1]

				for i, field := range fieldMap[parent] {
					if field.Name == child {
						fieldMap[parent][i].Type = valueType
						break
					}
				}
			} else {
				// Update type for flat field
				fieldTypes[col] = valueType
			}
		}

		// Create struct for the first row
		result, err := createStruct(columns, values, fieldTypes, fieldMap)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	// Process remaining rows
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))

		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("satır taranamadı: %w", err)
		}

		result, err := createStruct(columns, values, fieldTypes, fieldMap)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("satır işleme hatası: %w", err)
	}

	return results, nil
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
				Name: strings.Title(field.Name), // İlk harf büyük
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
			Name: strings.Title(col), // First letter uppercase
			Type: typ,
			Tag:  reflect.StructTag(fmt.Sprintf(`json:"%s"`, col)),
		})
	}

	// Add nested structs to the parent struct
	for parent, typ := range parentStructs {
		structFields = append(structFields, reflect.StructField{
			Name: strings.Title(parent), // First letter uppercase
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

			// Get parent object
			parentField := structValue.FieldByName(strings.Title(parent))

			// Select nested field and assign value
			childField := parentField.FieldByName(strings.Title(child))
			if childField.IsValid() && childField.CanSet() {
				childField.Set(reflect.ValueOf(val))
			}
		} else {
			// Assign value to parent field
			field := structValue.FieldByName(strings.Title(col))
			if field.IsValid() && field.CanSet() {
				field.Set(reflect.ValueOf(val))
			}
		}
	}

	return &DBResult{
		value: structValue.Interface(),
		typ:   structType,
	}, nil
}

// Get - returns the value of a field in the struct
func (dr *DBResult) Get(fieldName string) any {
	parts := strings.Split(fieldName, ".")
	val := reflect.ValueOf(dr.value)

	// If there is a nested field, proceed
	for _, part := range parts {
		fieldName := strings.Title(part) // First letter uppercase
		field := val.FieldByName(fieldName)

		if !field.IsValid() {
			return nil
		}

		val = field
	}

	return val.Interface()
}

// GetString - returns the value as a string
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

// GetInt - returns the value as an int
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

// GetFloat - returns the value as a float64
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

// GetBool - returns the value as a bool
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
