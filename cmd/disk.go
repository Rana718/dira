package cmd

import (
	"fmt"
	"strings"

	"github.com/Rana718/dira/internal/disk"
	"github.com/Rana718/dira/internal/helper"
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

var diskCmd = &cobra.Command{
	Use:   "disk",
	Short: "Show disk usage, partitions, and SSD health",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		printDiskUsage(disk.GetMounts())
		printBlockDevices(disk.GetBlockDevices())
		printSSDHealth(disk.GetSSDHealth())
		return nil
	},
}

func printDiskUsage(mounts []disk.Mount) {
	fmt.Println(diskHdr.Render("── Disk Usage ───────────────────────────────────"))
	fmt.Println(diskHdr.Render(fmt.Sprintf("  %-28s  %-8s  %-8s  %-8s  %-26s  %s", "MOUNT", "SIZE", "USED", "FREE", "USE%", "FILESYSTEM")))
	fmt.Println(diskDim.Render("  " + strings.Repeat("─", 88)))
	for _, d := range mounts {
		parts := strings.Split(d.Mount, "\x00")
		bar := usageBar(d.UsePct, 20)
		pctStyle := diskGood
		if d.UsePct >= 90 {
			pctStyle = diskCrit
		} else if d.UsePct >= 75 {
			pctStyle = diskWarn
		}
		fmt.Printf("  %-28s  %-8s  %-8s  %-8s  %s  %s\n",
			diskVal.Render(helper.Pad(parts[0], 28)),
			diskVal.Render(helper.Pad(d.Size, 8)),
			diskVal.Render(helper.Pad(d.Used, 8)),
			diskVal.Render(helper.Pad(d.Avail, 8)),
			bar+" "+pctStyle.Render(fmt.Sprintf("%d%%", d.UsePct)),
			diskDim.Render(d.FSType),
		)
		for _, extra := range parts[1:] {
			fmt.Printf("  %s\n", diskDim.Render("  └─ "+extra+"  (same device)"))
		}
	}
}

func printBlockDevices(blocks []disk.BlockDevice) {
	fmt.Println(diskHdr.Render("\n── Block Devices ────────────────────────────────"))
	fmt.Println(diskHdr.Render(fmt.Sprintf("  %-12s  %-8s  %-6s  %-8s  %-20s  %s", "DEVICE", "SIZE", "TYPE", "TRANS", "FSTYPE", "MOUNT / MODEL")))
	fmt.Println(diskDim.Render("  " + strings.Repeat("─", 78)))
	for _, b := range blocks {
		nameStyle := diskDim
		if b.DevType == "disk" {
			nameStyle = diskDevice
		}
		driveType := b.Tran
		if b.Rota {
			driveType = "HDD"
		} else if driveType == "" && b.DevType == "disk" {
			driveType = "SSD"
		}
		extra := b.Mount
		if b.Model != "" {
			if extra != "" {
				extra = b.Model + "  " + extra
			} else {
				extra = b.Model
			}
		}
		fmt.Printf("  %s  %-8s  %-6s  %-8s  %-20s  %s\n",
			nameStyle.Render(helper.Pad(b.Name, 12)),
			diskVal.Render(helper.Pad(b.Size, 8)),
			diskDim.Render(helper.Pad(b.DevType, 6)),
			diskVal.Render(helper.Pad(driveType, 8)),
			diskDim.Render(helper.Pad(b.FSType, 20)),
			diskDim.Render(extra),
		)
	}
}

func printSSDHealth(drives []disk.SSDHealth) {
	if len(drives) == 0 {
		return
	}
	fmt.Println(diskHdr.Render("\n── SSD / NVMe Health ────────────────────────────"))
	kv := func(k, v string) string {
		return diskLabel.Render(fmt.Sprintf("    %-26s", k+":")) + v + "\n"
	}
	for _, h := range drives {
		fmt.Print(diskHdr.Render("  "+diskDevice.Render(h.Device)+"  ") + diskVal.Render(h.Model) + "\n")
		fmt.Print(kv("Health", diskHealthColor(h.Health)))
		fmt.Print(kv("Temperature", diskTempColor(h.TempC, 55, 70)))
		fmt.Print(kv("Warn / Crit temp", fmt.Sprintf("%d°C / %d°C", h.WarnTempC, h.CritTempC)))
		fmt.Print(kv("Wear level", diskWearColor(h.WearPct)))
		fmt.Print(kv("Spare available", h.SpareAvail+" (threshold "+h.SpareThresh+")"))
		fmt.Print(kv("Data read", h.DataRead))
		fmt.Print(kv("Data written", h.DataWritten))
		fmt.Print(kv("Power on hours", h.PowerOnHours))
		fmt.Print(kv("Power cycles", h.PowerCycles))
		fmt.Print(kv("Unsafe shutdowns", diskUnsafeColor(h.UnsafeShuts)))
		fmt.Print(kv("Media errors", diskErrColor(h.MediaErrors)))
	}
}

func usageBar(pct, width int) string {
	filled := pct * width / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	if pct >= 90 {
		return diskCrit.Render(bar)
	} else if pct >= 75 {
		return diskWarn.Render(bar)
	}
	return diskGood.Render(bar)
}

func diskHealthColor(s string) string {
	if strings.Contains(strings.ToUpper(s), "PASSED") {
		return diskGood.Render(s)
	}
	return diskCrit.Render(s)
}

func diskTempColor(t, warn, crit int) string {
	s := fmt.Sprintf("%d°C", t)
	if t >= crit {
		return diskCrit.Render(s)
	} else if t >= warn {
		return diskWarn.Render(s)
	}
	return diskGood.Render(s)
}

func diskWearColor(pct int) string {
	s := fmt.Sprintf("%d%% used", pct)
	if pct >= 90 {
		return diskCrit.Render(s + "  ⚠ replace soon")
	} else if pct >= 70 {
		return diskWarn.Render(s)
	}
	return diskGood.Render(s)
}

func diskUnsafeColor(n int) string {
	if n > 500 {
		return diskWarn.Render(fmt.Sprintf("%d  (high — avoid forced shutdowns)", n))
	}
	return diskVal.Render(fmt.Sprintf("%d", n))
}

func diskErrColor(n int) string {
	if n > 0 {
		return diskCrit.Render(fmt.Sprintf("%d  ⚠ errors detected", n))
	}
	return diskGood.Render("0  clean")
}
