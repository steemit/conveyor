// Package main is the conveyor server entry point.
package main

import (
	"os"

	"github.com/rs/zerolog/log"

	"github.com/steemit/conveyor/internal/config"
	"github.com/steemit/conveyor/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
		os.Exit(1)
	}

	app, err := server.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create app")
		os.Exit(1)
	}

	if err := app.Run(); err != nil {
		log.Fatal().Err(err).Msg("server error")
		os.Exit(1)
	}
}
