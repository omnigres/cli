package cmd

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/lib/pq"
	"github.com/omnigres/cli/orb"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"maps"
	"os"
	"path"
	"strings"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate [orb...]",
	Short: "Migrate orbs",
	Long:  `By default, will migrate all listed orbs`,
	Run: func(cmd *cobra.Command, args []string) {
		var cluster orb.OrbCluster
		var err error
		cluster, err = getOrbCluster()
		if err != nil {
			panic(err)
		}
		ctx := context.Background()
		orbs := lo.Map(cluster.Config().Orbs, func(cfg orb.OrbCfg, _ int) string { return cfg.Name })
		err = migrate(ctx, cluster, dbReset, orbs, orbs)

	},
}

func migrate(ctx context.Context, cluster orb.OrbCluster, dbReset bool, orbs []string, databases []string) (err error) {
	if len(orbs) != len(databases) {
		err = errors.New("orbs and databases have to be of the same size")
		return
	}
	logger := log.New(os.Stderr)
	logger.SetReportTimestamp(true)
	logger.Info("Starting migration...")

	var db *sql.DB
	db, err = cluster.Connect(ctx, "omnigres")
	if err != nil {
		return
	}
	for i, dbName := range databases {
		logger.SetPrefix(fmt.Sprintf("[%s] ", dbName))
		var datname string
		if databaseName != "" {
			dbName = databaseName
		}
		err := db.QueryRowContext(ctx, `select datname from pg_database where datname = $1`, dbName).Scan(&datname)
		if err != nil && err != sql.ErrNoRows {
			panic(err)
		}

		createDb := false

		if err == sql.ErrNoRows {
			createDb = true
		}

		if dbReset {
			if err != sql.ErrNoRows {
				_, err = db.ExecContext(ctx, fmt.Sprintf(`drop database %q`, dbName))
				if err != nil {
					panic(err)
				}
				createDb = true
			}
		}
		if createDb {
			_, err = db.ExecContext(ctx, fmt.Sprintf(`create database %q`, dbName))
			if err != nil {
				panic(err)
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
			panic(err)
		}
		defer conn.Close()
		conn.Raw(func(driverConn any) error {
			pq.SetNoticeHandler(driverConn.(driver.Conn), reporting)
			return nil
		})

		orbSource := path.Join(orbs[i], "src")
		if orbs[i] == "." {
			orbSource = ""
		}
		rows, err := conn.QueryContext(ctx,
			`select migration_filename, migration_statement, execution_error from omni_schema.assemble_schema($1, omni_vfs.local_fs('/mnt/host'), $2) where execution_error is not null`,
			fmt.Sprintf("dbname=%s user=omnigres", dbName), orbSource)

		if err != nil {
			panic(err)
		}

		for rows.Next() {
			var migration_filename, migration_statement, execution_error sql.NullString
			err = rows.Scan(&migration_filename, &migration_statement, &execution_error)
			if err != nil {
				panic(err)
			}
		}

	}
	return
}

var dbReset bool
var databaseName string

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().StringVarP(&databaseName, "db", "d", "", "database name")
	migrateCmd.Flags().BoolVarP(&dbReset, "dbReset", "r", false, "dbReset")
}
