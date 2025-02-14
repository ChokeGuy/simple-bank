package server

import (
	"testing"

	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	pkg "github.com/ChokeGuy/simple-bank/pkg/config"
	"github.com/ChokeGuy/simple-bank/worker"

	"github.com/ChokeGuy/simple-bank/pkg/token"
	"github.com/ChokeGuy/simple-bank/pkg/token/paseto"
	"github.com/ChokeGuy/simple-bank/validations"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
)

// Server serves HTTP requests for our banking service.
type Server struct {
	Config          *pkg.Config
	Store           db.Store
	Router          *gin.Engine
	TokenMaker      token.Maker
	TaskDistributor worker.TaskDistributior
}

// NewServer creates a new HTTP server and set up routing.
func NewServer(
	store db.Store,
	config *pkg.Config,
	tokenMaker token.Maker,
	taskDistributor worker.TaskDistributior,
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
func NewTestServer(t *testing.T, store db.Store, cf *pkg.Config, taskDistributor worker.TaskDistributior) *Server {
	tokenMaker, err := paseto.NewPasetoMaker(cf.SymetricKey)
	require.NoError(t, err)

	server, err := NewServer(store, cf, tokenMaker, taskDistributor)
	require.NoError(t, err)

	return server
}

// Start runs the HTTP server on a specific address.
func (server *Server) Start(address string) error {
	return server.Router.Run(address)
}
