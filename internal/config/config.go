package config

import (
	"github.com/denzelpenzel/nyx/internal/common"
	"github.com/denzelpenzel/nyx/internal/utils"
	"github.com/urfave/cli"
	"net"
	"time"
)

type DBConfig struct {
	DBDir          string
	Backup         string
	ExpireInterval time.Duration
}

// ServerConfig ... Server configuration options
type ServerConfig struct {
	HTTPAddr      net.Addr
	HTTPAdvertise *net.TCPAddr

	JoinServerHost string
	JoinServerPort int
}

// Config ... Application level configuration defined by `FilePath` value
type Config struct {
	Environment  common.Env
	DBConfig     *DBConfig
	ServerConfig *ServerConfig
}

// NewConfig ... Initializer
func NewConfig(c *cli.Context) *Config {
	env := c.String("env")
	dbDir := c.String("db-dir")
	backup := c.String("backup")
	httpAddr, _ := utils.GetTCPAddr(c.String("server-addr"))
	httpAdvertise, _ := utils.GetTCPAddr(c.String("server-advertise"))
	dbExpireInterval, _ := time.ParseDuration(c.String("db-expire-interval"))

	config := &Config{
		Environment: common.Env(env),

		DBConfig: &DBConfig{
			DBDir:          dbDir,
			Backup:         backup,
			ExpireInterval: dbExpireInterval,
		},

		ServerConfig: &ServerConfig{
			HTTPAddr:      httpAddr,
			HTTPAdvertise: httpAdvertise,
		},
	}

	return config
}

// IsProduction ... Returns true if the env is production
func (cfg *Config) IsProduction() bool {
	return cfg.Environment == common.Production
}

// IsDevelopment ... Returns true if the env is development
func (cfg *Config) IsDevelopment() bool {
	return cfg.Environment == common.Development
}

// IsLocal ... Returns true if the env is local
func (cfg *Config) IsLocal() bool {
	return cfg.Environment == common.Local
}
