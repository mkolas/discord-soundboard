package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

func main() {

	os.Remove("config/sounds.db")

	db, err := sql.Open("sqlite3", "config/sounds.db")
	if err != nil {
		log.Fatal("Unable to create database")
	}
	defer db.Close()

	createStmt := `
	create table if not exists sounds(command text, file text, played int);
	`
	_, err = db.Exec(createStmt)
	if err != nil {
		log.Fatalln("%q, %s\n", err, createStmt)
	}
	// lets load up our sounds
	soundsFile, err := os.OpenFile("config/sounds.csv", os.O_RDWR|os.O_CREATE, os.ModePerm) // should figure out what these os objects are
	if err != nil {
		log.Fatalln(err)
	}
	defer soundsFile.Close()

	reader := csv.NewReader(soundsFile)
	//Configure reader options Ref http://golang.org/src/pkg/encoding/csv/reader.go?s=#L81
	reader.Comma = ','
	reader.Comment = '#'
	reader.FieldsPerRecord = 2
	reader.TrimLeadingSpace = true

	for {
		record, err := reader.Read()
		// end-of-file is fitted into err
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Println("Error:", err)
			reader.Read()
			continue
		}

		insertStmt, err := db.Prepare("insert into sounds(command, file, played) values(?,?,?)")
		if err != nil {
			log.Fatal(err)
		}

		insertStmt.Exec(record[1], record[0], 0)
	}
}
