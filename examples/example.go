package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/mstgnz/godyno"
)

func main() {
	db, err := sql.Open("postgres", "host=localhost user=user password=pass dbname=mydb sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	query := `SELECT 
		p.id, 
		pt.title, 
		fp.active,
		f.year AS facility.year,
		f.name AS facility.name,
		a.city AS address.city
	FROM facility_pitches fp 
	JOIN pitches p ON p.id = fp.pitch_id 
	JOIN pitch_translations pt ON pt.pitch_id = p.id 
	JOIN facilities f ON f.id = fp.facility_id
	JOIN addresses a ON a.facility_id = f.id
	WHERE fp.facility_id = $1`

	results, err := godyno.QueryToStruct(db, query, 1)
	if err != nil {
		log.Fatal(err)
	}

	for _, result := range results {
		if result.GetBool("active") {
			fmt.Println("Bu pitch aktif!")
		}

		id := result.GetInt("id")
		year := result.GetInt("facility.year")

		if year > 2020 {
			fmt.Printf("Yeni tesis (%d), Pitch ID: %d\n", year, id)
		}

		title := result.GetString("title")
		city := result.GetString("address.city")

		fmt.Printf("Pitch: %s, Şehir: %s\n", title, city)
	}
}
