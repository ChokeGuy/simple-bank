package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/ChokeGuy/simple-bank/api"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	cf "github.com/ChokeGuy/simple-bank/pkg/config"
	_ "github.com/lib/pq"
)

func main() {
	cf, err := cf.LoadConfig(".")

	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	fmt.Println(cf.DBDriver)
	fmt.Println(cf.DBSource)

	conn, err := sql.Open(cf.DBDriver, cf.DBSource)

	if err != nil {
		log.Fatalf("cannot connect to db: %v", err)
	}

	store := db.NewStore(conn)
	server := api.NewServer(store)

	err = server.Start(cf.ServerAddress)
	if err != nil {
		log.Fatalf("cannot start server: %v", err)
	}
}
