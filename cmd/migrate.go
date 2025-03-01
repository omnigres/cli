package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/omnigres/cli/orb"
	"github.com/spf13/cobra"
	"os"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate revisions",
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
		log.Debug("Migrate revisions in orbs", "orbs", orbs)
		migrateRevisions(
			ctx,
			cluster,
			orbs,
		)
	},
}

func migrateRevisions(
	ctx context.Context,
	cluster orb.OrbCluster,
	orbs []string,
) (err error) {
	var db *sql.DB
	db, err = cluster.Connect(ctx, "omnigres")
	if err != nil {
		log.Error("Could not connect to orb. Ensure the docker container is running, perhaps 'omnigres start' will fix it.")
		return
	}
	for _, orbName := range orbs {
		log.Infof("Migrating orb %s", orbName)
		conn, err := db.Conn(ctx)
		if err != nil {
			log.Error(err)
			return err
		}
		err = setupCloudevents(ctx, conn)
		if err != nil {
			log.Error(err)
			return err
		}
		defer conn.Close()

		var dbExists bool
		err = db.QueryRowContext(
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

		var rows *sql.Rows
		rows, err = conn.QueryContext(
			ctx,
			`select revision, omni_schema.migrate_to_schema_revision(omni_vfs.local_fs($1), 'revisions', revision, $2) is null as success
from omni_schema.schema_revisions(omni_vfs.local_fs($1), 'revisions')`,
			fmt.Sprintf("/mnt/host/%s", orbName),
			fmt.Sprintf("dbname=%s user=omnigres", orbName),
		)
		if err != nil {
			log.Error(err)
			return err
		}
		var revision string
		var success bool
		for rows.Next() {
			err = rows.Scan(&revision, &success)
			if success {
				log.Infof("✅ Applied revision %s", revision)
			} else {
				log.Infof("🔴 Failed to apply revision %s", revision)
			}
		}
	}

	return nil
}
