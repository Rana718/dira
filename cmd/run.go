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
		if len(args) == 1 {
			return runInstant(args[0])
		}

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

func runInstant(name string) error {
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
		matched = &container.Preset{
			Label: name,
			Image: name,
		}
		if !strings.Contains(matched.Image, ":") {
			matched.Image += ":latest"
		}
	}

	fmt.Printf("Launching %s (%s)...\n", matched.Label, matched.Image)

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
