package sqlc

import (
	"context"
	"log"
	"os"
	"testing"

	cf "github.com/ChokeGuy/simple-bank/pkg/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

var testStore Store

func TestMain(m *testing.M) {

	var err error
	cf, err := cf.LoadConfig("../..")

	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	connPool, err := pgxpool.New(context.Background(), cf.DBSource)

	if err != nil {
		log.Fatalf("cannot connect to db: %v", err)
	}

	testStore = NewStore(connPool)

	os.Exit(m.Run())
}
