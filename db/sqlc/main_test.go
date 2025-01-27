package sqlc

import (
	"database/sql"
	"log"
	"os"
	"testing"

	cf "github.com/ChokeGuy/simple-bank/pkg/config"

	_ "github.com/lib/pq"
)

var testQueries *Queries
var testDb *sql.DB

func TestMain(m *testing.M) {

	var err error
	cf, err := cf.LoadConfig("../..")

	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	testDb, err = sql.Open(cf.DBDriver, cf.DBSource)

	if err != nil {
		log.Fatalf("cannot connect to db: %v", err)
	}

	testQueries = New(testDb)

	os.Exit(m.Run())
}
