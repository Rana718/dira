package cmd

import (
	"github.com/Rana718/dira/internal/clean"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Run all cleanup tasks in parallel (caches, logs, trash, docker…)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		clean.RunAll()
		return nil
	},
}
