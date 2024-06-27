package main

import (
	"context"
	"github.com/denzelpenzel/nyx/internal/app"
	"github.com/denzelpenzel/nyx/internal/config"
	"github.com/denzelpenzel/nyx/internal/logging"
	"github.com/urfave/cli"
	"go.uber.org/zap"
	"os"
)

func main() {
	ctx := context.Background()
	logger := logging.WithContext(ctx)

	a := cli.NewApp()
	a.Name = "fast db"
	a.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "env",
			Value: "local",
			Usage: "Set the application env",
		},
		// db
		&cli.StringFlag{
			Name:  "db-dir",
			Value: "-db-tmp-",
			Usage: "Set the db dirname",
		},
		&cli.StringFlag{
			Name:  "restore",
			Value: "",
			Usage: "Set the backup file name for load",
		},
		// server config
		&cli.StringFlag{
			Name:  "slave-addr",
			Value: "127.0.0.1:4002",
			Usage: "slave TCP address",
		},
		&cli.Uint64Flag{
			Name:  "keep-alive",
			Value: 10,
			Usage: "Keep-alive connection, in seconds",
		},
		&cli.StringFlag{
			Name:  "backup",
			Value: "",
			Usage: "specify backup filename",
		},
		&cli.StringFlag{
			Name:  "server-addr",
			Value: "localhost:4001",
			Usage: "HTTP server bind address",
		},
		&cli.StringFlag{
			Name:  "db-expire-interval",
			Value: "60s",
			Usage: "Set the expiration interval for the keys",
		},
		// raft
	}
	a.Usage = "Fast db Application"
	a.Description = "Fast db"
	a.Action = RunFastCache
	a.Commands = []cli.Command{}

	err := a.Run(os.Args)
	if err != nil {
		logger.Fatal("Error running application", zap.Error(err))
	}
}

// RunFastCache ... Application entry point
func RunFastCache(c *cli.Context) error {
	cfg := config.NewConfig(c)
	ctx := context.Background()

	// Init logger
	logging.New(cfg.Environment)
	logger := logging.WithContext(ctx)

	app, shutDown, err := app.NewFastCacheApp(ctx, cfg)

	if err != nil {
		logger.Fatal("Error creating nyx application", zap.Error(err))
		return err
	}

	logger.Info("Starting nyx server")

	if err := app.Start(); err != nil {
		logger.Fatal("Error starting nyx server", zap.Error(err))
		return err
	}

	app.ListenForShutdown(shutDown)
	logger.Debug("Waiting for all application threads to end")

	logger.Info("Successful nyx shutdown")
	return nil
}
