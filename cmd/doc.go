// Package cmd implements all dira CLI commands using cobra.
//
// # Layout
//
// Each file = one command (or group of related commands):
//
//   - root.go      → root command, help formatting, subcommand registration
//   - rgb.go       → keycolor, keymode, keyspeed, keylight, keystate
//   - power.go     → power profile TUI
//   - info.go      → hardware info display
//   - disk.go      → disk usage + SSD health
//   - service.go   → systemd service manager TUI
//   - container.go → Docker/Podman container manager TUI
//   - ports.go     → open port scanner
//
// # Adding a new command
//
//  1. Create a new file: cmd/mycommand.go
//  2. Define a var: var myCmd = &cobra.Command{...}
//  3. Register it in root.go's init(): rootCmd.AddCommand(myCmd)
//  4. Done. No interfaces, no factories, no registration magic.
//
// # Conventions
//
//   - Commands that show data → print directly (fmt.Printf with lipgloss styling)
//   - Commands that need interaction → launch bubbletea TUI (tea.NewProgram)
//   - Commands that change system state → use sysWrite() or exec.Command("sudo", ...)
//   - All commands return error (RunE), cobra handles display
//
// # Flags
//
// Register flags in init() at the bottom of each file.
// Use cobra's standard Flags().BoolVar / StringVar patterns.
package cmd
