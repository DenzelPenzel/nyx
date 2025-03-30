package app

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/DenzelPenzel/nyx/config"
	"github.com/DenzelPenzel/nyx/internal/logging"
	"github.com/DenzelPenzel/nyx/internal/network"
	"github.com/DenzelPenzel/nyx/internal/nyx"
	"github.com/DenzelPenzel/nyx/internal/server"
	"go.uber.org/zap"
)

// Application Nyx app struct
type Application struct {
	ctx   context.Context
	cfg   *config.Config
	lis   server.ListenConst
	store nyx.DBHandler
}

func NewFastCacheApp(ctx context.Context, cfg *config.Config) (*Application, func(), error) {
	store, cleanup, err := setupStore(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	lis := server.TCPListener(cfg.ServerConfig.SocketAddr)

	if err := setupHTTPServer(ctx, cfg, store); err != nil {
		return nil, nil, err
	}

	app := &Application{
		ctx:   ctx,
		cfg:   cfg,
		lis:   lis,
		store: store,
	}

	return app, cleanup, nil
}

// Start starts the application
func (a *Application) Start() error {
	// OpenShard metrics server
	// a.metrics.OpenShard()

	go server.ListenAndServe(a.ctx, a.lis, a.store)

	// metrics.WithContext(a.ctx).RecordUp()

	return nil
}

// ListenForShutdown handles and listens for shutdown
func (a *Application) ListenForShutdown(stop func()) {
	done := <-a.End() // Blocks until an OS signal is received

	logging.WithContext(a.ctx).
		Info("Received shutdown OS signal", zap.String("signal", done.String()))
	stop()
}

// End returns a channel that will receive an OS signal
func (a *Application) End() <-chan os.Signal {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	return sigs
}

func setupHTTPServer(ctx context.Context, cfg *config.Config, s *nyx.Nyx) error {
	srv := server.NewHTTPServer(ctx, cfg.ServerConfig, s)
	return srv.ListenAndServeHTTP()
}

func setupStore(ctx context.Context, cfg *config.Config) (*nyx.Nyx, func(), error) {
	logger := logging.WithContext(ctx)
	// configure network layer
	nt := network.NewNetwork()
	raftAddr := cfg.StorageConfig.RaftAddr

	if err := nt.Open(raftAddr.String()); err != nil {
		logger.Fatal("failed to open network layer", zap.String("addr", raftAddr.String()), zap.Error(err))
	}

	n := nyx.New(ctx, cfg, nt, nil)

	// Determine join addresses, if necessary
	canJoin, err := n.JoinAllowed(cfg.StorageConfig.RaftDir)
	if err != nil {
		logger.Fatal("Unable to determine if join permitted", zap.Error(err))
	}

	var joins []string
	if canJoin {
		if cfg.StorageConfig.Join != "" {
			joins = strings.Split(cfg.StorageConfig.Join, ",")
		}
	}

	// Initialize the Raft server
	if err := n.Open(len(joins) == 0); err != nil {
		logger.Fatal("Unable to open raft store", zap.Error(err))
	}

	apiAdvertise := cfg.ServerConfig.HTTPAddr.String()
	if cfg.ServerConfig.HTTPAdvertise.IP != nil {
		apiAdvertise = cfg.ServerConfig.HTTPAdvertise.String()
	}

	meta := map[string]string{
		"api_addr": apiAdvertise,
	}

	if len(joins) > 0 {
		logger.Info("list of joining addresses", zap.String("address list", strings.Join(joins, ",")))
		raftAdvertise := cfg.StorageConfig.RaftAddr
		if cfg.StorageConfig.RaftAdvertise.IP != nil {
			raftAdvertise = cfg.StorageConfig.RaftAdvertise
		}
		// Join to the raft cluster
		joinedAddr, err := server.Join(joins, n.ID(), raftAdvertise, meta)
		if err != nil {
			logger.Fatal("Failed to join raft cluster", zap.Error(err))
		} else {
			logger.Info("Successfully joined raft cluster", zap.String("joined addr", joinedAddr))
		}
	} else {
		logger.Info("No join addresses specified")
	}

	// Wait until the store is in full consensus
	leader, err := n.WaitForLeader(n.OpenTimeout)
	if err != nil {
		logger.Fatal("Failed to achieve consensus", zap.Error(err))
		return nil, nil, err
	}

	logger.Info("Found the leader", zap.String("leader", leader))
	err = n.WaitForApplied(n.OpenTimeout)
	if err != nil {
		logger.Fatal("Can't apply last index", zap.Error(err))
		return nil, nil, err
	}

	if err := n.SetMetadata(meta); err != nil && !errors.Is(err, nyx.ErrNotLeader) {
		logger.Fatal("Unable to store metadata within the Raft consensus cluster", zap.Error(err))
	}

	stop := func() {
		logging.WithContext(ctx).Info("Starting to shutdown store")

		if err := n.Shutdown(); err != nil {
			logging.WithContext(ctx).Fatal("Failed to close store", zap.Error(err))
		}
	}

	return n, stop, nil
}
