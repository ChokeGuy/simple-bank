package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"net/http"

	"github.com/ChokeGuy/simple-bank/api/account"
	"github.com/ChokeGuy/simple-bank/api/transfer"
	"github.com/ChokeGuy/simple-bank/api/user"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	_ "github.com/ChokeGuy/simple-bank/doc/statik"
	grpcapi "github.com/ChokeGuy/simple-bank/grpc-api"
	"github.com/ChokeGuy/simple-bank/pb"
	cf "github.com/ChokeGuy/simple-bank/pkg/config"
	"github.com/ChokeGuy/simple-bank/pkg/token"
	"github.com/ChokeGuy/simple-bank/pkg/token/paseto"
	grpcSv "github.com/ChokeGuy/simple-bank/server/grpc"
	httpSv "github.com/ChokeGuy/simple-bank/server/http"
	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	_ "github.com/lib/pq"
	"github.com/rakyll/statik/fs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

// setUpRouter set up all routes
func setUpRouter(server *httpSv.Server) {
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

	go runGatewayServer(cf, store, tokenMaker)
	runGrpcServer(cf, store, tokenMaker)
}

func runHttpServer(cfg cf.Config, store db.Store, tokenMaker token.Maker) {
	server, err := httpSv.NewServer(store, &cfg, tokenMaker)

	if err != nil {
		log.Fatalf("cannot create server: %v", err)
	}

	setUpRouter(server)

	err = server.Start(cfg.HttpServerAddress)
	if err != nil {
		log.Fatalf("cannot start server: %v", err)
	}
}

func runGrpcServer(cfg cf.Config, store db.Store, tokenMaker token.Maker) {
	server, err := grpcSv.NewServer(store, &cfg, tokenMaker)
	if err != nil {
		log.Fatalf("cannot create server: %v", err)
	}

	serviceHandler := grpcapi.NewServiceHandler(server)

	grpcServer := grpc.NewServer()
	pb.RegisterSimpleBankServer(grpcServer, serviceHandler)
	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", cfg.GrpcServerAddress)
	if err != nil {
		log.Fatalf("cannot listen to grpc server: %v", err)
	}

	log.Printf("start grpc server on %s", cfg.GrpcServerAddress)
	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatalf("cannot start grpc server: %v", err)
	}
}

func runGatewayServer(cfg cf.Config, store db.Store, tokenMaker token.Maker) {
	server, err := grpcSv.NewServer(store, &cfg, tokenMaker)
	if err != nil {
		log.Fatalf("cannot create server: %v", err)
	}

	serviceHandler := grpcapi.NewServiceHandler(server)

	jsonOption := runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			UseProtoNames: true,
		},
		UnmarshalOptions: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	})

	grpcMux := runtime.NewServeMux(jsonOption)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = pb.RegisterSimpleBankHandlerServer(ctx, grpcMux, serviceHandler)
	if err != nil {
		log.Fatalf("cannot register grpc handler: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", grpcMux)

	statikFS, err := fs.New()
	if err != nil {
		log.Fatalf("cannot create statik file system: %v", err)
	}

	swaggerHandler := http.StripPrefix("/swagger/", http.FileServer(statikFS))
	mux.Handle("/swagger/", swaggerHandler)

	listener, err := net.Listen("tcp", cfg.HttpServerAddress)
	if err != nil {
		log.Fatalf("cannot listen to grpc server: %v", err)
	}

	log.Printf("start http gateway server on %s", cfg.HttpServerAddress)
	err = http.Serve(listener, mux)
	if err != nil {
		log.Fatalf("cannot start HTTP Gatewat Server: %v", err)
	}
}
