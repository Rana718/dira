package service

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type Entry struct {
	Name        string
	Description string
	Load        string
	Active      string
	Sub         string
	Enabled     string
}

func List() []Entry {
	// list-units: services that have been loaded (running, dead, failed, etc.)
	out, err := exec.Command("sudo", "systemctl", "list-units",
		"--type=service", "--all", "--no-pager", "--no-legend", "--plain").Output()
	if err != nil {
		out, _ = exec.Command("systemctl", "list-units",
			"--type=service", "--all", "--no-pager", "--no-legend", "--plain").Output()
	}

	// list-unit-files: ALL installed services including never-loaded ones
	filesOut, _ := exec.Command("sudo", "systemctl", "list-unit-files",
		"--type=service", "--no-pager", "--no-legend", "--plain").Output()
	if len(filesOut) == 0 {
		filesOut, _ = exec.Command("systemctl", "list-unit-files",
			"--type=service", "--no-pager", "--no-legend", "--plain").Output()
	}

	// build enabled map and track all known service names
	enabledMap := map[string]string{}
	allNames := map[string]bool{}
	for _, line := range strings.Split(string(filesOut), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			enabledMap[fields[0]] = fields[1]
			allNames[fields[0]] = true
		}
	}

	// parse list-units entries
	services := []Entry{}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimLeft(line, "● ")
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		name := fields[0]
		desc := ""
		if len(fields) > 4 {
			desc = strings.Join(fields[4:], " ")
		}
		services = append(services, Entry{
			Name:        name,
			Load:        fields[1],
			Active:      fields[2],
			Sub:         fields[3],
			Description: desc,
			Enabled:     enabledMap[name],
		})
		seen[name] = true
	}

	// add services from unit-files that were never loaded (not in list-units)
	for name := range allNames {
		if !seen[name] {
			services = append(services, Entry{
				Name:    name,
				Load:    "not-loaded",
				Active:  "inactive",
				Sub:     "dead",
				Enabled: enabledMap[name],
			})
		}
	}

	// sort by name
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	return services
}

func Logs(name string, lines int) (string, error) {
	cmd := exec.Command("sudo", "journalctl", "-u", name,
		"-n", fmt.Sprintf("%d", lines), "--no-pager")
	out, err := cmd.CombinedOutput()
	if err != nil || len(out) == 0 {
		// fallback without sudo
		out, err = exec.Command("journalctl", "-u", name,
			"-n", fmt.Sprintf("%d", lines), "--no-pager").CombinedOutput()
	}
	return string(out), err
}

func Info(name string) (string, error) {
	out, err := exec.Command("sudo", "systemctl", "show", name, "--no-pager").Output()
	if err != nil || len(out) == 0 {
		out, err = exec.Command("systemctl", "show", name, "--no-pager").Output()
	}
	return string(out), err
}

func Start(name string) error {
	return exec.Command("sudo", "systemctl", "start", name).Run()
}

func Stop(name string) error {
	return exec.Command("sudo", "systemctl", "stop", name).Run()
}

func Restart(name string) error {
	return exec.Command("sudo", "systemctl", "restart", name).Run()
}

func Enable(name string) error {
	return exec.Command("sudo", "systemctl", "enable", name).Run()
}

func Disable(name string) error {
	return exec.Command("sudo", "systemctl", "disable", name).Run()
}
