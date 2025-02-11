package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/ChokeGuy/simple-bank/api/account"
	"github.com/ChokeGuy/simple-bank/api/transfer"
	"github.com/ChokeGuy/simple-bank/api/user"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	cf "github.com/ChokeGuy/simple-bank/pkg/config"
	"github.com/ChokeGuy/simple-bank/pkg/token/paseto"
	sv "github.com/ChokeGuy/simple-bank/server"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// setUpRouter set up all routes
func setUpRouter(server *sv.Server) {
	server.Router.GET("", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, "Welcome to Simple Bank")
	})

	userHandler := user.NewUserHandler(server)
	userHandler.MapRoutes()

	tranferHandler := transfer.NewTransferHandler(server)
	tranferHandler.MapRoutes()

	accountHandler := account.NewAccountHandler(server)
	accountHandler.MapRoutes()

}

func main() {
	cf, err := cf.LoadConfig("./")

	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	conn, err := sql.Open(cf.DBDriver, cf.DBSource)

	if err != nil {
		log.Fatalf("cannot connect to db: %v", err)
	}

	store := db.NewStore(conn)
	tokenMaker, err := paseto.NewPasetoMaker(cf.SymetricKey)
	if err != nil {
		log.Fatalf("Token maker err: %v", err)
	}
	server, err := sv.NewServer(store, &cf, tokenMaker)

	if err != nil {
		log.Fatalf("cannot create server: %v", err)
	}

	setUpRouter(server)

	err = server.Start(cf.ServerAddress)
	if err != nil {
		log.Fatalf("cannot start server: %v", err)
	}
}
