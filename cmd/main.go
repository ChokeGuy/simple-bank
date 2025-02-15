package main

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"os"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ChokeGuy/simple-bank/api/account"
	"github.com/ChokeGuy/simple-bank/api/transfer"
	"github.com/ChokeGuy/simple-bank/api/user"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	_ "github.com/ChokeGuy/simple-bank/doc/statik"
	grpcapi "github.com/ChokeGuy/simple-bank/grpc-api"
	"github.com/ChokeGuy/simple-bank/pb"
	cf "github.com/ChokeGuy/simple-bank/pkg/config"
	"github.com/ChokeGuy/simple-bank/pkg/logger"
	"github.com/ChokeGuy/simple-bank/pkg/token"
	"github.com/ChokeGuy/simple-bank/pkg/token/paseto"
	grpcSv "github.com/ChokeGuy/simple-bank/server/grpc"
	httpSv "github.com/ChokeGuy/simple-bank/server/http"
	"github.com/ChokeGuy/simple-bank/worker"
	"github.com/gin-gonic/gin"
	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	_ "github.com/lib/pq"
	"github.com/rakyll/statik/fs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	cf, err := cf.LoadConfig("./")

	if err != nil {
		log.Fatal().Msgf("cannot load config: %v", err)
	}

	if cf.ENV == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	conn, err := sql.Open(cf.DBDriver, cf.DBSource)

	if err != nil {
		log.Fatal().Msgf("cannot connect to db: %v", err)
	}

	runDBMigration(cf)

	store := db.NewStore(conn)
	tokenMaker, err := paseto.NewPasetoMaker(cf.SymetricKey)
	if err != nil {
		log.Fatal().Msgf("Token maker err: %v", err)
	}

	redisOpt := asynq.RedisClientOpt{
		Addr: cf.RedisAddress,
	}

	taskDistributor := worker.NewRedisTaskDistributor(redisOpt)

	go worker.RunTaskProcessor(redisOpt, store)
	go runHttpServer(cf, store, tokenMaker, taskDistributor)
	runGrpcServer(cf, store, tokenMaker, taskDistributor)
}

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

// runHttpServer run http server
func runHttpServer(cfg cf.Config, store db.Store, tokenMaker token.Maker, taskDistributor worker.TaskDistributor) {
	server, err := httpSv.NewServer(store, &cfg, tokenMaker, taskDistributor)

	if err != nil {
		log.Fatal().Msgf("cannot create server: %v", err)
	}

	setUpRouter(server)

	err = server.Start(cfg.HttpServerAddress)
	if err != nil {
		log.Fatal().Msgf("cannot start server: %v", err)
	}
}

// runGrpcServer run grpc server
func runGrpcServer(cfg cf.Config, store db.Store, tokenMaker token.Maker, taskDistributor worker.TaskDistributor) {
	server, err := grpcSv.NewServer(store, &cfg, tokenMaker, taskDistributor)
	if err != nil {
		log.Fatal().Msgf("cannot create server: %v", err)
	}

	serviceHandler := grpcapi.NewServiceHandler(server)

	grpcLogger := grpc.UnaryInterceptor(logger.GrpcLogger)
	grpcServer := grpc.NewServer(grpcLogger)
	pb.RegisterSimpleBankServer(grpcServer, serviceHandler)
	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", cfg.GrpcServerAddress)
	if err != nil {
		log.Fatal().Msgf("cannot listen to grpc server: %v", err)
	}

	log.Info().Msgf("start grpc server on %s", cfg.GrpcServerAddress)
	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatal().Msgf("cannot start grpc server: %v", err)
	}
}

// runGatewayServer run grpc-gateway server
func runGatewayServer(cfg cf.Config, store db.Store, tokenMaker token.Maker, taskDistributor worker.TaskDistributor) {
	server, err := grpcSv.NewServer(store, &cfg, tokenMaker, taskDistributor)
	if err != nil {
		log.Fatal().Msgf("cannot create server: %v", err)
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
		log.Fatal().Msgf("cannot register grpc handler: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", grpcMux)

	statikFS, err := fs.New()
	if err != nil {
		log.Fatal().Msgf("cannot create statik file system: %v", err)
	}

	swaggerHandler := http.StripPrefix("/swagger/", http.FileServer(statikFS))
	mux.Handle("/swagger/", swaggerHandler)

	listener, err := net.Listen("tcp", cfg.HttpServerAddress)
	if err != nil {
		log.Fatal().Msgf("cannot listen to grpc server: %v", err)
	}

	log.Info().Msgf("start http gateway server on %s", cfg.HttpServerAddress)
	handler := logger.HttpLogger(mux)

	err = http.Serve(listener, handler)
	if err != nil {
		log.Fatal().Msgf("cannot start HTTP Gateway Server: %v", err)
	}
}

func runDBMigration(cfg cf.Config) {
	migration, err := migrate.New(cfg.MigrationUrl, cfg.DBSource)
	if err != nil {
		log.Fatal().Msgf("cannot create migration: %v", err)
	}

	if err = migration.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal().Msgf("failed to run migrate up: %v", err)
	}

	log.Info().Msg("db migration successfully")
}
