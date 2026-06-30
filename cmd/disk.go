package cmd

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	diskHdr    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	diskLabel  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	diskVal    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	diskGood   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	diskWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	diskCrit   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	diskDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	diskDevice = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
)

type diskEntry struct {
	device  string
	size    string
	used    string
	avail   string
	usePct  int
	mount   string
	fstype  string
}

type blockDevice struct {
	name    string
	size    string
	devType string // disk, part
	fstype  string
	mount   string
	model   string
	tran    string // nvme, sata, usb
	rota    bool   // rotational = HDD
}

var diskCmd = &cobra.Command{
	Use:   "disk",
	Short: "Show disk usage, partitions, and SSD health",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		mounts := getDiskUsage()
		blocks := getBlockDevices()
		ssdHealth := getSSDHealth()

		// ── Disk Usage ──
		fmt.Println(diskHdr.Render("── Disk Usage ───────────────────────────────────"))
		fmt.Println(diskHdr.Render(fmt.Sprintf("  %-28s  %-8s  %-8s  %-8s  %-6s  %s",
			"MOUNT", "SIZE", "USED", "FREE", "USE%", "FILESYSTEM")))
		fmt.Println(diskDim.Render("  " + strings.Repeat("─", 78)))

		for _, d := range mounts {
			// split primary mount from extras (btrfs subvolumes etc.)
			mountParts := strings.Split(d.mount, "\x00")
			primaryMount := mountParts[0]
			extras := mountParts[1:]

			bar := usageBar(d.usePct, 20)
			pctStyle := diskGood
			if d.usePct >= 90 {
				pctStyle = diskCrit
			} else if d.usePct >= 75 {
				pctStyle = diskWarn
			}

			fmt.Printf("  %-28s  %-8s  %-8s  %-8s  %s  %s\n",
				diskVal.Render(pad(primaryMount, 28)),
				diskVal.Render(pad(d.size, 8)),
				diskVal.Render(pad(d.used, 8)),
				diskVal.Render(pad(d.avail, 8)),
				bar+" "+pctStyle.Render(fmt.Sprintf("%d%%", d.usePct)),
				diskDim.Render(d.fstype),
			)
			// show shared subvolumes/mounts dimmed and indented
			for _, extra := range extras {
				fmt.Printf("  %s\n",
					diskDim.Render("  └─ "+extra+"  (same device)"),
				)
			}
		}

		// ── Block Devices ──
		fmt.Println(diskHdr.Render("\n── Block Devices ────────────────────────────────"))
		fmt.Println(diskHdr.Render(fmt.Sprintf("  %-12s  %-8s  %-6s  %-8s  %-20s  %s",
			"DEVICE", "SIZE", "TYPE", "TRANS", "FSTYPE", "MOUNT / MODEL")))
		fmt.Println(diskDim.Render("  " + strings.Repeat("─", 78)))

		for _, b := range blocks {
			nameStyle := diskDim
			if b.devType == "disk" {
				nameStyle = diskDevice
			}

			driveType := b.tran
			if b.rota {
				driveType = "HDD"
			} else if driveType == "" && b.devType == "disk" {
				driveType = "SSD"
			}

			extra := b.mount
			if b.model != "" {
				if extra != "" {
					extra = b.model + "  " + extra
				} else {
					extra = b.model
				}
			}

			fmt.Printf("  %s  %-8s  %-6s  %-8s  %-20s  %s\n",
				nameStyle.Render(pad(b.name, 12)),
				diskVal.Render(pad(b.size, 8)),
				diskDim.Render(pad(b.devType, 6)),
				diskVal.Render(pad(driveType, 8)),
				diskDim.Render(pad(b.fstype, 20)),
				diskDim.Render(extra),
			)
		}

		// ── SSD Health ──
		if len(ssdHealth) > 0 {
			fmt.Println(diskHdr.Render("\n── SSD / NVMe Health ────────────────────────────"))
			for _, h := range ssdHealth {
				fmt.Println(h)
			}
		}

		return nil
	},
}

func usageBar(pct, width int) string {
	filled := pct * width / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	switch {
	case pct >= 90:
		return diskCrit.Render(bar)
	case pct >= 75:
		return diskWarn.Render(bar)
	default:
		return diskGood.Render(bar)
	}
}

func getDiskUsage() []diskEntry {
	out, err := exec.Command("df", "-h", "--output=source,size,used,avail,pcent,target,fstype").Output()
	if err != nil {
		return nil
	}

	skip := map[string]bool{
		"tmpfs": true, "devtmpfs": true, "efivarfs": true,
		"none": true, "overlay": true, "squashfs": true,
	}

	// group by device — keep first entry, collect extra mounts
	type group struct {
		entry  diskEntry
		extras []string // additional mount points on same device
	}
	byDevice := map[string]*group{}
	var order []string

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if first {
			first = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		fstype := fields[6]
		if skip[fstype] {
			continue
		}
		pctStr := strings.TrimSuffix(fields[4], "%")
		pct, _ := strconv.Atoi(pctStr)
		dev := fields[0]
		mount := fields[5]

		if g, exists := byDevice[dev]; exists {
			g.extras = append(g.extras, mount)
		} else {
			byDevice[dev] = &group{
				entry: diskEntry{
					device: dev,
					size:   fields[1],
					used:   fields[2],
					avail:  fields[3],
					usePct: pct,
					mount:  mount,
					fstype: fstype,
				},
			}
			order = append(order, dev)
		}
	}

	var entries []diskEntry
	for _, dev := range order {
		g := byDevice[dev]
		// encode extras into mount field for rendering
		if len(g.extras) > 0 {
			g.entry.mount = g.entry.mount + "\x00" + strings.Join(g.extras, "\x00")
		}
		entries = append(entries, g.entry)
	}
	return entries
}

