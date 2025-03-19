package godyno

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNew(t *testing.T) {
	result := New()
	if result == nil {
		t.Errorf("New() should return a non-nil DBResult")
	}
}

func TestQueryToStruct(t *testing.T) {
	// Create mock DB
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Setup test cases
	testCases := []struct {
		name          string
		query         string
		mockSetup     func(sqlmock.Sqlmock)
		expectedCount int
		expectedError bool
		validation    func([]*DBResult) bool
	}{
		{
			name:  "Basic query with simple fields",
			query: "SELECT id, title, active FROM products",
			mockSetup: func(mock sqlmock.Sqlmock) {
				columns := []string{"id", "title", "active"}
				mock.ExpectQuery("SELECT id, title, active FROM products").
					WillReturnRows(sqlmock.NewRows(columns).
						AddRow(1, "Product 1", true).
						AddRow(2, "Product 2", false))
			},
			expectedCount: 2,
			expectedError: false,
			validation: func(results []*DBResult) bool {
				if results[0].GetInt("id") != 1 || results[0].GetString("title") != "Product 1" || !results[0].GetBool("active") {
					return false
				}
				if results[1].GetInt("id") != 2 || results[1].GetString("title") != "Product 2" || results[1].GetBool("active") {
					return false
				}
				return true
			},
		},
		{
			name:  "Query with nested fields",
			query: "SELECT id, title, category.name FROM products",
			mockSetup: func(mock sqlmock.Sqlmock) {
				columns := []string{"id", "title", "category.name"}
				mock.ExpectQuery("SELECT id, title, category.name FROM products").
					WillReturnRows(sqlmock.NewRows(columns).
						AddRow(1, "Product 1", "Category A"))
			},
			expectedCount: 1,
			expectedError: false,
			validation: func(results []*DBResult) bool {
				return results[0].GetInt("id") == 1 &&
					results[0].GetString("title") == "Product 1" &&
					results[0].GetString("category.name") == "Category A"
			},
		},
		{
			name:  "Query with type conversion",
			query: "SELECT id, price, active FROM products",
			mockSetup: func(mock sqlmock.Sqlmock) {
				columns := []string{"id", "price", "active"}
				mock.ExpectQuery("SELECT id, price, active FROM products").
					WillReturnRows(sqlmock.NewRows(columns).
						AddRow([]byte("1"), []byte("99.99"), []byte("1")))
			},
			expectedCount: 1,
			expectedError: false,
			validation: func(results []*DBResult) bool {
				return results[0].GetInt("id") == 1 &&
					results[0].GetFloat("price") == 99.99 &&
					results[0].GetBool("active")
			},
		},
		{
			name:  "Query with error",
			query: "SELECT * FROM non_existent_table",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("table does not exist"))
			},
			expectedCount: 0,
			expectedError: true,
			validation:    func(results []*DBResult) bool { return true },
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mock expectations
			tc.mockSetup(mock)

			// Execute the function
			results, err := QueryToStruct(db, tc.query)

			// Check error expectations
			if tc.expectedError && err == nil {
				t.Errorf("Expected an error but got none")
			}
			if !tc.expectedError && err != nil {
				t.Errorf("Did not expect an error but got: %v", err)
			}

			// Skip further validation if we expected an error
			if tc.expectedError {
				return
			}

			// Check result count
			if len(results) != tc.expectedCount {
				t.Errorf("Expected %d results, got %d", tc.expectedCount, len(results))
			}

			// Validate results
			if !tc.validation(results) {
				t.Errorf("Result validation failed")
			}
		})
	}

	// Ensure all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestGetMethods(t *testing.T) {
	// Create a sample struct with different field types
	structFields := []reflect.StructField{
		{
			Name: "Id",
			Type: reflect.TypeOf(0),
			Tag:  reflect.StructTag(`json:"id"`),
		},
		{
			Name: "Title",
			Type: reflect.TypeOf(""),
			Tag:  reflect.StructTag(`json:"title"`),
		},
		{
			Name: "Price",
			Type: reflect.TypeOf(0.0),
			Tag:  reflect.StructTag(`json:"price"`),
		},
		{
			Name: "Active",
			Type: reflect.TypeOf(false),
			Tag:  reflect.StructTag(`json:"active"`),
		},
		{
			Name: "Category",
			Type: reflect.StructOf([]reflect.StructField{
				{
					Name: "Name",
					Type: reflect.TypeOf(""),
					Tag:  reflect.StructTag(`json:"name"`),
				},
				{
					Name: "Id",
					Type: reflect.TypeOf(0),
					Tag:  reflect.StructTag(`json:"id"`),
				},
			}),
			Tag: reflect.StructTag(`json:"category"`),
		},
	}

	structType := reflect.StructOf(structFields)
	structVal := reflect.New(structType).Elem()

	// Set values
	structVal.FieldByName("Id").Set(reflect.ValueOf(1))
	structVal.FieldByName("Title").Set(reflect.ValueOf("Test Product"))
	structVal.FieldByName("Price").Set(reflect.ValueOf(19.99))
	structVal.FieldByName("Active").Set(reflect.ValueOf(true))

	categoryVal := structVal.FieldByName("Category")
	categoryVal.FieldByName("Name").Set(reflect.ValueOf("Test Category"))
	categoryVal.FieldByName("Id").Set(reflect.ValueOf(5))

	// Create DBResult
	dbResult := &DBResult{
		value: structVal.Interface(),
		typ:   structType,
	}

	// Test cases
	t.Run("GetString", func(t *testing.T) {
		if got := dbResult.GetString("title"); got != "Test Product" {
			t.Errorf("GetString(title) = %v, want %v", got, "Test Product")
		}
		if got := dbResult.GetString("category.name"); got != "Test Category" {
			t.Errorf("GetString(category.name) = %v, want %v", got, "Test Category")
		}
		if got := dbResult.GetString("non_existent"); got != "" {
			t.Errorf("GetString(non_existent) = %v, want empty string", got)
		}
	})

	t.Run("GetInt", func(t *testing.T) {
		if got := dbResult.GetInt("id"); got != 1 {
			t.Errorf("GetInt(id) = %v, want %v", got, 1)
		}
		if got := dbResult.GetInt("category.id"); got != 5 {
			t.Errorf("GetInt(category.id) = %v, want %v", got, 5)
		}
		if got := dbResult.GetInt("non_existent"); got != 0 {
			t.Errorf("GetInt(non_existent) = %v, want 0", got)
		}
	})

	t.Run("GetFloat", func(t *testing.T) {
		if got := dbResult.GetFloat("price"); got != 19.99 {
			t.Errorf("GetFloat(price) = %v, want %v", got, 19.99)
		}
		if got := dbResult.GetFloat("non_existent"); got != 0 {
			t.Errorf("GetFloat(non_existent) = %v, want 0", got)
		}
	})

	t.Run("GetBool", func(t *testing.T) {
		if got := dbResult.GetBool("active"); !got {
			t.Errorf("GetBool(active) = %v, want %v", got, true)
		}
		if got := dbResult.GetBool("non_existent"); got {
			t.Errorf("GetBool(non_existent) = %v, want %v", got, false)
		}
	})

	t.Run("Get for non-existent field", func(t *testing.T) {
		if got := dbResult.Get("non_existent"); got != nil {
			t.Errorf("Get(non_existent) = %v, want nil", got)
		}
	})
}

