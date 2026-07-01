package cmd

import (
	"fmt"
	"os"

	"github.com/Rana718/dira/internal/service"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage systemd services",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := service.NewModel()
		if _, err := tea.NewProgram(m,
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		).Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return nil
	},
}
