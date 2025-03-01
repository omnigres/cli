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

var revisionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List revisions",
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
		log.Debug("List revisions in orbs", "orbs", orbs)
		listRevisions(
			ctx,
			cluster,
			dbReset,
			orbs,
			func(orbName string) string { return orbName },
		)
	},
}

func listRevisions(
	ctx context.Context,
	cluster orb.OrbCluster,
	dbReset bool,
	orbs []string,
	databaseForOrb func(string) string,
) (err error) {
	var db *sql.DB
	db, err = cluster.Connect(ctx, "omnigres")
	if err != nil {
		log.Error("Could not connect to orb. Ensure the docker container is running, perhaps 'omnigres start' will fix it.")
		return
	}
	for _, orbName := range orbs {
		conn, err := db.Conn(ctx)
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		if err != nil {
			log.Error(err)
			return err
		}

		var rows *sql.Rows
		rows, err = conn.QueryContext(
			ctx,
			`with revs as (select revision, parents, metadata from omni_schema.schema_revisions(omni_vfs.local_fs($1), 'revisions')) 
                     select revision, not exists (select from revs r1 where r.revision = any(r1.parents)) as top from revs as r order by top`,
			fmt.Sprintf("/mnt/host/%s", orbName),
		)
		if err != nil {
			log.Error(err)
			return err
		}
		var revision string
		var top bool
		for rows.Next() {
			err = rows.Scan(&revision, &top)
			if top {
				fmt.Printf("* %s\n", revision)
			} else {
				fmt.Printf("  %s\n", revision)
			}
		}

	}
	return
}
