package main

import (
	"context"
	"os"

	"github.com/DenzelPenzel/nyx/config"
	"github.com/DenzelPenzel/nyx/internal/app"
	"github.com/DenzelPenzel/nyx/internal/common"
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
	appEnv := c.String("env")
	cfg, err := config.LoadConfig(appEnv)
	ctx := context.Background()

	// Init logger
	logging.New(common.Env(cfg.App.Env))
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
