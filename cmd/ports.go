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

var knownPorts = map[int]string{
	21: "FTP", 22: "SSH", 23: "Telnet ⚠", 25: "SMTP", 53: "DNS",
	80: "HTTP", 110: "POP3", 111: "RPC ⚠", 135: "MSRPC ⚠",
	137: "NetBIOS ⚠", 138: "NetBIOS ⚠", 139: "NetBIOS ⚠", 143: "IMAP",
	443: "HTTPS", 445: "SMB ⚠", 3306: "MySQL", 3389: "RDP ⚠",
	4444: "Metasploit ⚠⚠", 5353: "mDNS", 5355: "LLMNR", 5432: "PostgreSQL",
	5900: "VNC ⚠", 6379: "Redis", 6443: "Kubernetes API",
	8080: "HTTP alt", 8443: "HTTPS alt", 9200: "Elasticsearch",
	27017: "MongoDB", 41641: "Tailscale",
}

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
	state   string
	public  bool
	service string
	warn    bool
}

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Show listening ports and their processes",
	Example: `  dira ports          # listening (TCP + UDP)
  dira ports -a       # all sockets including connected
  dira ports -p       # public only
  dira ports -l       # local only
  dira ports --tcp    # TCP only
  dira ports --udp    # UDP only`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var entries []portEntry
		var err error

		if portsAll {
			entries, err = scanAll()
		} else {
			if !portsUDPOnly {
				tcp, e := scanSockets("tcp", false)
				if e != nil {
					return e
				}
				entries = append(entries, tcp...)
			}
			if !portsTCPOnly {
				udp, e := scanSockets("udp", false)
				if e != nil {
					return e
				}
				entries = append(entries, udp...)
			}
		}
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

		sort.Slice(filtered, func(i, j int) bool {
			if filtered[i].public != filtered[j].public {
				return filtered[i].public
			}
			return filtered[i].port < filtered[j].port
		})

		maxPort, maxAddr, maxProc, maxSvc, maxState := 4, 7, 7, 7, 6
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
			if l := len(e.state); l > maxState {
				maxState = l
			}
		}

		stateCol := ""
		if portsAll {
			stateCol = fmt.Sprintf("  %-*s", maxState, "STATE")
		}
		fmt.Println(phStyle.Render(fmt.Sprintf("  %-5s  %-*s  %-*s%s  %-*s  %s",
			"PROTO", maxPort, "PORT", maxAddr, "ADDRESS", stateCol, maxProc, "PROCESS", "SERVICE / PID")))
		fmt.Println(phStyle.Render("  " + strings.Repeat("─", 5+maxPort+maxAddr+maxProc+maxSvc+20)))

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

				stateStr := ""
				if portsAll {
					stateStr = "  " + pad(e.state, maxState)
				}

				svcPid := e.service
				if e.pid != "" {
					if svcPid != "" {
						svcPid += "  "
					}
					svcPid += "pid " + e.pid
				}

				var baseStyle lipgloss.Style
				if e.proto == "udp" {
					baseStyle = udpStyle
				} else if isPublic {
					baseStyle = ppStyle
				} else {
					baseStyle = plStyle
				}

				pStr := procStyle.Render(procStr)
				if e.process == "unknown" {
					pStr = unknStyle.Render(procStr)
				}
				svStr := plStyle.Render(svcPid)
				if e.warn && isPublic {
					svStr = warnStyle.Render(svcPid + "  ⚠ suspicious")
				}

				fmt.Printf("  %s  %s  %s%s  %s  %s\n",
					baseStyle.Render(protoStr),
					baseStyle.Render(portStr),
					baseStyle.Render(addrStr),
					stateStr,
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

func parseSocket(line, proto string, allSockets bool) (portEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return portEntry{}, false
	}
	state := fields[0]
	if !allSockets && state != "LISTEN" && state != "UNCONN" {
		return portEntry{}, false
	}
	if allSockets && (state == "nl" || state == "Netid") {
		return portEntry{}, false
	}

	addrField := fields[3]
	addrField = strings.TrimPrefix(addrField, "[")
	lastColon := strings.LastIndex(addrField, ":")
	if lastColon < 0 {
		return portEntry{}, false
	}
	addr := strings.TrimSuffix(strings.TrimSuffix(addrField[:lastColon], "]"), "%lo")
	port, err := strconv.Atoi(addrField[lastColon+1:])
	if err != nil {
		return portEntry{}, false
	}

	process, pid := "unknown", ""
	for _, f := range fields[5:] {
		if m := reProcess.FindStringSubmatch(f); m != nil {
			process = m[1]
			pid = m[2]
			break
		}
	}

	public := addr == "0.0.0.0" || addr == "::" || addr == "*"
	svc := knownPorts[port]
	warn := suspiciousPorts[port] && public

	// normalize proto from ss output
	if strings.HasPrefix(fields[0], "tcp") || proto == "tcp" {
		proto = "tcp"
	}

	return portEntry{
		proto: proto, addr: addr, port: port,
		process: process, pid: pid, state: state,
		public: public, service: svc, warn: warn,
	}, true
}

func scanSockets(proto string, allSockets bool) ([]portEntry, error) {
	flag := "-tlnp"
	if proto == "udp" {
		flag = "-ulnp"
	}
	if allSockets {
		flag = strings.Replace(flag, "l", "", 1)
	}

	cmd := exec.Command("sudo", "ss", flag)
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
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
		if strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Netid") {
			continue
		}
		if e, ok := parseSocket(line, proto, allSockets); ok {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func scanAll() ([]portEntry, error) {
	cmd := exec.Command("sudo", "ss", "-anp")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		out, err = exec.Command("ss", "-anp").Output()
		if err != nil {
			return nil, fmt.Errorf("ss not available: %w", err)
		}
	}

	var entries []portEntry
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Netid") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		proto := fields[0]
		if proto == "nl" || proto == "u_str" || proto == "u_dgr" || proto == "u_seq" || proto == "p_raw" || proto == "p_dgr" {
			continue
		}
		if e, ok := parseSocket(line, proto, true); ok {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func init() {
	portsCmd.Flags().BoolVarP(&portsPublicOnly, "public", "p", false, "Show public ports only")
	portsCmd.Flags().BoolVarP(&portsLocalOnly, "local", "l", false, "Show local ports only")
	portsCmd.Flags().BoolVar(&portsTCPOnly, "tcp", false, "Show TCP only")
	portsCmd.Flags().BoolVar(&portsUDPOnly, "udp", false, "Show UDP only")
	portsCmd.Flags().BoolVarP(&portsAll, "all", "a", false, "Show all sockets (not just listening)")
}
