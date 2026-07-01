package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Rana718/dira/internal/ports"
	"github.com/charmbracelet/lipgloss"
	"github.com/Rana718/dira/internal/helper"
	"github.com/spf13/cobra"
)

var (
	portsPublicOnly bool
	portsLocalOnly  bool
	portsUDPOnly    bool
	portsTCPOnly    bool
	portsAll        bool
)

var (
	phStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	ppStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	plStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	procStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	unknStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	udpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Show listening ports and their processes",
	Example: `  dira ports        # TCP + UDP listeners
  dira ports -a     # all sockets
  dira ports -p     # public only
  dira ports -l     # local only`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var entries []ports.Entry
		var err error
		if portsAll {
			entries, err = ports.ScanAll()
			if err != nil {
				return err
			}
		} else {
			if !portsUDPOnly {
				tcp, e := ports.Scan("tcp", false)
				if e != nil {
					return e
				}
				entries = append(entries, tcp...)
			}
			if !portsTCPOnly {
				udp, e := ports.Scan("udp", false)
				if e != nil {
					return e
				}
				entries = append(entries, udp...)
			}
		}

		var filtered []ports.Entry
		for _, e := range entries {
			if portsPublicOnly && !e.Public {
				continue
			}
			if portsLocalOnly && e.Public {
				continue
			}
			filtered = append(filtered, e)
		}
		if len(filtered) == 0 {
			fmt.Println("No matching ports found.")
			return nil
		}

		maxPort, maxAddr, maxProc, maxSvc, maxState := 4, 7, 7, 7, 6
		for _, e := range filtered {
			if l := len(strconv.Itoa(e.Port)); l > maxPort {
				maxPort = l
			}
			if l := len(e.Addr); l > maxAddr {
				maxAddr = l
			}
			if l := len(e.Process); l > maxProc {
				maxProc = l
			}
			if l := len(e.Service); l > maxSvc {
				maxSvc = l
			}
			if l := len(e.State); l > maxState {
				maxState = l
			}
		}

		stateHdr := ""
		if portsAll {
			stateHdr = fmt.Sprintf("  %-*s", maxState, "STATE")
		}
		fmt.Println(phStyle.Render(fmt.Sprintf("  %-5s  %-*s  %-*s%s  %-*s  %s",
			"PROTO", maxPort, "PORT", maxAddr, "ADDRESS", stateHdr, maxProc, "PROCESS", "SERVICE / PID")))
		fmt.Println(phStyle.Render("  " + strings.Repeat("─", 5+maxPort+maxAddr+maxProc+maxSvc+20)))

		var public, local []ports.Entry
		for _, e := range filtered {
			if e.Public {
				public = append(public, e)
			} else {
				local = append(local, e)
			}
		}

		printRows := func(rows []ports.Entry, isPublic bool) {
			for _, e := range rows {
				stateStr := ""
				if portsAll {
					stateStr = "  " + helper.Pad(e.State, maxState)
				}
				svcPid := e.Service
				if e.PID != "" {
					if svcPid != "" {
						svcPid += "  "
					}
					svcPid += "pid " + e.PID
				}
				var base lipgloss.Style
				switch {
				case e.Proto == "udp":
					base = udpStyle
				case isPublic:
					base = ppStyle
				default:
					base = plStyle
				}
				pStr := procStyle.Render(helper.Pad(e.Process, maxProc))
				if e.Process == "unknown" {
					pStr = unknStyle.Render(helper.Pad(e.Process, maxProc))
				}
				svStr := plStyle.Render(svcPid)
				if e.Warn {
					svStr = warnStyle.Render(svcPid + "  ⚠ suspicious")
				}
				fmt.Printf("  %s  %s  %s%s  %s  %s\n",
					base.Render(helper.Pad(e.Proto, 5)),
					base.Render(helper.Pad(strconv.Itoa(e.Port), maxPort)),
					base.Render(helper.Pad(e.Addr, maxAddr)),
					stateStr, pStr, svStr,
				)
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

func init() {
	portsCmd.Flags().BoolVarP(&portsPublicOnly, "public", "p", false, "Show public ports only")
	portsCmd.Flags().BoolVarP(&portsLocalOnly, "local", "l", false, "Show local ports only")
	portsCmd.Flags().BoolVar(&portsTCPOnly, "tcp", false, "TCP only")
	portsCmd.Flags().BoolVar(&portsUDPOnly, "udp", false, "UDP only")
	portsCmd.Flags().BoolVarP(&portsAll, "all", "a", false, "All sockets (not just listening)")
}
