package app

import (
	"context"
	"github.com/denzelpenzel/nyx/internal/config"
	"github.com/denzelpenzel/nyx/internal/db"
	"github.com/denzelpenzel/nyx/internal/logging"
	"github.com/denzelpenzel/nyx/internal/nyx"
	"github.com/denzelpenzel/nyx/internal/server"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
)

// Application ... Pessimism app struct
type Application struct {
	ctx context.Context
	cfg *config.Config
	l   server.ListenConst
	db  db.DB
}

func NewFastCacheApp(ctx context.Context, cfg *config.Config) (*Application, func(), error) {
	d, err := db.NewDB(ctx, cfg.DBConfig)
	if err != nil {
		return nil, nil, err
	}

	app := &Application{ctx: ctx, cfg: cfg, db: d}
	app.l = server.TCPListener(cfg.ServerConfig.HTTPAddr)

	return app, func() {}, nil
}

// Start ... Starts the application
func (a *Application) Start() error {
	// Run metrics server
	// a.metrics.Run()

	// Run the API server
	go server.ListenAndServe(a.ctx, a.l, a.db, nyx.NewNyx)

	// if err := a.server.Run(); err != nil {
	// return err
	// }

	// metrics.WithContext(a.ctx).RecordUp()

	return nil
}

// ListenForShutdown ... Handles and listens for shutdown
func (a *Application) ListenForShutdown(stop func()) {
	done := <-a.End() // Blocks until an OS signal is received

	logging.WithContext(a.ctx).
		Info("Received shutdown OS signal", zap.String("signal", done.String()))
	stop()
}

// End ... Returns a channel that will receive an OS signal
func (a *Application) End() <-chan os.Signal {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	return sigs
}