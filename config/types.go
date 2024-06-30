package config

type AppConfig struct {
	Env         string `env:"APP_ENV,notEmpty"` // development, main, staging
	IsDebug     bool   `env:"APP_IS_DEBUG,notEmpty"`
	BaseAddr    string `env:"APP_BASE_ADDR,notEmpty"`
	ReplicaAddr string `env:"REPLICA_ADDR,notEmpty"`
}

type DBConfig struct {
	ExpireIntervalInSeconds int    `env:"DB_EXPIRE_INTERVAL_SECONDS,notEmpty"`
	Dir                     string `env:"DB_DATA_DIR,notEmpty"`
	Restore                 string `env:"RESTORE"`
	Backup                  string `env:"BACKUP"`
}
