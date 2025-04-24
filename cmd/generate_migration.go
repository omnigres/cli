package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"maps"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/lib/pq"
	"github.com/omnigres/cli/orb"
	"github.com/spf13/cobra"
)

var generateMigrationsCmd = &cobra.Command{
	Use:   "generate-migrations",
	Short: "Generate migrations for revisions",
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
		log.Debug("Generating migrations for revisions in orbs", "orbs", orbs)
		generateMigrations(
			ctx,
			cluster,
			orbs,
		)
	},
}

func generateMigrations(
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
		log.Infof("Generating migrations for orb %s", orbName)
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
			`with fs as (select omni_vfs.local_fs($1)),
      revisions as (
			  select name 
        from fs, omni_vfs.list(fs.local_fs, 'revisions')
        where kind = 'dir'
			) 
      select r.name as revision, migrations
      from fs, revisions r,
      lateral omni_schema.generate_migration(
        fs.local_fs,
        'revisions',
        r.name::omni_schema.revision_id
      ) migrations
      where not exists (
        select from fs, omni_vfs.list(
          fs.local_fs,
          format('revisions/%s/migrate.sql', r.name)
        ) 
        where kind = 'file'
      ) order by r.name`,
			fmt.Sprintf("/mnt/host/%s", orbName),
		)
		if err != nil {
			log.Error(err)
			return err
		}

		// split into separate revisions
		var revision string
		var migrations []string
		revisions := make(map[string][]string)
		for rows.Next() {
			err = rows.Scan(&revision, pq.Array(&migrations))
			log.Debugf("Creating migration for %s...", revision)
			if revisions[revision] == nil {
				revisions[revision] = []string{migrations[0]}
			} else {
				revisions[revision] = append(revisions[revision], migrations[0])
			}
		}
		for revision := range maps.Keys(revisions) {
			log.Debugf("Generating file for revision: %s", revision)
			fileName := fmt.Sprintf(
				"%s/%s/revisions/%s/migrate.sql",
				workspace,
				orbName,
				revision,
			)
			file, err := os.OpenFile(fileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
			if err != nil {
				log.Fatal("Could not open file %s", fileName)
			}
			log.Debugf("Writing file %s", fileName)
			file.WriteString(strings.Join(revisions[revision], ";\n") + ";\n")
			file.Close()
		}
	}

	return nil
}

