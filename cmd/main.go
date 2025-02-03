package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/ChokeGuy/simple-bank/api/account"
	"github.com/ChokeGuy/simple-bank/api/transfer"
	"github.com/ChokeGuy/simple-bank/api/user"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	cf "github.com/ChokeGuy/simple-bank/pkg/config"
	"github.com/ChokeGuy/simple-bank/pkg/token/paseto"
	"github.com/ChokeGuy/simple-bank/server"
	_ "github.com/lib/pq"
)

func main() {
	cf, err := cf.LoadConfig("./")

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
	tokenMaker, err := paseto.NewPasetoMaker(cf.SymetricKey)
	if err != nil {
		log.Fatalf("Token maker err: %v", err)
	}
	server, err := server.NewServer(store, &cf, tokenMaker)

	if err != nil {
		log.Fatalf("cannot create server: %v", err)
	}

	//Routes
	accountHandler := account.NewAccountHandler(server)
	accountHandler.MapRoutes()

	tranferHandler := transfer.NewTransferHandler(server)
	tranferHandler.MapRoutes()

	userHandler := user.NewUserHandler(server)
	userHandler.MapRoutes()

	err = server.Start(cf.ServerAddress)
	if err != nil {
		log.Fatalf("cannot start server: %v", err)
	}
}
