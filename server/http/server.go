package server

import (
	"context"
	"net/http"
	"testing"

	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	pkg "github.com/ChokeGuy/simple-bank/pkg/config"
	"github.com/ChokeGuy/simple-bank/pkg/token"
	"github.com/ChokeGuy/simple-bank/pkg/token/paseto"
	"github.com/ChokeGuy/simple-bank/validations"
	"github.com/ChokeGuy/simple-bank/worker"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

// Server serves HTTP requests for our banking service.
type Server struct {
	Config          *pkg.Config
	Store           db.Store
	Router          *gin.Engine
	TokenMaker      token.Maker
	TaskDistributor worker.TaskDistributor
	HttpServer      *http.Server
}

// NewServer creates a new HTTP server and set up routing.
func NewServer(
	store db.Store,
	config *pkg.Config,
	tokenMaker token.Maker,
	taskDistributor worker.TaskDistributor,
) (*Server, error) {
	server := &Server{
		Store:           store,
		TokenMaker:      tokenMaker,
		Config:          config,
		TaskDistributor: taskDistributor,
	}

	router := gin.Default()

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("currency", validations.ValidCurrency)
		v.RegisterValidation("password", validations.ValidPassword)
	}

	server.Router = router
	return server, nil
}

// NewTestServer creates a new HTTP server for testing.
func NewTestServer(t *testing.T, store db.Store, cf *pkg.Config, taskDistributor worker.TaskDistributor) *Server {
	tokenMaker, err := paseto.NewPasetoMaker(cf.SymetricKey)
	require.NoError(t, err)

	server, err := NewServer(store, cf, tokenMaker, taskDistributor)
	require.NoError(t, err)

	return server
}

func (server *Server) Start() error {
	log.Info().Msgf("starting HTTP server on %s", server.Config.HttpServerAddress)
	if err := server.HttpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (server *Server) Stop(ctx context.Context) error {
	log.Info().Msg("gracefully stopping HTTP server")
	err := server.HttpServer.Shutdown(ctx)

	if err != nil {
		log.Error().Err(err).Msg("fail to stop HTTP server")
		return err
	}
	log.Info().Msg("HTTP server shutdown is complete")
	return nil
}
