package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rakyll/statik/fs"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/ChokeGuy/simple-bank/api/account"
	"github.com/ChokeGuy/simple-bank/api/transfer"
	"github.com/ChokeGuy/simple-bank/api/user"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	_ "github.com/ChokeGuy/simple-bank/doc/statik"
	grpcapi "github.com/ChokeGuy/simple-bank/grpc-api"
	"github.com/ChokeGuy/simple-bank/pb"
	cf "github.com/ChokeGuy/simple-bank/pkg/config"
	dbmigrations "github.com/ChokeGuy/simple-bank/pkg/db-migrations"
	"github.com/ChokeGuy/simple-bank/pkg/logger"
	"github.com/ChokeGuy/simple-bank/pkg/token"
	"github.com/ChokeGuy/simple-bank/pkg/token/paseto"
	grpcSv "github.com/ChokeGuy/simple-bank/server/grpc"
	httpSv "github.com/ChokeGuy/simple-bank/server/http"
	"github.com/ChokeGuy/simple-bank/worker"
	"github.com/gin-gonic/gin"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

var interruptSignal = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
	syscall.SIGQUIT,
}

func main() {
	cf, err := cf.LoadConfig("./")
	if err != nil {
		log.Fatal().Msgf("cannot load config: %v", err)
	}

	if cf.ENV == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	ctx, stop := signal.NotifyContext(context.Background(), interruptSignal...)
	defer stop()

	conn, err := pgxpool.New(ctx, cf.DBSource)
	if err != nil {
		log.Fatal().Msgf("cannot connect to db: %v", err)
	}

	dbmigrations.RunDBMigration(cf)
	store := db.NewStore(conn)
	tokenMaker, err := paseto.NewPasetoMaker(cf.SymetricKey)
	if err != nil {
		log.Fatal().Msgf("Token maker err: %v", err)
	}

	redisOpt := asynq.RedisClientOpt{
		Addr: cf.RedisAddress,
		TLSConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		},
		DialTimeout:  10 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		PoolSize:     10,
	}

	taskDistributor := worker.NewRedisTaskDistributor(redisOpt)

	waitGroup, ctx := errgroup.WithContext(ctx)

	worker.RunTaskProcessor(ctx, waitGroup, redisOpt, store)
	runHttpServer(ctx, waitGroup, cf, store, tokenMaker, taskDistributor)
	runGrpcServer(ctx, waitGroup, cf, store, tokenMaker, taskDistributor)

	err = waitGroup.Wait()
	if err != nil {
		log.Fatal().Msgf("error group error: %v", err)
	}
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
func runHttpServer(
	ctx context.Context,
	waitGroup *errgroup.Group,
	cfg cf.Config,
	store db.Store,
	tokenMaker token.Maker,
	taskDistributor worker.TaskDistributor,
) {
	server, err := httpSv.NewServer(store, &cfg, tokenMaker, taskDistributor)
	if err != nil {
		log.Fatal().Msgf("cannot create HTTP server: %v", err)
	}

	setUpRouter(server)

	server.HttpServer = &http.Server{
		Addr:         server.Config.HttpServerAddress,
		Handler:      server.Router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	waitGroup.Go(func() error {
		return server.Start()
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		return server.Stop(context.Background())
	})
}

// runGrpcServer run grpc server
func runGrpcServer(
	ctx context.Context,
	waitGroup *errgroup.Group,
	cfg cf.Config,
	store db.Store,
	tokenMaker token.Maker,
	taskDistributor worker.TaskDistributor,
) {
	server, err := grpcSv.NewServer(store, &cfg, tokenMaker, taskDistributor)
	if err != nil {
		log.Fatal().Msgf("cannot create gRPC server: %v", err)
	}

	serviceHandler := grpcapi.NewServiceHandler(server)

	server.Listener, err = net.Listen("tcp", server.Config.GrpcServerAddress)
	if err != nil {
		log.Fatal().Msgf("cannot listen on %s: %v", server.Config.GrpcServerAddress, err)
	}

	grpcLogger := grpc.UnaryInterceptor(logger.GrpcLogger)
	server.GrpcServer = grpc.NewServer(grpcLogger)

	// Register service handler
	pb.RegisterSimpleBankServer(server.GrpcServer, serviceHandler)
	reflection.Register(server.GrpcServer)

	waitGroup.Go(func() error {
		return server.Start()
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		return server.Stop(ctx)
	})
}

// runGatewayServer run grpc-gateway server
func runGatewayServer(
	ctx context.Context,
	waitGroup *errgroup.Group,
	cfg cf.Config,
	store db.Store,
	tokenMaker token.Maker,
	taskDistributor worker.TaskDistributor,
) {
	server, err := grpcSv.NewServer(store, &cfg, tokenMaker, taskDistributor)
	if err != nil {
		log.Fatal().Msgf("cannot create gateway server: %v", err)
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

	err = pb.RegisterSimpleBankHandlerServer(ctx, grpcMux, serviceHandler)
	if err != nil {
		log.Fatal().Msgf("cannot register handler server: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", grpcMux)

	statikFS, err := fs.New()
	if err != nil {
		log.Fatal().Msgf("cannot create statik file system: %v", err)
	}

	swaggerHandler := http.StripPrefix("/swagger/", http.FileServer(statikFS))
	mux.Handle("/swagger/", swaggerHandler)

	httpServer := &http.Server{
		Handler: logger.HttpLogger(mux),
		Addr:    cfg.HttpServerAddress,
	}

	waitGroup.Go(func() error {
		log.Info().Msgf("starting HTTP gateway server on %s", cfg.HttpServerAddress)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("shutting down HTTP gateway server")
		err := httpServer.Shutdown(context.Background())

		if err != nil {
			log.Error().Err(err).Msg("Fail to shutdown HTTP gateway server")
			return err
		}

		log.Info().Msg("HTTP gateway server shutdown is complete")
		return nil
	})
}
