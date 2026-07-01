package cmd

import (
	"fmt"
	"os"

	"github.com/Rana718/dira/internal/container"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var containerCmd = &cobra.Command{
	Use:   "container",
	Short: "Manage Docker and Podman containers",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := container.NewModel()
		if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return nil
	},
}
