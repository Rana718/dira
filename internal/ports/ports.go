package ports

import (
	"sort"
)

// Entry represents a single listening socket or connection.
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

// KnownPorts maps well-known port numbers to service names.
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

// SuspiciousPorts are ports that deserve a warning when publicly exposed.
var SuspiciousPorts = map[int]bool{
	23: true, 111: true, 135: true, 137: true, 138: true,
	139: true, 445: true, 3389: true, 4444: true, 5900: true,
}

// sortEntries sorts entries: public first, then by port number.
func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Public != entries[j].Public {
			return entries[i].Public
		}
		return entries[i].Port < entries[j].Port
	})
}

// annotate fills in Service and Warn fields based on KnownPorts/SuspiciousPorts.
func annotate(e *Entry) {
	e.Service = KnownPorts[e.Port]
	e.Warn = SuspiciousPorts[e.Port] && e.Public
}
