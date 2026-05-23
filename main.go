package main

import (

	//"db-seeder/handlers"
	"context"
	"db-seeder/db"
	"db-seeder/display"
	"db-seeder/walker"
	"fmt"
	"log"
	//"log"
)

func main() {
	fmt.Println("Hello, World!")
	//db.CreateDatabase()
	conn, err := db.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(context.Background())

	db.RunSchema(conn)

	//err = handlers.InsertBatch(conn, 200, 200) // batchSize=200, total=1000
	//if err != nil {
	//	log.Fatal("Seeding failed:", err)
	//	}

	if err := walker.Init(); err != nil {
		fmt.Printf("walker unavailable: %v\n", err)
	}

	display.StartInterface(conn)
}
