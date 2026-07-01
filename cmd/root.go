package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// ── Brand colours 

var (
	pink   = lipgloss.NewStyle().Foreground(lipgloss.Color("#CD2976")).Bold(true)
	cream  = lipgloss.NewStyle().Foreground(lipgloss.Color("#D8D0BF")).Bold(true)
	dim    = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	bold   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D8D0BF")).Bold(true)
	cat    = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
)

var logo = []string{
	`  ██████╗ ██╗██████╗  █████╗ `,
	`  ██╔══██╗██║██╔══██╗██╔══██╗`,
	`  ██║  ██║██║██████╔╝███████║`,
	`  ██║  ██║██║██╔══██╗██╔══██║`,
	`  ██████╔╝██║██║  ██║██║  ██║`,
	`  ╚═════╝ ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝`,
}

func printHome(version string) {
	for _, line := range logo {
		runes := []rune(line)
		mid := len(runes) / 2
		fmt.Print(pink.Render(string(runes[:mid])))
		fmt.Println(cream.Render(string(runes[mid:])))
	}

	fmt.Println()
	fmt.Println(cream.Render(fmt.Sprintf("  v%s", version)) +
		dim.Render("  ·Linux system helper"))
	fmt.Println(dim.Render("  ─────────────────────────────────────────────────"))
	fmt.Println()

	type entry struct{ cmd, desc string }
	groups := []struct {
		title   string
		entries []entry
	}{
		{"ASUS TUF Keyboard", []entry{
			{"keycolor <hex>", "set backlight color"},
			{"keymode", "pick animation mode"},
			{"keyspeed", "pick animation speed"},
			{"keylight", "pick brightness (0–3)"},
			{"keystate", "set LED state per power mode"},
		}},
		{"System", []entry{
			{"power", "manage CPU/GPU power profiles"},
			{"info", "show hardware info  (--cpu --gpu --ssd ...)"},
			{"disk", "disk usage, partitions, SSD health"},
			{"service", "manage systemd services"},
		}},
		{"Network & Dev", []entry{
			{"ports", "show open ports and processes"},
			{"container", "manage Docker / Podman containers"},
		}},
	}

	for _, g := range groups {
		fmt.Println(cat.Render("  " + g.title))
		for _, e := range g.entries {
			fmt.Printf("    %s  %s\n",
				bold.Render(fmt.Sprintf("%-24s", "dira "+e.cmd)),
				dim.Render(e.desc),
			)
		}
		fmt.Println()
	}

	fmt.Println(dim.Render(`  dira <command> --help for details`))
	fmt.Println()
}


type cmdEntry struct{ Name, Short string }

func printColorHelp(cmd *cobra.Command) {
	var cmds []cmdEntry
	for _, c := range cmd.Commands() {
		if !c.Hidden && c.Name() != "completion" {
			cmds = append(cmds, cmdEntry{c.Name(), c.Short})
		}
	}

	type group struct {
		label string
		names map[string]bool
	}
	groups := []group{
		{"Keyboard", map[string]bool{
			"keycolor": true, "keymode": true, "keyspeed": true,
			"keylight": true, "keystate": true,
		}},
		{"System", map[string]bool{
			"power": true, "info": true, "disk": true, "service": true,
		}},
		{"Network & Dev", map[string]bool{
			"ports": true, "container": true,
		}},
		{"Other", nil},
	}

	fmt.Println()
	fmt.Println(pink.Render("  dira") + "  " + dim.Render("— "+cmd.Short))
	fmt.Println()
	fmt.Println(dim.Render("  Usage:"))
	fmt.Println("    " + bold.Render("dira") + " " + pink.Render("[command]") + " " + dim.Render("[flags]"))
	fmt.Println()

	// bucket commands into groups
	buckets := make(map[string][]cmdEntry)
	for _, c := range cmds {
		placed := false
		for _, g := range groups[:len(groups)-1] {
			if g.names[c.Name] {
				buckets[g.label] = append(buckets[g.label], c)
				placed = true
				break
			}
		}
		if !placed {
			buckets["Other"] = append(buckets["Other"], c)
		}
	}

	for _, g := range groups {
		entries := buckets[g.label]
		if len(entries) == 0 {
			continue
		}
		fmt.Println(cat.Render("  " + g.label))
		for _, e := range entries {
			fmt.Printf("    %s  %s\n",
				bold.Render(fmt.Sprintf("%-18s", e.Name)),
				dim.Render(e.Short),
			)
		}
		fmt.Println()
	}

	fmt.Println(dim.Render("  Flags:"))
	fmt.Println("    " + pink.Render("-h, --help") + "     " + dim.Render("help for dira"))
	fmt.Println("    " + pink.Render("-v, --version") + "  " + dim.Render("version for dira"))
	fmt.Println()
	fmt.Println(dim.Render(`  Use "dira [command] --help" for more information.`))
	fmt.Println()
}

// ── Root command 

var appVersion string

var rootCmd = &cobra.Command{
	Use:   "dira",
	Short: "ASUS TUF Linux system helper",
	RunE: func(cmd *cobra.Command, args []string) error {
		printHome(appVersion)
		return nil
	},
}

func SetVersion(v string) {
	appVersion = v
	rootCmd.Version = v
}

func Execute() {
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		printColorHelp(cmd)
	})
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(
		keycolorCmd, keymodeCmd, keyspeedCmd, keylightCmd, keystateCmd,
		powerCmd, infoCmd, portsCmd, containerCmd, diskCmd, serviceCmd,
	)
}