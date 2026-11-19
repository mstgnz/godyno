package godyno_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/mstgnz/godyno"
)

// The examples use sqlmock so they run without a database; with a real one,
// pass the *sql.DB from sql.Open instead.

func Example() {
	db, mock, err := sqlmock.New()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT").WithArgs(5).WillReturnRows(sqlmock.NewRows([]string{"id", "title", "active", "price"}).
		AddRow(int64(1), "Product 1", true, 19.99).
		AddRow(int64(2), "Product 2", false, 29.99).
		AddRow(int64(3), "Product 3", true, 9.99))

	products, err := godyno.QueryToStruct(db, "SELECT id, title, active, price FROM products WHERE category_id = $1", 5)
	if err != nil {
		log.Fatal(err)
	}
	for _, p := range products {
		if p.GetBool("active") {
			fmt.Printf("#%d %s %.2f\n", p.GetInt("id"), p.GetString("title"), p.GetFloat("price"))
		}
	}
	// Output:
	// #1 Product 1 19.99
	// #3 Product 3 9.99
}

func Example_nestedFields() {
	db, mock, err := sqlmock.New()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"title", "category.name", "stock.quantity"}).
		AddRow("Go Book", "Books", int64(10)))

	// Postgres needs double quotes around a dotted alias, MySQL needs backticks.
	rows, err := godyno.QueryToStruct(db, `SELECT p.title, c.name AS "category.name", s.qty AS "stock.quantity"
		FROM products p JOIN categories c ON c.id = p.category_id JOIN stock s ON s.product_id = p.id`)
	if err != nil {
		log.Fatal(err)
	}
	p := rows[0]
	fmt.Println(p.GetString("category.name"), p.GetInt("stock.quantity"))
	fmt.Println(p.Get("category"))
	// Output:
	// Books 10
	// map[name:Books]
}

func ExampleDBResult_MarshalJSON() {
	db, mock, err := sqlmock.New()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id", "title", "category.name", "deleted_at"}).
		AddRow(int64(1), "Go Book", "Books", nil))

	rows, err := godyno.QueryToStruct(db, "SELECT ...")
	if err != nil {
		log.Fatal(err)
	}
	out, err := json.Marshal(rows)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))
	// Output:
	// [{"category":{"name":"Books"},"deleted_at":null,"id":1,"title":"Go Book"}]
}

func ExampleDBResult_GetIntOk() {
	db, mock, err := sqlmock.New()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"name", "score"}).
		AddRow("Ada", int64(0)).
		AddRow("Bob", nil))

	rows, err := godyno.QueryToStruct(db, "SELECT name, score FROM players")
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range rows {
		if score, ok := r.GetIntOk("score"); ok {
			fmt.Println(r.GetString("name"), "scored", score)
		} else if r.IsNull("score") {
			fmt.Println(r.GetString("name"), "has no score yet")
		}
	}
	// Output:
	// Ada scored 0
	// Bob has no score yet
}

func ExampleQueryIter() {
	db, mock, err := sqlmock.New()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).
		AddRow(int64(1)).AddRow(int64(2)).AddRow(int64(3)))

	// Rows are read one at a time; leaving the loop early closes them.
	for r, err := range godyno.QueryIter(context.Background(), db, "SELECT id FROM events") {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(r.GetInt("id"))
	}
	// Output:
	// 1
	// 2
	// 3
}
