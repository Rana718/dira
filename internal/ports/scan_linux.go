
package ports

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

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
	return parseSS(string(out), proto, allSockets), nil
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
		if e, ok := parseSSLine(line, proto, true); ok {
			entries = append(entries, e)
		}
	}
	sortEntries(entries)
	return entries, nil
}

func parseSS(out, proto string, allSockets bool) []Entry {
	var entries []Entry
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Netid") {
			continue
		}
		if e, ok := parseSSLine(line, proto, allSockets); ok {
			entries = append(entries, e)
		}
	}
	sortEntries(entries)
	return entries
}

func parseSSLine(line, proto string, allSockets bool) (Entry, bool) {
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
	e := Entry{
		Proto: proto, Addr: addr, Port: port,
		Process: process, PID: pid, State: state,
		Public: public,
	}
	annotate(&e)
	return e, true
}
