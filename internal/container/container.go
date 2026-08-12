package container

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/moby/moby/client"
)

type Container struct {
	ID         string
	Name       string
	Image      string
	Status     string
	Ports      string
	RunningFor string
	Runtime    string
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
	NanoCPUs        int64
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

		cli, err := newDockerClient(rt)
		if err != nil {
			out = append(out, listCLI(rt)...)
			continue
		}

		ctx := context.Background()
		if _, err := cli.Ping(ctx, client.PingOptions{}); err != nil {
			cli.Close()
			out = append(out, listCLI(rt)...)
			continue
		}

		containers, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true})
		cli.Close()
		if err != nil {
			out = append(out, listCLI(rt)...)
			continue
		}

		for _, c := range containers.Items {
			name := ""
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}
			running := strings.ToLower(string(c.State)) == "running"

			var portParts []string
			for _, p := range c.Ports {
				if p.PublicPort > 0 {
					portParts = append(portParts, fmt.Sprintf("%d:%d", p.PublicPort, p.PrivatePort))
				} else {
					portParts = append(portParts, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
				}
			}

			id := c.ID
			if len(id) > 12 {
				id = id[:12]
			}

			out = append(out, Container{
				ID:         id,
				Name:       name,
				Image:      c.Image,
				Status:     c.Status,
				Ports:      strings.Join(portParts, ", "),
				RunningFor: c.Status,
				Runtime:    rt,
				Running:    running,
			})
		}
	}
	return out
}

func listCLI(rt string) []Container {
	var out []Container
	b, err := exec.Command(rt, "ps", "-a",
		"--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}\t{{.RunningFor}}").Output()
	if err != nil {
		return nil
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
	return out
}


func Logs(rt, id string, tail int) (string, error) {
	cli, err := newDockerClient(rt)
	if err != nil {
		return logsCLI(rt, id, tail)
	}
	defer cli.Close()

	ctx := context.Background()
	reader, err := cli.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
	})
	if err != nil {
		return logsCLI(rt, id, tail)
	}
	defer reader.Close()

	b, _ := io.ReadAll(reader)
	return stripDockerLogHeaders(string(b)), nil
}

func logsCLI(rt, id string, tail int) (string, error) {
	b, err := exec.Command(rt, "logs", "--tail", fmt.Sprintf("%d", tail), id).CombinedOutput()
	return string(b), err
}

func stripDockerLogHeaders(s string) string {
	var out strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if len(line) >= 8 {
			if line[0] <= 2 && line[1] == 0 && line[2] == 0 && line[3] == 0 {
				out.WriteString(line[8:])
				out.WriteByte('\n')
				continue
			}
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}

func Stop(rt, id string) error {
	cli, err := newDockerClient(rt)
	if err != nil {
		return exec.Command(rt, "stop", id).Run()
	}
	defer cli.Close()

	ctx := context.Background()
	_, err = cli.ContainerStop(ctx, id, client.ContainerStopOptions{})
	return err
}

func Start(rt, id string) error {
	cli, err := newDockerClient(rt)
	if err != nil {
		return exec.Command(rt, "start", id).Run()
	}
	defer cli.Close()

	ctx := context.Background()
	_, err = cli.ContainerStart(ctx, id, client.ContainerStartOptions{})
	return err
}

func Delete(rt, id string) error {
	cli, err := newDockerClient(rt)
	if err != nil {
		return exec.Command(rt, "rm", "-f", id).Run()
	}
	defer cli.Close()

	ctx := context.Background()
	_, err = cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})
	return err
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
	cli, err := newDockerClient(rt)
	if err != nil {
		return inspectCLI(rt, id)
	}
	defer cli.Close()

	ctx := context.Background()
	result, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return inspectCLI(rt, id)
	}

	data := result.Container
	info := InspectInfo{
		ID:      data.ID,
		Name:    strings.TrimPrefix(data.Name, "/"),
		Created: data.Created,
	}

	if data.Config != nil {
		info.Image = data.Config.Image
		info.Env = data.Config.Env
	}
	if data.State != nil {
		info.Status = string(data.State.Status)
	}

	if data.HostConfig != nil {
		res := data.HostConfig.Resources
		info.Resources = ResourceLimits{
			MemoryBytes:     res.Memory,
			MemorySwapBytes: res.MemorySwap,
			NanoCPUs:        res.NanoCPUs,
			CPUShares:       res.CPUShares,
			PidsLimit:       derefInt64(res.PidsLimit),
			CPUQuota:        res.CPUQuota,
			CPUPeriod:       res.CPUPeriod,
		}
	}

	for _, m := range data.Mounts {
		info.Mounts = append(info.Mounts, Mount{
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
		})
	}

	return info, nil
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func inspectCLI(rt, id string) (InspectInfo, error) {
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


type ContainerDetail struct {
	ID         string
	Name       string
	Image      string
	Created    string
	Status     string
	StartedAt  string
	Platform   string
	RestartCnt int
	Cmd        []string
	Env        []string
	Ports      []string
	Mounts     []Mount
	Resources  ResourceLimits
}

func GetDetail(rt, id string) (ContainerDetail, error) {
	cli, err := newDockerClient(rt)
	if err != nil {
		return getDetailCLI(rt, id)
	}
	defer cli.Close()

	ctx := context.Background()
	result, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return getDetailCLI(rt, id)
	}

	data := result.Container
	d := ContainerDetail{
		ID:      data.ID,
		Name:    strings.TrimPrefix(data.Name, "/"),
		Created: data.Created,
	}

	if data.State != nil {
		d.Status = string(data.State.Status)
		d.StartedAt = data.State.StartedAt
	}
	if data.Config != nil {
		d.Image = data.Config.Image
		d.Cmd = data.Config.Cmd
		d.Env = data.Config.Env
	}
	if data.HostConfig != nil {
		res := data.HostConfig.Resources
		d.Resources = ResourceLimits{
			MemoryBytes:     res.Memory,
			MemorySwapBytes: res.MemorySwap,
			NanoCPUs:        res.NanoCPUs,
			CPUShares:       res.CPUShares,
			PidsLimit:       derefInt64(res.PidsLimit),
			CPUQuota:        res.CPUQuota,
			CPUPeriod:       res.CPUPeriod,
		}
	}
	d.RestartCnt = data.RestartCount

	for _, m := range data.Mounts {
		d.Mounts = append(d.Mounts, Mount{
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
		})
	}

	return d, nil
}

