package container

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type Container struct {
	ID         string
	Name       string
	Image      string
	Status     string
	Ports      string
	RunningFor string
	Runtime    string // "docker" or "podman"
	Running    bool
}

type Mount struct {
	Source      string
	Destination string
	Mode        string
}

type Stats struct {
	CPU      string
	MemUsage string
	NetIO    string
	BlockIO  string
}

type InspectInfo struct {
	ID        string
	Name      string
	Image     string
	Status    string
	Created   string
	Ports     map[string]string
	Mounts    []Mount
	Env       []string
	Resources ResourceLimits
}

type ResourceLimits struct {
	MemoryBytes     int64
	MemorySwapBytes int64
	NanoCPUs        int64 // 1 CPU = 1_000_000_000
	CPUShares       int64
	PidsLimit       int64
	CPUQuota        int64
	CPUPeriod       int64
}

func runtimeAvailable(rt string) bool {
	_, err := exec.LookPath(rt)
	return err == nil
}

func List() []Container {
	var out []Container
	for _, rt := range []string{"docker", "podman"} {
		if !runtimeAvailable(rt) {
			continue
		}
		b, err := exec.Command(rt, "ps", "-a",
			"--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}\t{{.RunningFor}}").Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if line == "" {
				continue
			}
			parts := strings.Split(line, "\t")
			for len(parts) < 6 {
				parts = append(parts, "")
			}
			running := strings.HasPrefix(strings.ToLower(parts[3]), "up")
			out = append(out, Container{
				ID:         parts[0][:min(12, len(parts[0]))],
				Name:       parts[1],
				Image:      parts[2],
				Status:     parts[3],
				Ports:      parts[4],
				RunningFor: parts[5],
				Runtime:    rt,
				Running:    running,
			})
		}
	}
	return out
}

func Logs(rt, id string, tail int) (string, error) {
	b, err := exec.Command(rt, "logs", "--tail", fmt.Sprintf("%d", tail), id).CombinedOutput()
	return string(b), err
}

func Stop(rt, id string) error {
	return exec.Command(rt, "stop", id).Run()
}

func Start(rt, id string) error {
	return exec.Command(rt, "start", id).Run()
}

func Delete(rt, id string) error {
	return exec.Command(rt, "rm", "-f", id).Run()
}

func GetStats(rt, id string) (Stats, error) {
	b, err := exec.Command(rt, "stats", id, "--no-stream",
		"--format", "{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}").Output()
	if err != nil {
		return Stats{}, err
	}
	parts := strings.Split(strings.TrimSpace(string(b)), "\t")
	for len(parts) < 4 {
		parts = append(parts, "—")
	}
	return Stats{
		CPU:      parts[0],
		MemUsage: parts[1],
		NetIO:    parts[2],
		BlockIO:  parts[3],
	}, nil
}

func Inspect(rt, id string) (InspectInfo, error) {
	b, err := exec.Command(rt, "inspect", id).Output()
	if err != nil {
		return InspectInfo{}, err
	}
	var raw []map[string]any
	if err := json.Unmarshal(b, &raw); err != nil || len(raw) == 0 {
		return InspectInfo{}, fmt.Errorf("parse error")
	}
	d := raw[0]
	info := InspectInfo{}
	info.ID = strField(d, "Id")
	info.Created = strField(d, "Created")

	if cfg, ok := d["Config"].(map[string]any); ok {
		info.Image = strField(cfg, "Image")
		if env, ok := cfg["Env"].([]any); ok {
			for _, e := range env {
				if s, ok := e.(string); ok {
					info.Env = append(info.Env, s)
				}
			}
		}
	}
	if state, ok := d["State"].(map[string]any); ok {
		info.Status = strField(state, "Status")
	}
	if name, ok := d["Name"].(string); ok {
		info.Name = strings.TrimPrefix(name, "/")
	}

	// resource limits
	if hc, ok := d["HostConfig"].(map[string]any); ok {
		info.Resources = ResourceLimits{
			MemoryBytes:     int64Field(hc, "Memory"),
			MemorySwapBytes: int64Field(hc, "MemorySwap"),
			NanoCPUs:        int64Field(hc, "NanoCpus"),
			CPUShares:       int64Field(hc, "CpuShares"),
			PidsLimit:       int64Field(hc, "PidsLimit"),
			CPUQuota:        int64Field(hc, "CpuQuota"),
			CPUPeriod:       int64Field(hc, "CpuPeriod"),
		}
	}

	// mounts
	if mounts, ok := d["Mounts"].([]any); ok {
		for _, m := range mounts {
			if mv, ok := m.(map[string]any); ok {
				info.Mounts = append(info.Mounts, Mount{
					Source:      strField(mv, "Source"),
					Destination: strField(mv, "Destination"),
					Mode:        strField(mv, "Mode"),
				})
			}
		}
	}
	return info, nil
}

func strField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func int64Field(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	}
	return 0
}
