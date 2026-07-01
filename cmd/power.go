package cmd

import (
	"fmt"

	"github.com/Rana718/dira/internal/power"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var powerCmd = &cobra.Command{
	Use:   "power",
	Short: "Manage power profile (CPU/GPU limits, TDP)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := power.NewTUIModel()
		result, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
		if err != nil {
			return err
		}
		pm := result.(power.TUIModel)
		if pm.Chosen == nil {
			return nil
		}
		p := *pm.Chosen
		if !power.IsBuiltin(p.Name) {
			if err := power.SaveCustomProfile(p); err != nil {
				return err
			}
		}
		fmt.Printf("Applying profile: %s\n", p.Name)
		return power.Apply(p)
	},
}
