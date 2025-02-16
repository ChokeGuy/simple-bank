package dbmigrations

import (
	cf "github.com/ChokeGuy/simple-bank/pkg/config"
	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/rs/zerolog/log"
)

func RunDBMigration(cfg cf.Config) {
	migration, err := migrate.New(cfg.MigrationUrl, cfg.DBSource)
	if err != nil {
		log.Fatal().Msgf("cannot create migration: %v", err)
	}

	if err = migration.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal().Msgf("failed to run migrate up: %v", err)
	}

	log.Info().Msg("db migration successfully")
}
