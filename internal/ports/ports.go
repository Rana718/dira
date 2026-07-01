package ports

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Entry struct {
	Proto   string
	Addr    string
	Port    int
	Process string
	PID     string
	State   string
	Public  bool
	Service string
	Warn    bool
}

var KnownPorts = map[int]string{
	21: "FTP", 22: "SSH", 23: "Telnet ⚠", 25: "SMTP", 53: "DNS",
	80: "HTTP", 110: "POP3", 111: "RPC ⚠", 135: "MSRPC ⚠",
	137: "NetBIOS ⚠", 138: "NetBIOS ⚠", 139: "NetBIOS ⚠", 143: "IMAP",
	443: "HTTPS", 445: "SMB ⚠", 3306: "MySQL", 3389: "RDP ⚠",
	4444: "Metasploit ⚠⚠", 5353: "mDNS", 5355: "LLMNR", 5432: "PostgreSQL",
	5900: "VNC ⚠", 6379: "Redis", 6443: "Kubernetes API",
	8080: "HTTP alt", 8443: "HTTPS alt", 9200: "Elasticsearch",
	27017: "MongoDB", 41641: "Tailscale",
}

var SuspiciousPorts = map[int]bool{
	23: true, 111: true, 135: true, 137: true, 138: true,
	139: true, 445: true, 3389: true, 4444: true, 5900: true,
}

var reProcess = regexp.MustCompile(`"([^"]+)",pid=(\d+)`)

func Scan(proto string, allSockets bool) ([]Entry, error) {
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
	return parseOutput(string(out), proto, allSockets), nil
}

func ScanAll() ([]Entry, error) {
	cmd := exec.Command("sudo", "ss", "-anp")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		out, err = exec.Command("ss", "-anp").Output()
		if err != nil {
			return nil, fmt.Errorf("ss not available: %w", err)
		}
	}
	var entries []Entry
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		proto := fields[0]
		if proto == "nl" || proto == "u_str" || proto == "u_dgr" ||
			proto == "u_seq" || proto == "p_raw" || proto == "p_dgr" || proto == "Netid" {
			continue
		}
		if e, ok := parseLine(line, proto, true); ok {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Public != entries[j].Public {
			return entries[i].Public
		}
		return entries[i].Port < entries[j].Port
	})
	return entries, nil
}

func parseOutput(out, proto string, allSockets bool) []Entry {
	var entries []Entry
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Netid") {
			continue
		}
		if e, ok := parseLine(line, proto, allSockets); ok {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Public != entries[j].Public {
			return entries[i].Public
		}
		return entries[i].Port < entries[j].Port
	})
	return entries
}

func parseLine(line, proto string, allSockets bool) (Entry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return Entry{}, false
	}
	state := fields[0]
	if !allSockets && state != "LISTEN" && state != "UNCONN" {
		return Entry{}, false
	}

	addrField := strings.TrimPrefix(fields[3], "[")
	lastColon := strings.LastIndex(addrField, ":")
	if lastColon < 0 {
		return Entry{}, false
	}
	addr := strings.TrimSuffix(strings.TrimSuffix(addrField[:lastColon], "]"), "%lo")
	port, err := strconv.Atoi(addrField[lastColon+1:])
	if err != nil {
		return Entry{}, false
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
	return Entry{
		Proto: proto, Addr: addr, Port: port,
		Process: process, PID: pid, State: state,
		Public:  public,
		Service: KnownPorts[port],
		Warn:    SuspiciousPorts[port] && public,
	}, true
}
