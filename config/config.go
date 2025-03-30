package config

import (
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/utils"
	"github.com/urfave/cli"
)

type Config struct {
	Environment   common.Env
	CPUProfile    string
	ServerConfig  *ServerConfig
	StorageConfig *StorageConfig
}

// ServerConfig server configuration options
type ServerConfig struct {
	SocketAddr    net.Addr
	HTTPAddr      net.Addr
	HTTPAdvertise *net.TCPAddr

	JoinServerHost string
	JoinServerPort int

	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
}

type StorageConfig struct {
	// DB config
	DBCfg *DBConfig

	// raft working dir
	RaftDir string

	// Node ID
	RaftID string

	// RaftAddr is the RPC address used by Nomad. This should be reachable
	// by the other servers and clients
	RaftAddr *net.TCPAddr

	// RaftAdvertise is the address that is advertised to client nodes for
	// the RPC endpoint. This can differ from the RPC address, if for example
	// the RaftAddr is unspecified "0.0.0.0:4646", but this address must be
	// reachable
	RaftAdvertise *net.TCPAddr

	RaftHeartbeatTimeout time.Duration

	// Join list of address
	Join string

	RaftElectionTimeout  time.Duration
	RaftApplyTimeout     time.Duration
	RaftOpenTimeout      time.Duration
	RaftSnapThreshold    uint64
	RaftShutdownOnRemove bool
}

type DBConfig struct {
	ExpireIntervalInSeconds int
	Dir                     string
	Restore                 string
	Backup                  string
}

func LoadConfig(c *cli.Context) *Config {
	dataPath := c.Args()[0]

	dbFilename := c.String("db-filename")
	dbRestore := c.String("db-restore")
	dbBackup := c.String("db-backup")
	ExpireIntervalInSeconds := c.Int("db-expire-interval")

	env := c.String("env")

	httpAddr, _ := utils.GetTCPAddr(c.String("server-addr"))
	socketAddr, _ := utils.GetTCPAddr(c.String("socket-addr"))
	httpAdvertise, _ := utils.GetTCPAddr(c.String("server-advertise"))

	cpuProfile := c.String("cpu_profile")

	raftAddr, _ := utils.GetTCPAddr(c.String("raft-addr"))
	raftAdvertise, _ := utils.GetTCPAddr(c.String("raft-advertise"))

	raftNodeID := c.String("node-id")
	if raftNodeID == "" {
		raftNodeID = raftAddr.String()
	}

	allowedOrigins := strings.Split(c.String("allowed-origins"), ",")
	allowedMethods := strings.Split(c.String("allowed-methods"), ",")
	allowedHeaders := strings.Split(c.String("allowed-headers"), ",")

	raftHeartbeatTimeout, _ := time.ParseDuration(c.String("raft-heartbeat-timeout"))
	raftElectionTimeout, _ := time.ParseDuration(c.String("raft-election-timeout"))
	raftApplyTimeout, _ := time.ParseDuration(c.String("raft-apply-timeout"))
	raftOpenTimeout, _ := time.ParseDuration(c.String("raft-open-timeout"))
	raftSnapThreshold := c.Uint64("raft-snap-threshold")
	raftShutdownOnRemove := c.Bool("raft-shutdown-on-remove")

	join := c.String("join")

	config := &Config{
		Environment: common.Env(env),
		CPUProfile:  cpuProfile,

		ServerConfig: &ServerConfig{
			SocketAddr:     socketAddr,
			HTTPAddr:       httpAddr,
			HTTPAdvertise:  httpAdvertise,
			AllowedOrigins: allowedOrigins,
			AllowedMethods: allowedMethods,
			AllowedHeaders: allowedHeaders,
		},

		StorageConfig: &StorageConfig{
			DBCfg: &DBConfig{
				Dir:                     filepath.Join(dataPath, dbFilename),
				ExpireIntervalInSeconds: ExpireIntervalInSeconds,
				Restore:                 dbRestore,
				Backup:                  dbBackup,
			},
			RaftID:               raftNodeID,
			RaftDir:              dataPath,
			RaftAddr:             raftAddr,
			RaftAdvertise:        raftAdvertise,
			RaftHeartbeatTimeout: raftHeartbeatTimeout,
			RaftElectionTimeout:  raftElectionTimeout,
			RaftApplyTimeout:     raftApplyTimeout,
			RaftOpenTimeout:      raftOpenTimeout,
			RaftSnapThreshold:    raftSnapThreshold,
			RaftShutdownOnRemove: raftShutdownOnRemove,
			Join:                 join,
		},
	}

	return config
}
