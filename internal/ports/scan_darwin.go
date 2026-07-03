//go:build darwin

package ports

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// lsof output pattern: COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME
// NAME field for TCP: host:port (LISTEN) or host:port->remote:port (ESTABLISHED)
var reLsofAddr = regexp.MustCompile(`^(.+):(\d+)$`)

// Scan uses lsof to find listening sockets on macOS.
func Scan(proto string, allSockets bool) ([]Entry, error) {
	protoFlag := "TCP"
	if proto == "udp" {
		protoFlag = "UDP"
	}

	args := []string{"-i", protoFlag, "-n", "-P"}
	if !allSockets {
		args = append(args, "-sTCP:LISTEN")
		if proto == "udp" {
			// UDP has no LISTEN state, just show all UDP
			args = []string{"-i", "UDP", "-n", "-P"}
		}
	}

	out, err := exec.Command("lsof", args...).Output()
	if err != nil {
		// try with sudo
		out, err = exec.Command("sudo", append([]string{"lsof"}, args...)...).Output()
		if err != nil {
			return nil, fmt.Errorf("lsof not available: %w", err)
		}
	}

	entries := parseLsof(string(out), proto, allSockets)
	sortEntries(entries)
	return entries, nil
}

// ScanAll returns all TCP + UDP sockets on macOS.
func ScanAll() ([]Entry, error) {
	args := []string{"-i", "-n", "-P"}
	out, err := exec.Command("lsof", args...).Output()
	if err != nil {
		out, err = exec.Command("sudo", append([]string{"lsof"}, args...)...).Output()
		if err != nil {
			return nil, fmt.Errorf("lsof not available: %w", err)
		}
	}

	var entries []Entry
	entries = append(entries, parseLsof(string(out), "", true)...)
	sortEntries(entries)
	return entries, nil
}

func parseLsof(out, defaultProto string, allSockets bool) []Entry {
	var entries []Entry
	seen := map[string]bool{}

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		// skip header
		if strings.HasPrefix(line, "COMMAND") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		command := fields[0]
		pid := fields[1]
		proto := strings.ToLower(fields[7]) // NODE field: TCP or UDP
		if proto != "tcp" && proto != "udp" {
			// try field 8 for some lsof versions
			if len(fields) > 8 {
				lower := strings.ToLower(fields[8])
				if lower == "tcp" || lower == "udp" {
					proto = lower
				} else {
					continue
				}
			} else {
				continue
			}
		}

		nameField := fields[len(fields)-1]
		// handle state in parentheses: "host:port (LISTEN)"
		state := ""
		if pIdx := strings.LastIndex(line, "("); pIdx > 0 {
			stateRaw := line[pIdx+1:]
			stateRaw = strings.TrimSuffix(stateRaw, ")")
			state = strings.TrimSpace(stateRaw)
		}

		if !allSockets && state != "LISTEN" && proto != "udp" {
			continue
		}

		// parse address:port from name field
		// strip ->remote for established connections
		local := nameField
		if arrow := strings.Index(local, "->"); arrow > 0 {
			local = local[:arrow]
		}
		// remove (STATE) suffix
		if paren := strings.Index(local, "("); paren > 0 {
			local = local[:paren]
		}
		local = strings.TrimSpace(local)

		m := reLsofAddr.FindStringSubmatch(local)
		if m == nil {
			// try parsing IPv6 like [::1]:port or *:port
			lastColon := strings.LastIndex(local, ":")
			if lastColon < 0 {
				continue
			}
			addr := local[:lastColon]
			portStr := local[lastColon+1:]
			port, err := strconv.Atoi(portStr)
			if err != nil {
				continue
			}
			addr = strings.Trim(addr, "[]")
			e := buildEntry(proto, addr, port, command, pid, state)
			key := fmt.Sprintf("%s:%s:%d", proto, e.Addr, port)
			if !seen[key] {
				seen[key] = true
				entries = append(entries, e)
			}
			continue
		}

		addr := m[1]
		port, _ := strconv.Atoi(m[2])
		addr = strings.Trim(addr, "[]")

		e := buildEntry(proto, addr, port, command, pid, state)
		key := fmt.Sprintf("%s:%s:%d", proto, e.Addr, port)
		if !seen[key] {
			seen[key] = true
			entries = append(entries, e)
		}
	}

	return entries
}

func buildEntry(proto, addr string, port int, process, pid, state string) Entry {
	public := addr == "*" || addr == "0.0.0.0" || addr == "::" || addr == ""
	if state == "" {
		if proto == "udp" {
			state = "UNCONN"
		} else {
			state = "LISTEN"
		}
	}
	e := Entry{
		Proto:   proto,
		Addr:    addr,
		Port:    port,
		Process: process,
		PID:     pid,
		State:   state,
		Public:  public,
	}
	annotate(&e)
	return e
}
