package cmd

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/lib/pq"
	"github.com/omnigres/cli/orb"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate orbs",
	Long:  `By default, will migrate all listed orbs`,
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
		log.Debug("Migrating orbs", "orbs", orbs)
		err = assembleOrbs(
			ctx,
			cluster,
			dbReset,
			orbs,
			func(orbName string) string { return orbName },
		)
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

func assembleOrbs(
	ctx context.Context,
	cluster orb.OrbCluster,
	dbReset bool,
	orbs []string,
	databaseForOrb func(string) string,
) (err error) {
	logger := log.New(os.Stdout)
	logger.SetReportTimestamp(true)

	var db *sql.DB
	db, err = cluster.Connect(ctx, "omnigres")
	if err != nil {
		return
	}
	for _, orbName := range orbs {
		log.Infof("Assembling orb %s", orbName)
		dbName := databaseForOrb(orbName)
		logger.SetPrefix(fmt.Sprintf("[%s] ", dbName))
		var dbExists bool
		err := db.QueryRowContext(
			ctx,
			`select exists(select from pg_database where datname = $1)`,
			dbName,
		).Scan(&dbExists)
		if err != nil {
			log.Fatal(err)
		}

		if dbReset && dbExists {
			if err != sql.ErrNoRows {
				_, err = db.ExecContext(ctx, fmt.Sprintf(`drop database %q`, dbName))
				if err != nil {
					log.Fatal(err)
				}
			}
		}

		if dbReset || !dbExists {
			_, err = db.ExecContext(ctx, fmt.Sprintf(`create database %q`, dbName))
			if err != nil {
				log.Fatal(err)
			}
		}

		levels := make(map[string]log.Level)
		levels["info"] = log.InfoLevel
		levels["error"] = log.ErrorLevel
		reporting := func(notice *pq.Error) {
			var message map[string]interface{}
			err := json.NewDecoder(strings.NewReader(notice.Message)).Decode(&message)
			if err != nil {
				logger.Error(err)
			}

			mapToArray := func(m map[string]interface{}) []interface{} {
				result := make([]interface{}, 0, len(m)*2) // Allocate slice with enough capacity
				for key, value := range m {
					result = append(result, key, value)
				}
				return result
			}
			strippedMessage := maps.Clone(message)
			delete(strippedMessage, "type")
			delete(strippedMessage, "message")
			tags := mapToArray(strippedMessage)
			logger.Log(levels[message["type"].(string)], message["message"], tags...)
		}

		conn, err := db.Conn(ctx)
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		conn.Raw(func(driverConn any) error {
			pq.SetNoticeHandler(driverConn.(driver.Conn), reporting)
			return nil
		})

		orbSource := path.Join(orbName, "src")
		rows, err := conn.QueryContext(ctx,
			`select migration_filename, migration_statement, execution_error from omni_schema.assemble_schema($1, omni_vfs.local_fs('/mnt/host'), $2) where execution_error is not null`,
			fmt.Sprintf("dbname=%s user=omnigres", dbName), orbSource)

		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		for rows.Next() {
			var migration_filename, migration_statement, execution_error sql.NullString
			err = rows.Scan(&migration_filename, &migration_statement, &execution_error)
			if err != nil {
				log.Fatal(err)
			}
		}

	}
	return
}

var dbReset bool

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().BoolVarP(&dbReset, "dbReset", "r", false, "dbReset")
}
