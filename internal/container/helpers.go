package container

import (
	"strings"

	"github.com/Rana718/dira/internal/helper"
	"github.com/charmbracelet/bubbles/viewport"
)

func parsePorts(raw string, wPorts int) []string {
	if raw == "" {
		return []string{"—"}
	}
	seen := map[string]bool{}
	var unique []string
	for _, part := range strings.Split(raw, ", ") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "[::]:") {
			ipv4 := strings.Replace(part, "[::]:", "0.0.0.0:", 1)
			if seen[ipv4] {
				continue
			}
		}
		mapped := formatPort(part)
		if !seen[mapped] {
			seen[mapped] = true
			unique = append(unique, mapped)
		}
	}
	if len(unique) == 0 {
		return []string{"—"}
	}
	var lines []string
	line := ""
	for _, p := range unique {
		if line == "" {
			line = p
		} else if len(line)+2+len(p) <= wPorts {
			line += " " + p
		} else {
			lines = append(lines, helper.PadR(line, wPorts))
			line = p
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func formatPort(s string) string {
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		s = s[idx+1:]
	}
	return strings.Replace(s, "->", "→", 1)
}

func scrollPct(vp viewport.Model) int {
	total := vp.TotalLineCount()
	if total == 0 || total <= vp.Height {
		return 100
	}
	return int(float64(vp.YOffset) / float64(total-vp.Height) * 100)
}
