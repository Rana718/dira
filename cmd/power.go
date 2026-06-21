package cmd

import (
	"github.com/Rana718/dira/internal/power"

	"github.com/spf13/cobra"
)

var underclockCmd = &cobra.Command{
	Use:   "underclock",
	Short: "Apply underclocking profile (low power, full fans)",
	Args:  cobra.NoArgs,
	RunE:  func(cmd *cobra.Command, args []string) error { return power.Apply() },
}

var underclockResetCmd = &cobra.Command{
	Use:   "underclock-reset",
	Short: "Revert underclocking profile to defaults",
	Args:  cobra.NoArgs,
	RunE:  func(cmd *cobra.Command, args []string) error { return power.Reset() },
}
