package main

import (

	//"db-seeder/handlers"
	"context"
	"db-seeder/db"
	"db-seeder/display"
	"db-seeder/simulation"
	"db-seeder/tools"
	"db-seeder/walker"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	//"log"
)

func main() {
	fmt.Println("Hello, World!")
	db.CreateDatabase()
	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	db.RunSchema(conn)

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("could not get working directory: %v", err)
	}

	//err = handlers.InsertBatch(conn, 200, 200) // batchSize=200, total=1000
	//if err != nil {
	//	log.Fatal("Seeding failed:", err)
	//	}

	tools := []tools.Tool{}

	walker_tool, err := walker.NewTool(cwd)
	if err != nil {
		log.Printf("walker unavailable: %v", err)
	} else {
		tools = append(tools, walker_tool)
	}

	sim_tool, err := simulation.NewTool(cwd)
	if err != nil {
		log.Printf("simulation unavailable: %v", err)
	} else {
		tools = append(tools, sim_tool)
	}

	display.StartInterface(conn, tools)
}
