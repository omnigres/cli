package orb

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net"
)

type OrbOptions struct {
	*Config
	Path string
}

type OrbStartEventListener struct {
	Started func(cluster OrbCluster)
	Ready   func(cluster OrbCluster)
}

type OrbRunEventListener struct {
	OutputHandler func(cluster OrbCluster, reader io.Reader)
	Stopped       func(cluster OrbCluster)
}

type OrbClusterStartOptions struct {
	Attachment struct {
		ShouldAttach bool
		Listeners    []OrbRunEventListener
	}
	AutoRemove bool
	Listeners  []OrbStartEventListener
	Runfile    bool
}

type OrbCluster interface {
	Configure(options OrbOptions) error
	Start(ctx context.Context, options OrbClusterStartOptions, user *string, entryPoint []string) error
	StartWithCurrentUser(ctx context.Context, options OrbClusterStartOptions) error
	Stop(ctx context.Context) error
	Endpoints(ctx context.Context) ([]Endpoint, error)
	Connect(ctx context.Context, database ...string) (*sql.DB, error)
	ConnectPsql(ctx context.Context, database ...string) error
	Close() error
	Config() *Config
}

type Endpoint struct {
	Database string
	net.IP
	Port     int
	Protocol string
}

func (e *Endpoint) String() (s string) {
	switch e.Protocol {
	case "HTTP":
		s = fmt.Sprintf("http://%s:%d", e.IP.String(), e.Port)
	case "Postgres":
		s = fmt.Sprintf("postgres://omnigres:omnigres@%s:%d/%s", e.IP.String(), e.Port, e.Database)
	default:
		s = fmt.Sprintf("%s:%d", e.IP.String(), e.Port)
	}
	return
}
