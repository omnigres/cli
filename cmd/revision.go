package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

var revisionCmd = &cobra.Command{
	Use:   "revision",
	Short: "Revision management",
}

func init() {
	rootCmd.AddCommand(revisionCmd)
	revisionCmd.AddCommand(captureCmd)

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
