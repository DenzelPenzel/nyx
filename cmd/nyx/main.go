package main

import (
	"context"
	"os"

	"github.com/DenzelPenzel/nyx/config"
	"github.com/DenzelPenzel/nyx/internal/app"
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
		&cli.StringFlag{
			Name:  "node-id",
			Value: "",
			Usage: "Raft unique node name",
		},
		&cli.StringFlag{
			Name:  "socket-addr",
			Value: "localhost:4003",
			Usage: "HTTP server bind address",
		},
		&cli.StringFlag{
			Name:  "server-addr",
			Value: "localhost:4001",
			Usage: "HTTP server bind address",
		},
		&cli.StringFlag{
			Name:  "server-advertise",
			Value: "",
			Usage: "Advertised HTTP address",
		},
		&cli.StringFlag{
			Name:  "raft-dir",
			Value: "tmp",
			Usage: "Set the Raft dir folder",
		},
		&cli.StringFlag{
			Name:  "raft-addr",
			Value: "localhost:4002",
			Usage: "Raft communication address",
		},
		&cli.StringFlag{
			Name:  "raft-advertise",
			Value: "",
			Usage: "Advertised Raft communication address",
		},
		&cli.StringFlag{
			Name:  "raft-heartbeat-timeout",
			Value: "1s",
			Usage: "Raft heartbeat timeout",
		},
		&cli.StringFlag{
			Name:  "raft-election-timeout",
			Value: "1s",
			Usage: "Raft election timeout",
		},
		&cli.StringFlag{
			Name:  "raft-apply-timeout",
			Value: "10s",
			Usage: "Raft apply timeout",
		},
		&cli.StringFlag{
			Name:  "raft-open-timeout",
			Value: "120s",
			Usage: "Set the time duration for the initial application of Raft logs. To skip the wait, use a duration of 0s",
		},
		&cli.Uint64Flag{
			Name:  "raft-snap-threshold",
			Value: 8192,
			Usage: "Comma-delimited list of nodes, through which a cluster can be joined (proto://host:port)",
		},
		&cli.BoolFlag{
			Name:     "raft-shutdown-on-remove",
			Required: false,
			Usage:    "Shutdown Raft if node removed",
		},
		&cli.StringFlag{
			Name:  "db-filename",
			Value: "db.nyx",
			Usage: "Set the database file name",
		},
		&cli.StringFlag{
			Name:  "allowed-origins",
			Value: "*",
			Usage: "Allowed origins for HTTP service",
		},
		&cli.StringFlag{
			Name:  "allowed-methods",
			Value: "*",
			Usage: "Allowed methods for HTTP service",
		},
		&cli.StringFlag{
			Name:  "allowed-headers",
			Value: "*",
			Usage: "Allowed headers for HTTP service",
		},
		&cli.StringFlag{
			Name:  "dsn",
			Value: "",
			Usage: "SQLite DSN parameters. E.g. 'cache=shared&mode=memory'",
		},
		&cli.StringFlag{
			Name:  "join",
			Value: "",
			Usage: "Comma-delimited list of nodes, through which a cluster can be joined (proto://host:port)",
		},
		&cli.StringFlag{
			Name:  "cpu_profile",
			Value: "",
			Usage: "Set the path to the file for CPU profiling information",
		},
		&cli.BoolFlag{
			Name:     "in-memory",
			Required: false,
			Usage:    "Use im-memory mode for the SQLite database",
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
	ctx := context.Background()
	cfg := config.LoadConfig(c)

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