func TestCreateStruct(t *testing.T) {
	// Define test columns and values
	columns := []string{"id", "name", "active", "category.name", "category.id"}
	values := []interface{}{1, "Test Product", true, "Test Category", 5}

	// Setup field types
	fieldTypes := map[string]reflect.Type{
		"id":     reflect.TypeOf(0),
		"name":   reflect.TypeOf(""),
		"active": reflect.TypeOf(true),
	}

	// Setup nested fields
	fieldMap := map[string][]FieldInfo{
		"category": {
			{Name: "name", Type: reflect.TypeOf("")},
			{Name: "id", Type: reflect.TypeOf(0)},
		},
	}

	// Create the struct
	result, err := createStruct(columns, values, fieldTypes, fieldMap)
	if err != nil {
		t.Fatalf("createStruct() error = %v", err)
	}

	// Validate the struct
	if result.GetInt("id") != 1 {
		t.Errorf("Expected id=1, got %v", result.GetInt("id"))
	}
	if result.GetString("name") != "Test Product" {
		t.Errorf("Expected name='Test Product', got %v", result.GetString("name"))
	}
	if !result.GetBool("active") {
		t.Errorf("Expected active=true, got %v", result.GetBool("active"))
	}
	if result.GetString("category.name") != "Test Category" {
		t.Errorf("Expected category.name='Test Category', got %v", result.GetString("category.name"))
	}
	if result.GetInt("category.id") != 5 {
		t.Errorf("Expected category.id=5, got %v", result.GetInt("category.id"))
	}
}

// Helper function to create a mock database for integration tests
func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	return db, mock
}
