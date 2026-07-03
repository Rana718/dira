package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Rana718/dira/internal/container"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func isRuntimeAvailable(rt string) bool {
	_, err := exec.LookPath(rt)
	return err == nil
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Quick-run a container (Redis, Postgres, etc.) with one command",
	Long: `Launch common services instantly without remembering docker run flags.

Pick from presets (Redis, PostgreSQL, MySQL, MongoDB, Nginx, RabbitMQ, etc.)
or search for any image from the registry — dira handles ports, env vars, and tags.

Supports both Docker and Podman.`,
	Example: `  dira run              # interactive: pick preset or search
  dira run redis        # instant: launch Redis with defaults
  dira run postgres     # instant: launch PostgreSQL with defaults
  dira run mongo        # instant: launch MongoDB with defaults`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// If an argument is given, try to match a preset and run instantly
		if len(args) == 1 {
			return runInstant(args[0])
		}

		// Otherwise, launch interactive TUI
		m := container.NewRunModel()
		result, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		_ = result
		return nil
	},
}

// runInstant tries to match the argument to a preset and launches it immediately.
func runInstant(name string) error {
	// detect runtime
	runtime := ""
	for _, rt := range []string{"docker", "podman"} {
		if isRuntimeAvailable(rt) {
			runtime = rt
			break
		}
	}
	if runtime == "" {
		return fmt.Errorf("neither docker nor podman found")
	}

	// find matching preset
	presets := container.LoadPresets()
	var matched *container.Preset
	for i, p := range presets {
		if p.Image == "" {
			continue
		}
		label := strings.ToLower(p.Label)
		img := strings.ToLower(p.Image)
		query := strings.ToLower(name)
		if strings.Contains(label, query) || strings.Contains(img, query) {
			matched = &presets[i]
			break
		}
	}

	if matched == nil {
		// not a preset — treat as image name, run with defaults
		matched = &container.Preset{
			Label: name,
			Image: name,
		}
		// add :latest if no tag
		if !strings.Contains(matched.Image, ":") {
			matched.Image += ":latest"
		}
	}

	fmt.Printf("Launching %s (%s)...\n", matched.Label, matched.Image)

	// fix podman short-name issue
	runPreset := *matched
	if runtime == "podman" {
		runPreset.Image = container.QualifyImage(runPreset.Image)
	}

	fmt.Printf("  %s\n", container.BuildRunCommand(runtime, runPreset, true))

	output, err := container.RunContainer(runtime, runPreset, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		if output != "" {
			fmt.Fprintf(os.Stderr, "%s\n", output)
		}
		return err
	}

	id := output
	if len(id) > 12 {
		id = id[:12]
	}
	fmt.Printf("✓ Started: %s\n", id)
	if len(runPreset.Ports) > 0 {
		fmt.Printf("  Ports: %s\n", strings.Join(runPreset.Ports, ", "))
	}
	if pw, ok := runPreset.Env["POSTGRES_PASSWORD"]; ok {
		fmt.Printf("  Password: %s\n", pw)
	} else if pw, ok := runPreset.Env["MYSQL_ROOT_PASSWORD"]; ok {
		fmt.Printf("  Root password: %s\n", pw)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(runCmd)
}
