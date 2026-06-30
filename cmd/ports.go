package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	portsPublicOnly bool
	portsLocalOnly  bool
)

var (
	phStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	ppStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	plStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	procStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	unknStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
)

type portEntry struct {
	proto   string
	addr    string
	port    int
	process string
	pid     string
	public  bool
}

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Show listening ports and their processes",
	Example: `  dira ports        # all
  dira ports -p     # public only (0.0.0.0 / ::)
  dira ports -l     # local only (127.x)`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := scanPorts()
		if err != nil {
			return err
		}

		var filtered []portEntry
		for _, e := range entries {
			if portsPublicOnly && !e.public {
				continue
			}
			if portsLocalOnly && e.public {
				continue
			}
			filtered = append(filtered, e)
		}

		if len(filtered) == 0 {
			fmt.Println("No matching ports found.")
			return nil
		}

		// column widths
		maxPort, maxAddr, maxProc, maxPid := 4, 7, 7, 5
		for _, e := range filtered {
			if len(strconv.Itoa(e.port)) > maxPort {
				maxPort = len(strconv.Itoa(e.port))
			}
			if len(e.addr) > maxAddr {
				maxAddr = len(e.addr)
			}
			if len(e.process) > maxProc {
				maxProc = len(e.process)
			}
			if len(e.pid) > maxPid {
				maxPid = len(e.pid)
			}
		}

		// header
		hFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%s", 5, maxPort, maxAddr, maxProc)
		divider := "  " + strings.Repeat("─", 5+maxPort+maxAddr+maxProc+maxPid+14)
		fmt.Println(phStyle.Render(fmt.Sprintf(hFmt, "PROTO", "PORT", "ADDRESS", "PROCESS", "PID")))
		fmt.Println(phStyle.Render(divider))

		var public, local []portEntry
		for _, e := range filtered {
			if e.public {
				public = append(public, e)
			} else {
				local = append(local, e)
			}
		}

		printRows := func(rows []portEntry, public bool) {
			for _, e := range rows {
				proto   := pad(e.proto, 5)
				port    := pad(strconv.Itoa(e.port), maxPort)
				addr    := pad(e.addr, maxAddr)
				process := pad(e.process, maxProc)
				pid     := e.pid
				if pid == "" {
					pid = "—"
				}

				if public {
					fmt.Printf("  %s  %s  %s  %s  %s\n",
						ppStyle.Render(proto),
						ppStyle.Render(port),
						ppStyle.Render(addr),
						procStyle.Render(process),
						plStyle.Render(pid),
					)
				} else {
					pStr := process
					if e.process == "unknown" {
						pStr = unknStyle.Render(process)
					} else {
						pStr = procStyle.Render(process)
					}
					fmt.Printf("  %s  %s  %s  %s  %s\n",
						plStyle.Render(proto),
						plStyle.Render(port),
						plStyle.Render(addr),
						pStr,
						plStyle.Render(pid),
					)
				}
			}
		}

		if len(public) > 0 {
			if !portsLocalOnly {
				fmt.Println(ppStyle.Render("  ● Public (exposed to network)"))
			}
			printRows(public, true)
		}
		if len(local) > 0 {
			if !portsPublicOnly {
				if len(public) > 0 {
					fmt.Println()
				}
				fmt.Println(plStyle.Render("  ● Local only (loopback)"))
			}
			printRows(local, false)
		}

		return nil
	},
}

var reProcess = regexp.MustCompile(`"([^"]+)",pid=(\d+)`)

func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func scanPorts() ([]portEntry, error) {
	// try sudo first for full process info, fall back to non-sudo
	var out []byte
	var err error

	sudoCmd := exec.Command("sudo", "ss", "-tlnp")
	sudoCmd.Stdin = os.Stdin
	out, err = sudoCmd.Output()
	if err != nil {
		out, err = exec.Command("ss", "-tlnp").Output()
		if err != nil {
			return nil, fmt.Errorf("ss not available: %w", err)
		}
	}

	var entries []portEntry
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "LISTEN") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		localAddr := fields[3]
		localAddr = strings.TrimPrefix(localAddr, "[")
		lastColon := strings.LastIndex(localAddr, ":")
		if lastColon < 0 {
			continue
		}
		addr := strings.TrimSuffix(strings.TrimSuffix(localAddr[:lastColon], "]"), "%lo")
		port, err := strconv.Atoi(localAddr[lastColon+1:])
		if err != nil {
			continue
		}

		process, pid := "unknown", ""
		if len(fields) >= 6 {
			if m := reProcess.FindStringSubmatch(fields[5]); m != nil {
				process = m[1]
				pid = m[2]
			}
		}

		public := addr == "0.0.0.0" || addr == "::" || addr == "*"

		entries = append(entries, portEntry{
			proto:   "tcp",
			addr:    addr,
			port:    port,
			process: process,
			pid:     pid,
			public:  public,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].public != entries[j].public {
			return entries[i].public
		}
		return entries[i].port < entries[j].port
	})

	return entries, nil
}

func init() {
	portsCmd.Flags().BoolVarP(&portsPublicOnly, "public", "p", false, "Show public ports only")
	portsCmd.Flags().BoolVarP(&portsLocalOnly, "local", "l", false, "Show local ports only")
}
