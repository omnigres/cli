package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"os"
	"path"
	"time"

	"github.com/charmbracelet/log"
	"github.com/omnigres/cli/orb"
	"github.com/relvacode/iso8601"
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

		err = setupCloudevents(ctx, conn)
		if err != nil {
			return err
		}

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

type testPassed struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	StartTime   iso8601.Time `json:"start_time"`
	EndTime     iso8601.Time `json:"end_time"`
}

type testFailed struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	StartTime   iso8601.Time `json:"start_time"`
	EndTime     iso8601.Time `json:"end_time"`
	Error       string       `json:"error"`
}

func init() {
	rootCmd.AddCommand(testCmd)

	handler := cloudeventHandler{
		Callback: func(e *cloudevents.Event) {
			switch e.Type() {
			case "org.omnigres.omni_test.test.passed.v1":
				var msg testPassed
				err := json.Unmarshal(e.Data(), &msg)
				if err == nil {
					log.Infof("✅ - %s (%s) [%s]", msg.Name, msg.Description, msg.EndTime.Sub(msg.StartTime.Time).String())
				} else {
					log.Error(err)
				}
			case "org.omnigres.omni_test.test.failed.v1":
				var msg testFailed
				err := json.Unmarshal(e.Data(), &msg)
				if err == nil {
					log.Infof("🔴 - %s (%s) [%s]: %s", msg.Name, msg.Description, msg.EndTime.Sub(msg.StartTime.Time).String(), msg.Error)
				} else {
					log.Error(err)
				}
			default:
			}
		},
	}
	cloudeventHandlers = append(cloudeventHandlers, handler)
}
