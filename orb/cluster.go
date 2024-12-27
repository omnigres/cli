package orb

import (
	"context"
	"database/sql"
)

type OrbOptions struct {
	*Config
	Path string
}

type OrbCluster interface {
	Configure(options OrbOptions) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Connect(ctx context.Context, database ...string) (*sql.DB, error)
	ConnectPsql(ctx context.Context, database ...string) error
	Port(ctx context.Context, name string) (int, error)
	Close() error
	Config() *Config
}
