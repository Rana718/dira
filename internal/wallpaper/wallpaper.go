package wallpaper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func DefaultWallpaperDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, "Pictures", "wallpapers")
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	return filepath.Join(home, "Pictures")
}

func PickFile() (string, error) {
	dir := DefaultWallpaperDir()

	out, err := exec.Command(
		"zenity",
		"--file-selection",
		"--title=Select Wallpaper",
		"--filename="+dir+"/",
		"--file-filter=Images | *.jpg *.jpeg *.png *.webp *.gif *.bmp",
		"--file-filter=All files | *",
	).Output()

	if err != nil {
		if strings.Contains(err.Error(), "exit status 1") {
			return "", fmt.Errorf("cancelled")
		}
		return "", fmt.Errorf("zenity: %w", err)
	}

	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", fmt.Errorf("cancelled")
	}
	return path, nil
}

func Monitors() ([]string, error) {
	out, err := exec.Command("hyprctl", "monitors").Output()
	if err != nil {
		return nil, fmt.Errorf("hyprctl monitors: %w", err)
	}
	var monitors []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Monitor ") {
			// "Monitor eDP-1 (ID 0):"
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				monitors = append(monitors, parts[1])
			}
		}
	}
	if len(monitors) == 0 {
		return nil, fmt.Errorf("no monitors found — is Hyprland running?")
	}
	return monitors, nil
}

func Apply(path string) error {
	monitors, err := Monitors()
	if err != nil {
		return err
	}

	for _, mon := range monitors {
		arg := fmt.Sprintf("%s,%s,cover", mon, path)
		if out, err := exec.Command("hyprctl", "hyprpaper", "wallpaper", arg).CombinedOutput(); err != nil {
			return fmt.Errorf("failed on %s: %s", mon, strings.TrimSpace(string(out)))
		}
	}

	_ = saveLastWall(path)
	_ = updateHyprpaperConf(path)
	return nil
}

func updateHyprpaperConf(path string) error {
	home, _ := os.UserHomeDir()
	confPath := filepath.Join(home, ".config", "hypr", "hyprpaper.conf")

	monitors, err := Monitors()
	if err != nil {
		monitors = []string{"eDP-1"}
	}

	var sb strings.Builder
	for _, mon := range monitors {
		sb.WriteString("wallpaper {\n")
		sb.WriteString(fmt.Sprintf("    monitor = %s\n", mon))
		sb.WriteString(fmt.Sprintf("    path = %s\n", path))
		sb.WriteString("    fit_mode = cover\n")
		sb.WriteString("}\n\n")
	}
	sb.WriteString("splash = false\n")

	return os.WriteFile(confPath, []byte(sb.String()), 0644)
}

func statePath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "dira", "wallpaper")
}

func saveLastWall(path string) error {
	p := statePath()
	os.MkdirAll(filepath.Dir(p), 0755)
	return os.WriteFile(p, []byte(path), 0644)
}

func LastWall() string {
	b, err := os.ReadFile(statePath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
