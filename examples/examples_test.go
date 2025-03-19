package main

import (
	"fmt"
	"log"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/mstgnz/godyno"
)

// This example demonstrates the basic usage of the godyno library
// to query a database and work with the results.
func Example_basicUsage() {
	// In a real application, you would use a real database connection
	// but for this example we use a mock
	db, mock, err := sqlmock.New()
	if err != nil {
		log.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Setup test query and mock response
	query := "SELECT id, title, active, price FROM products WHERE category_id = $1"
	columns := []string{"id", "title", "active", "price"}

	mock.ExpectQuery("SELECT id, title, active, price FROM products WHERE category_id = \\$1").
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(1, "Product 1", true, 19.99).
			AddRow(2, "Product 2", false, 29.99).
			AddRow(3, "Product 3", true, 9.99))

	// Execute the query
	results, err := godyno.QueryToStruct(db, query, 5)
	if err != nil {
		log.Fatalf("QueryToStruct failed: %v", err)
	}

	// Example of using the results as shown in the README
	for _, product := range results {
		// Boolean values can be used directly in if conditions
		if product.GetBool("active") {
			// Numeric values
			id := product.GetInt("id")
			price := product.GetFloat("price")

			// String values
			title := product.GetString("title")

			fmt.Printf("Product #%d: %s - %.2f TL\n", id, title, price)
		}
	}

	// Output:
	// Product #1: Product 1 - 19.99 TL
	// Product #3: Product 3 - 9.99 TL
}

// This example demonstrates using nested fields with the godyno library.
func Example_nestedFields() {
	// In a real application, you would use a real database connection
	// but for this example we use a mock
	db, mock, err := sqlmock.New()
	if err != nil {
		log.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Setup test query with nested fields
	query := `SELECT
		p.id,
		p.title,
		p.price,
		c.name AS category.name,
		s.stock AS stock.quantity,
		s.status AS stock.status
	FROM products p
	JOIN categories c ON c.id = p.category_id
	JOIN stock s ON s.product_id = p.id
	WHERE p.id = $1`

	columns := []string{"id", "title", "price", "category.name", "stock.quantity", "stock.status"}

	mock.ExpectQuery("SELECT").
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(42, "Awesome Product", 99.99, "Electronics", 10, "available"))

	// Execute the query
	result, err := godyno.QueryToStruct(db, query, 42)
	if err != nil {
		log.Fatalf("QueryToStruct failed: %v", err)
	}

	if len(result) > 0 {
		product := result[0]

		fmt.Printf("Product: %s\n", product.GetString("title"))
		fmt.Printf("Category: %s\n", product.GetString("category.name"))

		// Using nested fields in conditions
		if product.GetInt("stock.quantity") > 0 && product.GetString("stock.status") == "available" {
			fmt.Println("This product is in stock!")
		}
	}

	// Output:
	// Product: Awesome Product
	// Category: Electronics
	// This product is in stock!
}

// This example demonstrates how to handle different data types with the godyno library.
func Example_differentTypes() {
	// In a real application, you would use a real database connection
	// but for this example we use a mock
	db, mock, err := sqlmock.New()
	if err != nil {
		log.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Setup test query with different data types
	query := "SELECT id, name, price, active, created_at FROM products WHERE id = $1"
	columns := []string{"id", "name", "price", "active", "created_at"}

	// sqlmock expects certain types, so we use interface{} to match
	mock.ExpectQuery("SELECT").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(1, "Test Product", 29.99, true, "2023-01-15"))

	// Execute the query
	results, err := godyno.QueryToStruct(db, query, 1)
	if err != nil {
		log.Fatalf("QueryToStruct failed: %v", err)
	}

	if len(results) > 0 {
		product := results[0]

		// Working with different types
		id := product.GetInt("id")
		name := product.GetString("name")
		price := product.GetFloat("price")
		active := product.GetBool("active")
		created := product.GetString("created_at")

		fmt.Printf("Product ID: %d\n", id)
		fmt.Printf("Name: %s\n", name)
		fmt.Printf("Price: %.2f\n", price)
		fmt.Printf("Active: %t\n", active)
		fmt.Printf("Created At: %s\n", created)

		// Using values in calculations
		discountedPrice := price * 0.9 // 10% discount
		fmt.Printf("Discounted price: %.2f\n", discountedPrice)
	}

	// Output:
	// Product ID: 1
	// Name: Test Product
	// Price: 29.99
	// Active: true
	// Created At: 2023-01-15
	// Discounted price: 26.99
}
