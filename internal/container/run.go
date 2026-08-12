package container

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"strings"

	mobycontainer "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

var presetsJSON embed.FS

type Preset struct {
	Label    string            `json:"label"`
	Image    string            `json:"image"`
	Ports    []string          `json:"ports,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	Volumes  []string          `json:"volumes,omitempty"`
	Cmd      []string          `json:"cmd,omitempty"`
	Category string            `json:"category,omitempty"`
	Name     string            `json:"name,omitempty"`
}

func LoadPresets() []Preset {
	data, err := presetsJSON.ReadFile("presets.json")
	if err != nil {
		return []Preset{{Label: "Custom image...", Image: ""}}
	}
	var presets []Preset
	if err := json.Unmarshal(data, &presets); err != nil {
		return []Preset{{Label: "Custom image...", Image: ""}}
	}
	presets = append(presets, Preset{Label: "Custom image...", Image: ""})
	return presets
}

func FilterPresets(presets []Preset, query string) []Preset {
	if query == "" {
		return presets
	}
	q := strings.ToLower(query)
	var matched []Preset
	for _, p := range presets {
		label := strings.ToLower(p.Label)
		img := strings.ToLower(p.Image)
		cat := strings.ToLower(p.Category)
		if strings.Contains(label, q) || strings.Contains(img, q) || strings.Contains(cat, q) {
			matched = append(matched, p)
		}
	}
	matched = append(matched, Preset{Label: "Custom image...", Image: ""})
	return matched
}


func newDockerClient(runtime string) (*client.Client, error) {
	if runtime == "podman" {
		sock := podmanSocket()
		if sock == "" {
			return nil, fmt.Errorf("podman socket not found")
		}
		return client.NewClientWithOpts(
			client.WithHost("unix://"+sock),
			client.WithAPIVersionNegotiation(),
		)
	}

	dockerSock := "/var/run/docker.sock"
	if !isSocket(dockerSock) {
		userSock := fmt.Sprintf("/run/user/%d/docker.sock", os.Getuid())
		if !isSocket(userSock) {
			return nil, fmt.Errorf("docker socket not found")
		}
		dockerSock = userSock
	}

	return client.NewClientWithOpts(
		client.WithHost("unix://"+dockerSock),
		client.WithAPIVersionNegotiation(),
	)
}

func podmanSocket() string {
	uid := os.Getuid()
	paths := []string{
		fmt.Sprintf("/run/user/%d/podman/podman.sock", uid),
		"/run/podman/podman.sock",
		"/var/run/podman/podman.sock",
	}
	for _, p := range paths {
		if isSocket(p) {
			return p
		}
	}
	return ""
}

func daemonSocket(runtime string) string {
	if runtime == "podman" {
		return podmanSocket()
	}
	paths := []string{
		"/var/run/docker.sock",
		fmt.Sprintf("/run/user/%d/docker.sock", os.Getuid()),
	}
	for _, p := range paths {
		if isSocket(p) {
			return p
		}
	}
	return ""
}

func isSocket(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSocket != 0
}


func RunContainer(runtime string, p Preset, detach bool) (string, error) {
	ctx := context.Background()

	cli, err := newDockerClient(runtime)
	if err != nil {
		return runContainerCLI(runtime, p, detach)
	}
	defer cli.Close()

	if _, err := cli.Ping(ctx, client.PingOptions{}); err != nil {
		return runContainerCLI(runtime, p, detach)
	}

	pullResp, err := cli.ImagePull(ctx, p.Image, client.ImagePullOptions{})
	if err == nil {
		io.Copy(io.Discard, pullResp)
		pullResp.Close()
	}

	var envList []string
	for k, v := range p.Env {
		envList = append(envList, k+"="+v)
	}

	exposedPorts := network.PortSet{}
	portBindings := network.PortMap{}
	for _, mapping := range p.Ports {
		parts := strings.SplitN(mapping, ":", 2)
		if len(parts) != 2 {
			continue
		}
		hostPort := parts[0]
		containerPort, err := network.ParsePort(parts[1] + "/tcp")
		if err != nil {
			continue
		}
		exposedPorts[containerPort] = struct{}{}
		portBindings[containerPort] = []network.PortBinding{
			{HostIP: netip.MustParseAddr("0.0.0.0"), HostPort: hostPort},
		}
	}

	name := containerName(p)

	cfg := &mobycontainer.Config{
		Image:        p.Image,
		Env:          envList,
		ExposedPorts: exposedPorts,
	}
	if len(p.Cmd) > 0 {
		cfg.Cmd = p.Cmd
	}

	hostCfg := &mobycontainer.HostConfig{
		PortBindings: portBindings,
	}
	if len(p.Volumes) > 0 {
		hostCfg.Binds = p.Volumes
	}

	resp, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     cfg,
		HostConfig: hostCfg,
		Name:       name,
	})
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}

	if _, err := cli.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		return resp.ID, fmt.Errorf("start: %w", err)
	}

	return resp.ID, nil
}

func runContainerCLI(runtime string, p Preset, detach bool) (string, error) {
	args := []string{"run"}
	if detach {
		args = append(args, "-d")
	}

	name := containerName(p)
	args = append(args, "--name", name)

	for _, port := range p.Ports {
		args = append(args, "-p", port)
	}
	for k, v := range p.Env {
		args = append(args, "-e", k+"="+v)
	}
	for _, vol := range p.Volumes {
		args = append(args, "-v", vol)
	}

	args = append(args, p.Image)
	args = append(args, p.Cmd...)

	cmd := exec.Command(runtime, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}


func SearchImages(runtime, query string) ([]string, error) {
	if query == "" {
		return nil, nil
	}

	ctx := context.Background()
	cli, err := newDockerClient(runtime)
	if err == nil {
		defer cli.Close()
		result, err := cli.ImageSearch(ctx, query, client.ImageSearchOptions{})
		if err == nil && len(result.Items) > 0 {
			var names []string
			for _, r := range result.Items {
				names = append(names, r.Name)
				if len(names) >= 15 {
					break
				}
			}
			return names, nil
		}
	}

	args := []string{"search", "--limit", "15", "--format", "{{.Name}}", query}
	out, err := exec.Command(runtime, args...).Output()
	if err != nil {
		out, err = exec.Command(runtime, "search", "--limit", "15", query).Output()
		if err != nil {
			return nil, err
		}
		var results []string
		for i, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if i == 0 {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) > 0 {
				results = append(results, fields[0])
			}
		}
		return results, nil
	}
	var results []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			results = append(results, line)
		}
	}
	return results, nil
}

func ListTags(runtime, img string) []string {
	out, err := exec.Command("skopeo", "list-tags", "docker://docker.io/"+img).Output()
	if err == nil {
		return parseSkopeoTags(string(out))
	}

	if runtime == "podman" {
		out, err = exec.Command("podman", "search", "--list-tags", "--limit", "20", img).Output()
		if err == nil {
			return parsePodmanTags(string(out))
		}
	}

	return []string{"latest", "alpine", "slim"}
}

func parseSkopeoTags(raw string) []string {
	var tags []string
	idx := strings.Index(raw, `"Tags"`)
	if idx < 0 {
		return nil
	}
	start := strings.Index(raw[idx:], "[")
	end := strings.Index(raw[idx:], "]")
	if start < 0 || end < 0 {
		return nil
	}
	inner := raw[idx+start+1 : idx+end]
	for _, part := range strings.Split(inner, ",") {
		tag := strings.Trim(strings.TrimSpace(part), `"`)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	if len(tags) > 20 {
		tags = tags[len(tags)-20:]
	}
	return tags
}

func parsePodmanTags(raw string) []string {
	var tags []string
	for i, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if i == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			tags = append(tags, fields[len(fields)-1])
		}
	}
	if len(tags) > 20 {
		tags = tags[len(tags)-20:]
	}
	return tags
}


func containerName(p Preset) string {
	name := p.Name
	if name == "" {
		name = sanitizeName(p.Label)
	}
	return name
}

func sanitizeName(label string) string {
	label = strings.ToLower(label)
	var b strings.Builder
	for _, r := range label {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	s := strings.TrimRight(b.String(), "-")
	if s == "" {
		return "container"
	}
	return s
}

func BuildRunCommand(runtime string, p Preset, detach bool) string {
	parts := []string{runtime, "run"}
	if detach {
		parts = append(parts, "-d")
	}
	name := containerName(p)
	parts = append(parts, "--name", name)
	for _, port := range p.Ports {
		parts = append(parts, "-p", port)
	}
	for k, v := range p.Env {
		parts = append(parts, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	for _, vol := range p.Volumes {
		parts = append(parts, "-v", vol)
	}
	parts = append(parts, p.Image)
	parts = append(parts, p.Cmd...)
	return strings.Join(parts, " ")
}
