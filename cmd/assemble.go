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
	"strings"

	"github.com/charmbracelet/log"
	"github.com/lib/pq"
	"github.com/omnigres/cli/orb"
	"github.com/spf13/cobra"
)

var assembleCmd = &cobra.Command{
	Use:   "assemble",
	Short: "Assemble or reassemble orb",
	Long:  `By default, will assemble all listed orbs`,
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
		assembleOrbs(
			ctx,
			cluster,
			dbReset,
			orbs,
			func(orbName string) string { return orbName },
		)
	},
}

func assembleOrbs(
	ctx context.Context,
	cluster orb.OrbCluster,
	dbReset bool,
	orbs []string,
	databaseForOrb func(string) string,
) (err error) {
	var db *sql.DB
	db, err = cluster.Connect(ctx, "omnigres")
	if err != nil {
		return
	}
	for _, orbName := range orbs {
		log.Infof("Assembling orb %s", orbName)
		dbName := databaseForOrb(orbName)
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

		orbSource := path.Join(orbName, "src")
		assembleSchema(ctx, db, orbSource, dbName)
	}
	return
}

func assembleSchema(ctx context.Context, db *sql.DB, orbSource string, dbName string) {
	logger := log.New(os.Stdout)
	logger.SetReportTimestamp(true)

	logger.SetPrefix(fmt.Sprintf("[%s] ", dbName))

	levels := make(map[string]log.Level)
	levels["info"] = log.InfoLevel
	levels["error"] = log.ErrorLevel
	jsonMessageReporting := func(notice *pq.Error) {
		var message map[string]interface{}
		err := json.NewDecoder(strings.NewReader(notice.Message)).Decode(&message)
		if err != nil {
			logger.Errorf("Message is not valid JSON: %s", notice.Message)
			return
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
		pq.SetNoticeHandler(driverConn.(driver.Conn), jsonMessageReporting)
		return nil
	})

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
			log.Error(err)
			return
		}
	}
}

var dbReset bool

func init() {
	rootCmd.AddCommand(assembleCmd)
	assembleCmd.Flags().BoolVarP(&dbReset, "dbReset", "r", false, "dbReset")
}
