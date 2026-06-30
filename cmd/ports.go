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
	portsUDPOnly    bool
	portsTCPOnly    bool
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

// well-known ports with descriptions
var knownPorts = map[int]string{
	21:    "FTP",
	22:    "SSH",
	23:    "Telnet ⚠",
	25:    "SMTP",
	53:    "DNS",
	80:    "HTTP",
	110:   "POP3",
	111:   "RPC ⚠",
	135:   "MSRPC ⚠",
	137:   "NetBIOS ⚠",
	138:   "NetBIOS ⚠",
	139:   "NetBIOS ⚠",
	143:   "IMAP",
	443:   "HTTPS",
	445:   "SMB ⚠",
	3306:  "MySQL",
	3389:  "RDP ⚠",
	4444:  "Metasploit ⚠⚠",
	5353:  "mDNS",
	5355:  "LLMNR",
	5432:  "PostgreSQL",
	5900:  "VNC ⚠",
	6379:  "Redis",
	6443:  "Kubernetes API",
	8080:  "HTTP alt",
	8443:  "HTTPS alt",
	9200:  "Elasticsearch",
	27017: "MongoDB",
	41641: "Tailscale",
}

// ports that are suspicious if exposed publicly
var suspiciousPorts = map[int]bool{
	23: true, 111: true, 135: true, 137: true, 138: true,
	139: true, 445: true, 3389: true, 4444: true, 5900: true,
}

type portEntry struct {
	proto   string
	addr    string
	port    int
	process string
	pid     string
	public  bool
	service string
	warn    bool
}

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Show listening ports and their processes",
	Example: `  dira ports          # all (TCP + UDP)
  dira ports -p       # public only (0.0.0.0 / ::)
  dira ports -l       # local only (127.x)
  dira ports --tcp    # TCP only
  dira ports --udp    # UDP only`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var entries []portEntry

		if !portsUDPOnly {
			tcp, err := scanSockets("tcp")
			if err != nil {
				return err
			}
			entries = append(entries, tcp...)
		}
		if !portsTCPOnly {
			udp, err := scanSockets("udp")
			if err != nil {
				return err
			}
			entries = append(entries, udp...)
		}

		// filter
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

		// sort: public first, then by port
		sort.Slice(filtered, func(i, j int) bool {
			if filtered[i].public != filtered[j].public {
				return filtered[i].public
			}
			return filtered[i].port < filtered[j].port
		})

		// column widths
		maxPort, maxAddr, maxProc, maxSvc := 4, 7, 7, 7
		for _, e := range filtered {
			if l := len(strconv.Itoa(e.port)); l > maxPort {
				maxPort = l
			}
			if l := len(e.addr); l > maxAddr {
				maxAddr = l
			}
			if l := len(e.process); l > maxProc {
				maxProc = l
			}
			if l := len(e.service); l > maxSvc {
				maxSvc = l
			}
		}

		// header
		header := fmt.Sprintf("  %-5s  %-5s  %-*s  %-*s  %-*s  %s",
			"PROTO", "PORT", maxPort, "", maxAddr, "ADDRESS", maxProc, "PROCESS", "SERVICE / PID")
		fmt.Println(phStyle.Render(fmt.Sprintf("  %-5s  %-*s  %-*s  %-*s  %s",
			"PROTO", maxPort, "PORT", maxAddr, "ADDRESS", maxProc, "PROCESS", "SERVICE / PID")))
		_ = header
		fmt.Println(phStyle.Render("  " + strings.Repeat("─", 5+maxPort+maxAddr+maxProc+maxSvc+18)))

		var public, local []portEntry
		for _, e := range filtered {
			if e.public {
				public = append(public, e)
			} else {
				local = append(local, e)
			}
		}

		printRows := func(rows []portEntry, isPublic bool) {
			for _, e := range rows {
				protoStr := pad(e.proto, 5)
				portStr  := pad(strconv.Itoa(e.port), maxPort)
				addrStr  := pad(e.addr, maxAddr)
				procStr  := pad(e.process, maxProc)

				svcPid := ""
				if e.service != "" {
					svcPid = e.service
				}
				if e.pid != "" {
					if svcPid != "" {
						svcPid += "  "
					}
					svcPid += "pid " + e.pid
				}

				// color selection
				var protoC, portC, addrC lipgloss.Style
				if e.proto == "udp" {
					protoC, portC, addrC = udpStyle, udpStyle, udpStyle
				} else if isPublic {
					protoC, portC, addrC = ppStyle, ppStyle, ppStyle
				} else {
					protoC, portC, addrC = plStyle, plStyle, plStyle
				}

				pStr := procStyle.Render(procStr)
				if e.process == "unknown" {
					pStr = unknStyle.Render(procStr)
				}

				svStr := plStyle.Render(svcPid)
				if e.warn && isPublic {
					svStr = warnStyle.Render(svcPid + "  ⚠ suspicious public port")
				}

				fmt.Printf("  %s  %s  %s  %s  %s\n",
					protoC.Render(protoStr),
					portC.Render(portStr),
					addrC.Render(addrStr),
					pStr,
					svStr,
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

var reProcess = regexp.MustCompile(`"([^"]+)",pid=(\d+)`)

func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func scanSockets(proto string) ([]portEntry, error) {
	flag := "-tlnp"
	if proto == "udp" {
		flag = "-ulnp"
	}

	sudoCmd := exec.Command("sudo", "ss", flag)
	sudoCmd.Stdin = os.Stdin
	out, err := sudoCmd.Output()
	if err != nil {
		out, err = exec.Command("ss", flag).Output()
		if err != nil {
			return nil, fmt.Errorf("ss not available: %w", err)
		}
	}

	var entries []portEntry
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		state := fields[0]
		// TCP: LISTEN, UDP: UNCONN
		if state != "LISTEN" && state != "UNCONN" {
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
		svc := knownPorts[port]
		warn := suspiciousPorts[port] && public

		entries = append(entries, portEntry{
			proto:   proto,
			addr:    addr,
			port:    port,
			process: process,
			pid:     pid,
			public:  public,
			service: svc,
			warn:    warn,
		})
	}

	return entries, nil
}

func init() {
	portsCmd.Flags().BoolVarP(&portsPublicOnly, "public", "p", false, "Show public ports only")
	portsCmd.Flags().BoolVarP(&portsLocalOnly, "local", "l", false, "Show local ports only")
	portsCmd.Flags().BoolVar(&portsTCPOnly, "tcp", false, "Show TCP only")
	portsCmd.Flags().BoolVar(&portsUDPOnly, "udp", false, "Show UDP only")
}
