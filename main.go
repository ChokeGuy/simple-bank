package main

import (
	"database/sql"
	"log"

	"github.com/ChokeGuy/simple-bank/api"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	_ "github.com/lib/pq"
)

const (
	dbDriver      = "postgres"
	dbSource      = "postgresql://root:123456@localhost:5432/simple-bank?sslmode=disable"
	serverAddress = "0.0.0.0:8080"
)

func main() {
	conn, err := sql.Open(dbDriver, dbSource)

	if err != nil {
		log.Fatalf("cannot connect to db: %v", err)
	}

	store := db.NewStore(conn)
	server := api.NewServer(store)

	err = server.Start(serverAddress)

	if err != nil {
		log.Fatalf("cannot start server: %v", err)
	}
}
