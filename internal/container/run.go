package container

import (
	"embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

//go:embed presets.json
var presetsJSON embed.FS

// Preset defines a quick-run template for popular container images.
type Preset struct {
	Label    string            `json:"label"`
	Image    string            `json:"image"`
	Ports    []string          `json:"ports,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	Volumes  []string          `json:"volumes,omitempty"`
	Cmd      []string          `json:"cmd,omitempty"`
	Category string            `json:"category,omitempty"`
}

// LoadPresets reads the embedded presets.json and returns all presets.
// The last entry is always "Custom image..." with empty image.
func LoadPresets() []Preset {
	data, err := presetsJSON.ReadFile("presets.json")
	if err != nil {
		return []Preset{{Label: "Custom image...", Image: ""}}
	}
	var presets []Preset
	if err := json.Unmarshal(data, &presets); err != nil {
		return []Preset{{Label: "Custom image...", Image: ""}}
	}
	// append custom option at the end
	presets = append(presets, Preset{Label: "Custom image...", Image: ""})
	return presets
}

// FilterPresets returns presets matching the query (case-insensitive substring match on label, image, category).
func FilterPresets(presets []Preset, query string) []Preset {
	if query == "" {
		return presets
	}
	q := strings.ToLower(query)
	var matched []Preset
	for _, p := range presets {
		label := strings.ToLower(p.Label)
		image := strings.ToLower(p.Image)
		cat := strings.ToLower(p.Category)
		if strings.Contains(label, q) || strings.Contains(image, q) || strings.Contains(cat, q) {
			matched = append(matched, p)
		}
	}
	// always include custom option at the end
	matched = append(matched, Preset{Label: "Custom image...", Image: ""})
	return matched
}

// RunContainer executes docker/podman run with the given preset config.
func RunContainer(runtime string, p Preset, detach bool) (string, error) {
	args := []string{"run"}
	if detach {
		args = append(args, "-d")
	}

	name := sanitizeName(p.Label)
	if name != "" {
		args = append(args, "--name", name)
	}

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

// SearchImages searches the registry for images matching the query.
func SearchImages(runtime, query string) ([]string, error) {
	if query == "" {
		return nil, nil
	}
	args := []string{"search", "--limit", "15", "--format", "{{.Name}}", query}
	out, err := exec.Command(runtime, args...).Output()
	if err != nil {
		// fallback: try without --format (older podman)
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

// ListTags fetches available tags for an image (best-effort).
func ListTags(runtime, image string) []string {
	// Try skopeo first
	out, err := exec.Command("skopeo", "list-tags", "docker://docker.io/"+image).Output()
	if err == nil {
		return parseSkopeoTags(string(out))
	}

	// Try podman tag listing
	if runtime == "podman" {
		out, err = exec.Command("podman", "search", "--list-tags", "--limit", "20", image).Output()
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

func sanitizeName(label string) string {
	label = strings.ToLower(label)
	var b strings.Builder
	b.WriteString("dira-")
	for _, r := range label {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	s := b.String()
	return strings.TrimRight(s, "-")
}

// BuildRunCommand returns the full CLI command string (for display/copy).
func BuildRunCommand(runtime string, p Preset, detach bool) string {
	parts := []string{runtime, "run"}
	if detach {
		parts = append(parts, "-d")
	}
	name := sanitizeName(p.Label)
	if name != "" {
		parts = append(parts, "--name", name)
	}
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
