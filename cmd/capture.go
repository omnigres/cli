package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/charmbracelet/log"
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

	conn, err := db.Conn(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	err = setupCloudevents(ctx, conn)
	if err != nil {
		log.Error(err)
		return err
	}

	var revision string
	err = conn.QueryRowContext(
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
		log.Error("Could not connect to orb. Ensure the docker container is running, perhaps 'omnigres start' will fix it.")
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

	handler := cloudeventHandler{
		Callback: func(e *cloudevents.Event) {
			switch e.Type() {
			case "org.omnigres.omni_schema.progress_report.v1":
				message := string(e.Data())
				err := json.Unmarshal(e.Data(), &message)
				if err != nil {
					log.Errorf("Error parsing progress report %s", string(e.Data()))
					return
				}

				style := lipgloss.NewStyle().
					SetString("⏳ " + message).
					PaddingLeft(2).
					Width(120).
					Foreground(lipgloss.Color("201"))
				fmt.Print(style.Render() + "\r")
			default:
			}
		},
	}
	cloudeventHandlers = append(cloudeventHandlers, handler)
}
