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

// TestNullFirstRow verifies that a NULL in the first row does not prevent correct
// type inference: the engine now scans all rows before locking types, so the
// non-null value in row 2 correctly determines the column type as int.
func TestNullFirstRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	columns := []string{"id", "score"}

	mock.ExpectQuery("SELECT").WillReturnRows(
		sqlmock.NewRows(columns).
			AddRow(1, nil).          // score is NULL in row 1
			AddRow(2, []byte("42")), // score is 42 in row 2 — determines type as int
	)

	results, err := QueryToStruct(db, "SELECT id, score FROM products")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Row 1: score was NULL — stored as zero value of the inferred int type
	if got := results[0].GetInt("score"); got != 0 {
		t.Errorf("Row 1 score: got %d, want 0", got)
	}

	// Row 2: score is correctly stored as int 42
	if got := results[1].GetInt("score"); got != 42 {
		t.Errorf("Row 2 score: got %d, want 42", got)
	}
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
