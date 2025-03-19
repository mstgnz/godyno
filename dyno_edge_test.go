package godyno

import (
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// Custom driver.Value implementation for testing custom types
type customValue struct{}

func (c customValue) Value() (driver.Value, error) {
	return "custom value", nil
}

func TestEdgeCases(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	t.Run("Empty result set", func(t *testing.T) {
		query := "SELECT * FROM products WHERE 1=0"
		columns := []string{"id", "name"}

		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(columns))

		results, err := QueryToStruct(db, query)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Expected empty result set, got %d results", len(results))
		}
	})

	t.Run("SQL error", func(t *testing.T) {
		query := "SELECT * FROM invalid_table"

		// Return an error directly from the query
		mock.ExpectQuery("SELECT").WillReturnError(errors.New("table does not exist"))

		_, err := QueryToStruct(db, query)
		if err == nil {
			t.Error("Expected an error but got none")
		}
	})

	t.Run("Custom type value", func(t *testing.T) {
		query := "SELECT id, custom FROM products"
		columns := []string{"id", "custom"}

		// Use a custom type
		customVal := customValue{}

		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows(columns).AddRow(1, customVal),
		)

		results, err := QueryToStruct(db, query)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}

		if val := results[0].GetString("custom"); val != "custom value" {
			t.Errorf("Expected 'custom value', got '%s'", val)
		}
	})

	t.Run("Type conversion edge cases", func(t *testing.T) {
		query := "SELECT string_as_int, string_as_float, string_as_bool, invalid_number FROM products"
		columns := []string{"string_as_int", "string_as_float", "string_as_bool", "invalid_number"}

		mock.ExpectQuery("SELECT").WillReturnRows(
			sqlmock.NewRows(columns).AddRow(
				[]byte("123"),          // string that should convert to int
				[]byte("123.45"),       // string that should convert to float
				[]byte("true"),         // string that should convert to bool
				[]byte("not_a_number"), // string that can't convert to number
			),
		)

		results, err := QueryToStruct(db, query)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}

		// Test type conversions
		if val := results[0].GetInt("string_as_int"); val != 123 {
			t.Errorf("Expected int 123, got %d", val)
		}

		if val := results[0].GetFloat("string_as_float"); val != 123.45 {
			t.Errorf("Expected float 123.45, got %f", val)
		}

		if val := results[0].GetBool("string_as_bool"); !val {
			t.Errorf("Expected bool true, got %v", val)
		}

		// Invalid conversion should return default value
		if val := results[0].GetInt("invalid_number"); val != 0 {
			t.Errorf("Expected 0 for invalid number conversion, got %d", val)
		}
	})
}

func TestFailedDBConnection(t *testing.T) {
	// Create a mock DB that will be closed before use
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	db.Close() // Close immediately to simulate connection failure

	_, err = QueryToStruct(db, "SELECT * FROM products")
	if err == nil {
		t.Error("Expected an error with closed DB connection but got none")
	}
}
