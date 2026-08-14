package container

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/moby/moby/client"
)

// Image represents a local container image.
type Image struct {
	ID      string
	Repo    string
	Tag     string
	Size    string
	Created string
	Runtime string
	InUse   bool
}

// ListImages returns all local images from docker/podman.
func ListImages() []Image {
	var out []Image
	for _, rt := range []string{"docker", "podman"} {
		if !runtimeAvailable(rt) {
			continue
		}
		out = append(out, listImagesCLI(rt)...)
	}
	return out
}

func listImagesCLI(rt string) []Image {
	b, err := exec.Command(rt, "images", "--no-trunc",
		"--format", "{{.ID}}\t{{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}").Output()
	if err != nil {
		return nil
	}
	// collect image IDs used by containers
	usedIDs := usedImageIDs(rt)

	var out []Image
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		for len(parts) < 5 {
			parts = append(parts, "")
		}
		id := parts[0]
		if len(id) > 12 {
			id = id[:12]
		}
		out = append(out, Image{
			ID:      id,
			Repo:    parts[1],
			Tag:     parts[2],
			Size:    parts[3],
			Created: parts[4],
			Runtime: rt,
			InUse:   usedIDs[parts[0]],
		})
	}
	return out
}

func usedImageIDs(rt string) map[string]bool {
	b, err := exec.Command(rt, "ps", "-a", "--format", "{{.ImageID}}").Output()
	if err != nil {
		return nil
	}
	m := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line != "" {
			m[line] = true
		}
	}
	return m
}

// RemoveImage removes an image by ID.
func RemoveImage(rt, id string, force bool) error {
	cli, err := newDockerClient(rt)
	if err != nil {
		args := []string{"rmi", id}
		if force {
			args = []string{"rmi", "-f", id}
		}
		return exec.Command(rt, args...).Run()
	}
	defer cli.Close()
	ctx := context.Background()
	_, err = cli.ImageRemove(ctx, id, client.ImageRemoveOptions{Force: force, PruneChildren: true})
	return err
}

// PullImage pulls an image, streaming output to stdout via CLI.
func PullImage(rt, image string) error {
	cmd := exec.Command(rt, "pull", image)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// PruneImages removes all dangling images.
func PruneImages(rt string) (string, error) {
	b, err := exec.Command(rt, "image", "prune", "-f").CombinedOutput()
	return strings.TrimSpace(string(b)), err
}

// ImageInspectText returns formatted inspect output for an image.
func ImageInspectText(rt, id string) string {
	b, err := exec.Command(rt, "image", "inspect", id).Output()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return string(b)
}

// ─── Volume types & ops ──────────────────────────────────────────────────────

// Volume represents a named docker/podman volume.
type Volume struct {
	Name       string
	Driver     string
	Mountpoint string
	Created    string
	Runtime    string
	InUse      bool
}

// ListVolumes returns all named volumes.
func ListVolumes() []Volume {
	var out []Volume
	for _, rt := range []string{"docker", "podman"} {
		if !runtimeAvailable(rt) {
			continue
		}
		out = append(out, listVolumesCLI(rt)...)
	}
	return out
}

func listVolumesCLI(rt string) []Volume {
	b, err := exec.Command(rt, "volume", "ls",
		"--format", "{{.Name}}\t{{.Driver}}\t{{.Mountpoint}}").Output()
	if err != nil {
		return nil
	}
	usedVols := usedVolumeNames(rt)
	var out []Volume
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		out = append(out, Volume{
			Name:       parts[0],
			Driver:     parts[1],
			Mountpoint: parts[2],
			Runtime:    rt,
			InUse:      usedVols[parts[0]],
		})
	}
	return out
}

func usedVolumeNames(rt string) map[string]bool {
	b, err := exec.Command(rt, "ps", "-a",
		"--format", "{{.Mounts}}").Output()
	if err != nil {
		return nil
	}
	m := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		for _, part := range strings.Split(line, ",") {
			name := strings.TrimSpace(part)
			if name != "" {
				m[name] = true
			}
		}
	}
	return m
}

