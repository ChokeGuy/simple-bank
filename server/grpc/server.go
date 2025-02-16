package server

import (
	"context"
	"net"
	"testing"

	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	pkg "github.com/ChokeGuy/simple-bank/pkg/config"
	"github.com/ChokeGuy/simple-bank/worker"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/ChokeGuy/simple-bank/pkg/token"
	"github.com/ChokeGuy/simple-bank/pkg/token/paseto"
	"github.com/stretchr/testify/require"
)

// Server serves GRPC requests for our banking service.
type Server struct {
	Config          *pkg.Config
	Store           db.Store
	TokenMaker      token.Maker
	TaskDistributor worker.TaskDistributor
	GrpcServer      *grpc.Server
	Listener        net.Listener
}

// NewServer creates a new GRPC server.
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

	return server, nil
}

// NewTestServer creates a new GRPC server for testing.
func NewTestServer(t *testing.T, store db.Store, cf *pkg.Config, taskDistributor worker.TaskDistributor) *Server {
	tokenMaker, err := paseto.NewPasetoMaker(cf.SymetricKey)
	require.NoError(t, err)

	server, err := NewServer(store, cf, tokenMaker, taskDistributor)
	require.NoError(t, err)

	return server
}

func (server *Server) Start() error {
	log.Info().Msgf("starting gRPC server on %s", server.Config.GrpcServerAddress)
	err := server.GrpcServer.Serve(server.Listener)

	if err != nil {
		log.Error().Err(err).Msg("cannot start gRPC server")
		return err
	}

	return nil
}

func (server *Server) Stop(ctx context.Context) error {
	log.Info().Msg("gracefully stopping gRPC server")
	server.GrpcServer.GracefulStop()
	return nil
}
