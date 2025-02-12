package server

import (
	"testing"

	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	"github.com/ChokeGuy/simple-bank/pb"
	pkg "github.com/ChokeGuy/simple-bank/pkg/config"

	"github.com/ChokeGuy/simple-bank/pkg/token"
	"github.com/ChokeGuy/simple-bank/pkg/token/paseto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Server serves GRPC requests for our banking service.
type Server struct {
	pb.UnimplementedSimpleBankServer
	Config     *pkg.Config
	Store      db.Store
	Router     *gin.Engine
	TokenMaker token.Maker
}

// NewServer creates a new GRPC server.
func NewServer(
	store db.Store,
	config *pkg.Config,
	tokenMaker token.Maker,
) (*Server, error) {

	server := &Server{
		Store:      store,
		TokenMaker: tokenMaker,
		Config:     config,
	}

	return server, nil
}

// NewTestServer creates a new GRPC server for testing.
func NewTestServer(t *testing.T, store db.Store, cf *pkg.Config) *Server {
	tokenMaker, err := paseto.NewPasetoMaker(cf.SymetricKey)
	require.NoError(t, err)

	server, err := NewServer(store, cf, tokenMaker)
	require.NoError(t, err)

	return server
}