// RemoveVolume removes a named volume.
func RemoveVolume(rt, name string) error {
	out, err := exec.Command(rt, "volume", "rm", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// PruneVolumes removes all unused volumes.
func PruneVolumes(rt string) (string, error) {
	b, err := exec.Command(rt, "volume", "prune", "-f").CombinedOutput()
	return strings.TrimSpace(string(b)), err
}

// CreateVolume creates a named volume.
func CreateVolume(rt, name string) error {
	out, err := exec.Command(rt, "volume", "create", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ─── Network types & ops ─────────────────────────────────────────────────────

// Network represents a container network.
type Network struct {
	ID      string
	Name    string
	Driver  string
	Scope   string
	Subnet  string
	Runtime string
}

// ListNetworks returns all networks.
func ListNetworks() []Network {
	var out []Network
	for _, rt := range []string{"docker", "podman"} {
		if !runtimeAvailable(rt) {
			continue
		}
		out = append(out, listNetworksCLI(rt)...)
	}
	return out
}

func listNetworksCLI(rt string) []Network {
	b, err := exec.Command(rt, "network", "ls",
		"--format", "{{.ID}}\t{{.Name}}\t{{.Driver}}\t{{.Scope}}").Output()
	if err != nil {
		return nil
	}
	var out []Network
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		for len(parts) < 4 {
			parts = append(parts, "")
		}
		id := parts[0]
		if len(id) > 12 {
			id = id[:12]
		}
		out = append(out, Network{
			ID:      id,
			Name:    parts[1],
			Driver:  parts[2],
			Scope:   parts[3],
			Runtime: rt,
		})
	}
	return out
}

// NetworkInspectText returns formatted inspect output for a network.
func NetworkInspectText(rt, name string) string {
	b, err := exec.Command(rt, "network", "inspect", name).Output()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return string(b)
}

// RemoveNetwork removes a network.
func RemoveNetwork(rt, name string) error {
	out, err := exec.Command(rt, "network", "rm", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// CreateNetwork creates a named network.
func CreateNetwork(rt, name, driver string) error {
	args := []string{"network", "create"}
	if driver != "" {
		args = append(args, "--driver", driver)
	}
	args = append(args, name)
	out, err := exec.Command(rt, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ConnectNetwork connects a container to a network.
func ConnectNetwork(rt, network, containerID string) error {
	out, err := exec.Command(rt, "network", "connect", network, containerID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DisconnectNetwork disconnects a container from a network.
func DisconnectNetwork(rt, network, containerID string) error {
	out, err := exec.Command(rt, "network", "disconnect", network, containerID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ContainerNetworks returns the network names a container is connected to.
func ContainerNetworks(rt, id string) []string {
	b, err := exec.Command(rt, "inspect", id,
		"--format", "{{range $k,$v := .NetworkSettings.Networks}}{{$k}}\n{{end}}").Output()
	if err != nil {
		return nil
	}
	var nets []string
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line != "" {
			nets = append(nets, line)
		}
	}
	return nets
}

// PauseContainer pauses a running container.
func PauseContainer(rt, id string) error {
	out, err := exec.Command(rt, "pause", id).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// UnpauseContainer unpauses a paused container.
func UnpauseContainer(rt, id string) error {
	out, err := exec.Command(rt, "unpause", id).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RenameContainer renames a container.
func RenameContainer(rt, id, newName string) error {
	out, err := exec.Command(rt, "rename", id, newName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RestartContainer restarts a container.
func RestartContainer(rt, id string) error {
	out, err := exec.Command(rt, "restart", id).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// LogsFollow returns a command that streams logs (caller must start/kill it).
func LogsFollow(rt, id string) *exec.Cmd {
	return exec.Command(rt, "logs", "-f", "--tail", "100", id)
}

// FormatAge makes a duration string readable.
func FormatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
