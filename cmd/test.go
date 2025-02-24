package cmd

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/lib/pq"
	"github.com/omnigres/cli/orb"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test orbs",
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
		log.Debug("Assembling orbs src", "orbs", orbs)

		t := time.Now().Format("20060102150405")
		nameForTestDatabase := func(orbName string) string {
			return fmt.Sprintf("%s_%s_%s", orbName, "test", t)
		}

		err = assembleOrbs(ctx, cluster, true, orbs, nameForTestDatabase)
		if err != nil {
			log.Fatal(err)
		}

		err = testOrbs(ctx, cluster, orbs, nameForTestDatabase)

		if err != nil {
			log.Fatal(err)
		}
	},
}

func testOrbs(
	ctx context.Context,
	cluster orb.OrbCluster,
	orbs []string,
	databaseForOrb func(string) string,
) (err error) {

	var testTarget, testRunner *sql.DB
	testRunner, err = cluster.Connect(ctx, "omnigres")
	if err != nil {
		return
	}

	testOrb := func(orbName string) error {
		dbName := databaseForOrb(orbName)
		testTarget, err = cluster.Connect(ctx, dbName)
		if err != nil {
			return err
		}
		log.Debug("Testing orb", "orbName", orbName, "dbName", dbName)

		_, err = testRunner.ExecContext(
			ctx,
			"update pg_database set datistemplate = true where datname = $1",
			dbName,
		)
		if err != nil {
			return err
		}
		removeIsTemplate := func() {
			// remove istemplate so we can drop the database
			_, err = testRunner.ExecContext(
				ctx,
				"update pg_database set datistemplate = false where datname = $1",
				dbName,
			)
			if err != nil {
				log.Fatal(err)
			}
		}
		defer removeIsTemplate()

		_, err = testTarget.ExecContext(ctx, "create extension omni_test cascade")
		if err != nil {
			return err
		}
		testTarget.Close()

		conn, err := testRunner.Conn(ctx)
		if err != nil {
			return err
		}
		defer conn.Close()

		// assemble tests in target db
		orbSource := path.Join(orbName, "test")
		assembleSchema(ctx, testRunner, orbSource, dbName)

		// run tests
		log.Infof("")
		log.Infof("=== Running tests for %s ===", orbName)

		testResultReporting := func(notice *pq.Error) {
			messages := strings.Split(notice.Message, " – ")
			if messages[0] == "ok" && len(messages) >= 2 {
				log.Infof("✅ - %s", messages[1])
			} else if messages[0] == "failure" && len(messages) >= 3 {
				log.Errorf("🔴 - %s (%s)", messages[1], messages[2])
			}
		}
		conn.Raw(func(driverConn any) error {
			pq.SetNoticeHandler(driverConn.(driver.Conn), testResultReporting)
			return nil
		})

		testRows, err := conn.QueryContext(
			ctx,
			`select name, description, error_message from omni_test.run_tests($1)`,
			dbName,
		)
		if err != nil {
			return err
		}
		defer testRows.Close()

		for testRows.Next() {
			var name, description, error_message sql.NullString
			err = testRows.Scan(&name, &description, &error_message)
			if err != nil {
				return err
			}
		}
		log.Info("===================================================================")

		return nil
	}

	for _, orbName := range orbs {
		err := testOrb(orbName)
		if err != nil {
			log.Error(err)
		}
	}
	return
}

func init() {
	rootCmd.AddCommand(testCmd)
}
