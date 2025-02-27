package cmd

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/lib/pq"
	"github.com/omnigres/cli/orb"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

var captureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Capture orb schema changes",
	Long:  `By default, will capture all listed orbs`,
	Run: func(cmd *cobra.Command, args []string) {
		var cluster orb.OrbCluster
		var err error
		cluster, err = getOrbCluster()
		if err != nil {
			log.Fatal(err)
		}

		log.Debug("Workspace", "workspace", workspace)
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}

		orbs, err := currentOrbs(cluster, cwd)
		if err != nil {
			log.Fatal(err)
		}

		ctx := context.Background()
		log.Debug("Capturing orbs", "orbs", orbs)
		captureOrbs(ctx, cluster, orbs)
	},
}

func currentOrbs(cluster orb.OrbCluster, dir string) ([]string, error) {
	if dir == workspace {
		// return all orbs when running in workspace root
		return lo.Map(
			cluster.Config().Orbs,
			func(cfg orb.OrbCfg, _ int) string { return cfg.Name },
		), nil
	}

	parent := filepath.Dir(dir)
	if parent == workspace {
		// First level below workspace, this is the current orb
		cwd, err := os.Getwd()
		return []string{filepath.Base(cwd)}, err
	}

	if parent == dir {
		// Reached the root, return as in workspace root
		return currentOrbs(cluster, workspace)
	}

	return currentOrbs(cluster, parent)
}

func captureSchemaRevision(
	ctx context.Context,
	cluster orb.OrbCluster,
	orbName string,
) (err error) {
	var db *sql.DB
	db, err = cluster.Connect(ctx, orbName)
	if err != nil {
		return
	}

	log.Infof("Capturing schema for orb %s", orbName)
	_, err = db.ExecContext(ctx, "create extension if not exists omni_schema cascade")
	if err != nil {
		return err
	}

	jsonMessageReporting := func(notice *pq.Error) {
		log.Infof("%s", notice.Message)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer conn.Close()
	conn.Raw(func(driverConn any) error {
		pq.SetNoticeHandler(driverConn.(driver.Conn), jsonMessageReporting)
		return nil
	})
	var revision string
	err = db.QueryRowContext(
		ctx,
		`select omni_schema.capture_schema_revision(omni_vfs.local_fs($1), 'src', 'revisions')`,
		fmt.Sprintf("/mnt/host/%s", orbName),
	).Scan(&revision)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.ExecContext(ctx, fmt.Sprintf("drop database \"%s\"", revision))
	if err != nil {
		log.Errorf("Could not remove revision. You can try to manually remove using DROP DATABASE %s", revision)
	}
	log.Infof("📦 Revision %s created", revision)

	return
}

func captureOrbs(
	ctx context.Context,
	cluster orb.OrbCluster,
	orbs []string,
) (err error) {
	var db *sql.DB
	db, err = cluster.Connect(ctx, "omnigres")
	if err != nil {
		return
	}
	for _, orbName := range orbs {
		log.Infof("Capturing orb %s", orbName)
		var dbExists bool
		err := db.QueryRowContext(
			ctx,
			`select exists(select from pg_database where datname = $1)`,
			orbName,
		).Scan(&dbExists)
		if err != nil {
			log.Fatal(err)
		}

		if !dbExists {
			_, err = db.ExecContext(ctx, fmt.Sprintf(`create database %q`, orbName))
			if err != nil {
				log.Fatal(err)
			}
		}

		captureSchemaRevision(ctx, cluster, orbName)
	}
	return
}

func init() {
	rootCmd.AddCommand(captureCmd)
}