func getDetailCLI(rt, id string) (ContainerDetail, error) {
	info, err := inspectCLI(rt, id)
	if err != nil {
		return ContainerDetail{}, err
	}
	return ContainerDetail{
		ID:      info.ID,
		Name:    info.Name,
		Image:   info.Image,
		Status:  info.Status,
		Created: info.Created,
		Env:     info.Env,
		Mounts:  info.Mounts,
	}, nil
}

type ImageLayer struct {
	CreatedAt string
	CreatedBy string
	Size      string
}

func GetImageHistory(rt, imageRef string) ([]ImageLayer, error) {
	b, err := exec.Command(rt, "history", "--no-trunc",
		"--format", "{{.CreatedAt}}\t{{.CreatedBy}}\t{{.Size}}", imageRef).Output()
	if err != nil {
		return nil, err
	}
	var layers []ImageLayer
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		createdBy := parts[1]
		if len(createdBy) > 80 {
			createdBy = createdBy[:80] + "…"
		}
		layers = append(layers, ImageLayer{
			CreatedAt: parts[0],
			CreatedBy: createdBy,
			Size:      parts[2],
		})
	}
	return layers, nil
}

func EditConfig(rt, id string) error {
	cli, err := newDockerClient(rt)
	if err != nil {
		return fmt.Errorf("cannot connect to daemon")
	}
	defer cli.Close()

	ctx := context.Background()
	result, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return fmt.Errorf("inspect: %w", err)
	}

	editable := buildEditableConfig(result.Container)
	tmpFile, err := writeTempJSON(editable)
	if err != nil {
		return err
	}

	return applyEditedConfig(rt, id, tmpFile)
}

type EditableConfig struct {
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Cmd     []string          `json:"cmd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Ports   []string          `json:"ports,omitempty"`
	Volumes []string          `json:"volumes,omitempty"`
}

func buildEditableConfig(data interface{}) EditableConfig {
	type inspectable interface {
		GetName() string
	}
	b, _ := json.Marshal(data)
	var raw map[string]any
	json.Unmarshal(b, &raw)

	cfg := EditableConfig{}
	if name, ok := raw["Name"].(string); ok {
		cfg.Name = strings.TrimPrefix(name, "/")
	}
	if config, ok := raw["Config"].(map[string]any); ok {
		if img, ok := config["Image"].(string); ok {
			cfg.Image = img
		}
		if cmd, ok := config["Cmd"].([]any); ok {
			for _, c := range cmd {
				if s, ok := c.(string); ok {
					cfg.Cmd = append(cfg.Cmd, s)
				}
			}
		}
		if env, ok := config["Env"].([]any); ok {
			cfg.Env = map[string]string{}
			for _, e := range env {
				if s, ok := e.(string); ok {
					if eq := strings.IndexByte(s, '='); eq > 0 {
						cfg.Env[s[:eq]] = s[eq+1:]
					}
				}
			}
		}
	}
	if hc, ok := raw["HostConfig"].(map[string]any); ok {
		if binds, ok := hc["Binds"].([]any); ok {
			for _, b := range binds {
				if s, ok := b.(string); ok {
					cfg.Volumes = append(cfg.Volumes, s)
				}
			}
		}
	}
	return cfg
}

func writeTempJSON(cfg EditableConfig) (string, error) {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}

	f, err := os.CreateTemp("", "dira-config-*.json")
	if err != nil {
		return "", err
	}
	f.Write(b)
	f.Close()
	return f.Name(), nil
}

func applyEditedConfig(rt, id, tmpFile string) error {

	b, err := os.ReadFile(tmpFile)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	os.Remove(tmpFile)

	var cfg EditableConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	_ = Stop(rt, id)

	_ = Delete(rt, id)

	preset := Preset{
		Label:   cfg.Name,
		Name:    cfg.Name,
		Image:   cfg.Image,
		Cmd:     cfg.Cmd,
		Volumes: cfg.Volumes,
	}
	if len(cfg.Env) > 0 {
		preset.Env = cfg.Env
	}
	if len(cfg.Ports) > 0 {
		preset.Ports = cfg.Ports
	}

	_, err = RunContainer(rt, preset, true)
	return err
}

func GetConfigTempFile(rt, id string) (string, error) {
	cli, err := newDockerClient(rt)
	if err != nil {
		return "", fmt.Errorf("cannot connect to daemon")
	}
	defer cli.Close()

	ctx := context.Background()
	result, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		b, err := exec.Command(rt, "inspect", id).Output()
		if err != nil {
			return "", err
		}
		var raw []map[string]any
		json.Unmarshal(b, &raw)
		if len(raw) == 0 {
			return "", fmt.Errorf("empty inspect")
		}
		cfg := buildEditableConfig(raw[0])
		return writeTempJSON(cfg)
	}

	cfg := buildEditableConfig(result.Container)
	return writeTempJSON(cfg)
}

func ApplyEditedConfig(rt, id, tmpFile string) error {
	return applyEditedConfig(rt, id, tmpFile)
}
