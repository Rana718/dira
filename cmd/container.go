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
	Short: "Manage containers, images, volumes, and networks",
	Long: `Interactive TUI for full container lifecycle management.

Tabs:  1 Containers  2 Images  3 Volumes  4 Networks
Use Tab / Shift+Tab or 1-4 keys to switch tabs.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := container.NewModel()
		if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return nil
	},
}

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Manage container images (opens Images tab)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := container.NewImagesModel()
		if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return nil
	},
}

var volumesCmd = &cobra.Command{
	Use:   "volumes",
	Short: "Manage container volumes (opens Volumes tab)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := container.NewVolumesModel()
		if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return nil
	},
}

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Manage container networks (opens Networks tab)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := container.NewNetworksModel()
		if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return nil
	},
}
