package main

import (
	"context"
	"os"

	"github.com/DenzelPenzel/nyx/internal/app"
	"github.com/DenzelPenzel/nyx/internal/config"
	"github.com/DenzelPenzel/nyx/internal/logging"
	"github.com/urfave/cli"
	"go.uber.org/zap"
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
			Name:  "data-dir",
			Value: "-data-tmp-",
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
		&cli.StringFlag{
			Name:  "backup",
			Value: "",
			Usage: "specify backup filename",
		},
		&cli.StringFlag{
			Name:  "hostname",
			Value: "localhost:4001",
			Usage: "Addr to listen on for external connections",
		},
		&cli.StringFlag{
			Name:  "db-expire-interval",
			Value: "60s",
			Usage: "Set the expiration interval for the keys",
		},
	}
	a.Usage = "Nyx kvs"
	a.Description = "High-speed, key-value storage"
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

	a, shutDown, err := app.NewFastCacheApp(ctx, cfg)

	if err != nil {
		logger.Fatal("Error creating nyx application", zap.Error(err))
		return err
	}

	logger.Info("Starting nyx server")

	if err := a.Start(); err != nil {
		logger.Fatal("Error starting nyx server", zap.Error(err))
		return err
	}

	a.ListenForShutdown(shutDown)
	logger.Debug("Waiting for all application threads to end")

	logger.Info("Successful nyx shutdown")
	return nil
}
