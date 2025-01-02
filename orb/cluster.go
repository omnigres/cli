package orb

import (
	"context"
	"database/sql"
	"io"
)

type OrbOptions struct {
	*Config
	Path string
}

type RunEventListener struct {
	OutputHandler func(cluster OrbCluster, reader io.Reader)
	Started       func(cluster OrbCluster)
	Ready         func(cluster OrbCluster)
	Stopped       func(cluster OrbCluster)
}

type OrbCluster interface {
	Configure(options OrbOptions) error
	Run(ctx context.Context, listeners ...RunEventListener) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Connect(ctx context.Context, database ...string) (*sql.DB, error)
	ConnectPsql(ctx context.Context, database ...string) error
	Port(ctx context.Context, name string) (int, error)
	Close() error
	Config() *Config
}
