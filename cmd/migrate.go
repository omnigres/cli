package cmd

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/lib/pq"
	"github.com/omnigres/cli/orb"
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
		db, err := cluster.Connect(ctx, "omnigres")
		if err != nil {
			panic(err)
		}
		for _, orb := range cluster.Config().Orbs {
			var datname string
			err := db.QueryRowContext(ctx, `select datname from pg_database where datname = $1`, orb.Name).Scan(&datname)
			if err != nil && err != sql.ErrNoRows {
				panic(err)
			}

			createDb := false

			if err == sql.ErrNoRows {
				createDb = true
			}

			if reset {
				if err != sql.ErrNoRows {
					_, err = db.ExecContext(ctx, fmt.Sprintf(`drop database %q`, orb.Name))
					if err != nil {
						panic(err)
					}
					createDb = true
				}
			}
			if createDb {
				_, err = db.ExecContext(ctx, fmt.Sprintf(`create database %q`, orb.Name))
				if err != nil {
					panic(err)
				}
			}

			logger := log.New(os.Stderr)
			logger.SetReportTimestamp(true)
			logger.SetPrefix(fmt.Sprintf("[%s] ", orb.Name))

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

			rows, err := conn.QueryContext(ctx,
				`select migration_filename, migration_statement, execution_error from omni_schema.assemble_schema($1, omni_vfs.local_fs('/mnt/host'), $2) where execution_error is not null`,
				fmt.Sprintf("dbname=%s user=omnigres", orb.Name), path.Join(orb.Name, "src"))

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
	},
}

var reset bool

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().BoolVarP(&reset, "reset", "r", false, "reset")
}
