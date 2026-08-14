package clean

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

func tasks() []string {
	home, _ := os.UserHomeDir()
	bunCache := filepath.Join(home, ".bun", "install", "cache")
	trash := filepath.Join(home, ".local", "share", "Trash", "*")
	thumbnails := filepath.Join(home, ".cache", "thumbnails")
	return []string{
		"sudo paccache -rk1",
		"sudo pacman -Rns $(pacman -Qdtq)",
		"journalctl --vacuum-time=2weeks",
		"sudo rm -rf /tmp/*",
		"rm -rf " + trash,
		"rm -rf " + thumbnails,
		"rm -rf " + bunCache,
	}
}

func RunAll() {
	if err := exec.Command("sudo", "-v").Run(); err != nil {
		fmt.Fprintln(os.Stderr, "sudo authentication failed")
		os.Exit(1)
	}

	var wg sync.WaitGroup
	for _, cmd := range tasks() {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			run(c)
		}(cmd)
	}
	wg.Wait()
}

func run(cmdStr string) {
	fmt.Printf("  ▶ %s\n", cmdStr)
	cmd := exec.Command("/bin/sh", "-c", cmdStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("  ✗ %s: %v\n", cmdStr, err)
		return
	}
	fmt.Printf("  ✓ done\n")
}
