package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dira",
	Short: "Linux system helper tool",
	Long:  "dira — a Linux system helper tool.",
}

func SetVersion(v string) { rootCmd.Version = v }

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(keycolorCmd, keymodeCmd, keyspeedCmd, keylightCmd, keystateCmd, powerCmd, infoCmd, portsCmd)
}