func getBlockDevices() []blockDevice {
	out, err := exec.Command("lsblk", "-o", "NAME,SIZE,TYPE,FSTYPE,MOUNTPOINT,MODEL,ROTA,TRAN",
		"--noheadings", "--pairs").Output()
	if err != nil {
		return nil
	}

	var devices []blockDevice
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		b := blockDevice{}
		for _, pair := range strings.Fields(line) {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) != 2 {
				continue
			}
			val := strings.Trim(kv[1], `"`)
			switch kv[0] {
			case "NAME":
				b.name = val
			case "SIZE":
				b.size = val
			case "TYPE":
				b.devType = val
			case "FSTYPE":
				b.fstype = val
			case "MOUNTPOINT":
				b.mount = val
			case "MODEL":
				b.model = strings.TrimSpace(val)
			case "ROTA":
				b.rota = val == "1"
			case "TRAN":
				b.tran = val
			}
		}
		if b.devType == "disk" || b.devType == "part" {
			devices = append(devices, b)
		}
	}
	return devices
}

func getSSDHealth() []string {
	// find nvme/sata drives
	lsOut, err := exec.Command("lsblk", "-o", "NAME,TYPE", "--noheadings").Output()
	if err != nil {
		return nil
	}

	var drives []string
	scanner := bufio.NewScanner(strings.NewReader(string(lsOut)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[1] == "disk" {
			name := strings.TrimLeft(fields[0], "└─├─")
			drives = append(drives, "/dev/"+name)
		}
	}

	kv := func(k, v string) string {
		return diskLabel.Render(fmt.Sprintf("    %-26s", k+":")) + diskVal.Render(v) + "\n"
	}

	var lines []string
	for _, dev := range drives {
		cmd := exec.Command("sudo", "smartctl", "-a", dev)
		cmd.Stdin = nil
		out, err := cmd.Output()
		if err != nil {
			continue
		}

		info := map[string]string{}
		sc := bufio.NewScanner(strings.NewReader(string(out)))
		for sc.Scan() {
			line := sc.Text()
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}

		model := info["Model Number"]
		if model == "" {
			model = info["Device Model"]
		}
		if model == "" {
			continue
		}

		header := diskHdr.Render(fmt.Sprintf("  %s  ", diskDevice.Render(dev))) +
			diskVal.Render(model) + "\n"

		s := header
		s += kv("Health", healthColor(info["SMART overall-health self-assessment test result"]))
		s += kv("Temperature", tempColorStr(info["Temperature"]))
		s += kv("Warn / Crit temp",
			stripCelsius(info["Warning  Comp. Temp. Threshold"])+"°C / "+
				stripCelsius(info["Critical Comp. Temp. Threshold"])+"°C")
		s += kv("Wear level", wearColor(info["Percentage Used"]))
		s += kv("Spare available", info["Available Spare"]+" (threshold "+info["Available Spare Threshold"]+")")
		s += kv("Data read", info["Data Units Read"])
		s += kv("Data written", info["Data Units Written"])
		s += kv("Power on hours", info["Power On Hours"])
		s += kv("Power cycles", info["Power Cycles"])
		s += kv("Unsafe shutdowns", unsafeColorStr(info["Unsafe Shutdowns"]))
		s += kv("Media errors", errColorStr(info["Media and Data Integrity Errors"]))

		lines = append(lines, s)
	}
	return lines
}

func stripCelsius(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, " Celsius")
	return strings.Fields(s)[0]
}

func healthColor(s string) string {
	if strings.Contains(strings.ToUpper(s), "PASSED") {
		return diskGood.Render(s)
	}
	return diskCrit.Render(s)
}

func tempColorStr(s string) string {
	s = strings.TrimSuffix(strings.TrimSpace(s), " Celsius")
	v, err := strconv.Atoi(strings.Fields(s)[0])
	if err != nil {
		return s
	}
	out := fmt.Sprintf("%d°C", v)
	if v >= 70 {
		return diskCrit.Render(out)
	} else if v >= 55 {
		return diskWarn.Render(out)
	}
	return diskGood.Render(out)
}

func wearColor(s string) string {
	s = strings.TrimSuffix(s, "%")
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return s + "%"
	}
	out := fmt.Sprintf("%d%% used", v)
	if v >= 90 {
		return diskCrit.Render(out + "  ⚠ replace soon")
	} else if v >= 70 {
		return diskWarn.Render(out)
	}
	return diskGood.Render(out)
}

func unsafeColorStr(s string) string {
	s = strings.ReplaceAll(s, ",", "")
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return s
	}
	if v > 500 {
		return diskWarn.Render(fmt.Sprintf("%d  (high — avoid forced shutdowns)", v))
	}
	return diskVal.Render(fmt.Sprintf("%d", v))
}

func errColorStr(s string) string {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return s
	}
	if v > 0 {
		return diskCrit.Render(fmt.Sprintf("%d  ⚠ errors detected", v))
	}
	return diskGood.Render("0  clean")
}
