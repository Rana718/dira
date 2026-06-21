package cmd

import (
	"fmt"
	"strings"

	"github.com/Rana718/dira/internal/info"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	infoHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	infoLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(26)
	infoValue   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	infoWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	infoBad     = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	infoGood    = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	infoDivider = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func infoRow(label, val string) string {
	return infoLabel.Render("  "+label+":") + val + "\n"
}

func section(title string) string {
	pad := 38 - len(title)
	if pad < 0 {
		pad = 0
	}
	return "\n" + infoHeader.Render("── "+title+" "+strings.Repeat("─", pad)) + "\n"
}

var (
	flagCPU, flagGPU, flagRAM, flagBattery, flagSSD, flagWiFi, flagBIOS bool
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show detailed system hardware information",
	Example: `  dira info                  # all sections
  dira info --cpu            # CPU only
  dira info --gpu --ssd      # GPU + SSD only`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// if no flags → show all
		showAll := !flagCPU && !flagGPU && !flagRAM && !flagBattery && !flagSSD && !flagWiFi && !flagBIOS

		fmt.Println(infoDivider.Render("Gathering info..."))
		s := info.Gather()
		out := ""

		if showAll || flagBIOS {
			out += section("System")
			out += infoRow("Device", infoValue.Render(s.Product))
			out += infoRow("BIOS", infoValue.Render(fmt.Sprintf("%s  ver %s  (%s)", s.BIOS.Vendor, s.BIOS.Version, s.BIOS.Released)))
		}

		if showAll || flagCPU {
			c := s.CPU
			out += section("CPU")
			out += infoRow("Model", infoValue.Render(c.Model))
			out += infoRow("Cores / Threads", infoValue.Render(fmt.Sprintf("%d cores / %d threads", c.Cores, c.Threads)))
			out += infoRow("Base clock", infoValue.Render(fmt.Sprintf("%d MHz", c.BaseMHz)))
			out += infoRow("Max clock (turbo)", infoValue.Render(fmt.Sprintf("%d MHz", c.MaxMHz)))
			out += infoRow("Min clock", infoValue.Render(fmt.Sprintf("%d MHz", c.MinMHz)))
			out += infoRow("Current avg clock", infoValue.Render(fmt.Sprintf("%.0f MHz", c.CurMHz)))
			out += infoRow("Governor", infoValue.Render(c.Governor))
			out += infoRow("TDP  PL1 / PL2", infoValue.Render(fmt.Sprintf("%dW / %dW", c.PL1Watts, c.PL2Watts)))
			out += infoRow("Cache L1d / L1i", infoValue.Render(fmt.Sprintf("%s / %s", c.L1d, c.L1i)))
			out += infoRow("Cache L2 / L3", infoValue.Render(fmt.Sprintf("%s / %s", c.L2, c.L3)))
			out += infoRow("Package temp", tempStr(c.TempPkg, c.TempMaxC-10, c.TempCritC))
			out += infoRow("Max / Crit temp", infoValue.Render(fmt.Sprintf("%d°C / %d°C", c.TempMaxC, c.TempCritC)))
			out += infoRow("CPU fan", infoValue.Render(fmt.Sprintf("%d RPM", c.FanRPM)))
		}

		if showAll || flagGPU {
			for _, g := range s.GPUs {
				label := "GPU"
				if g.IsIntegrated {
					label = "GPU (integrated)"
				}
				out += section(label)
				out += infoRow("Model", infoValue.Render(g.Name))
				if !g.IsIntegrated {
					out += infoRow("Driver / VBIOS", infoValue.Render(fmt.Sprintf("%s / %s", g.DriverVersion, g.VBIOSVersion)))
					out += infoRow("VRAM", infoValue.Render(fmt.Sprintf("%d MB total  |  used %d MB  |  free %d MB", g.MemTotalMB, g.MemUsedMB, g.MemFreeMB)))
					out += infoRow("Max GPU clock", infoValue.Render(fmt.Sprintf("%d MHz", g.MaxClockMHz)))
					out += infoRow("Max VRAM clock", infoValue.Render(fmt.Sprintf("%d MHz", g.MaxMemMHz)))
					out += infoRow("Current clock", infoValue.Render(fmt.Sprintf("%d MHz", g.CurClockMHz)))
					out += infoRow("GPU temp", tempStr(g.TempC, g.MaxTempC-10, g.MaxTempC))
					maxTempStr := "N/A"
					if g.MaxTempC > 0 {
						maxTempStr = fmt.Sprintf("%d°C (slowdown threshold)", g.MaxTempC)
					}
					out += infoRow("Max temp", infoValue.Render(maxTempStr))
					out += infoRow("Fan speed", infoValue.Render(g.FanPct+"%"))
					out += infoRow("Power state", infoValue.Render(g.PState))
				}
			}
		}

		if showAll || flagRAM {
			out += section("RAM")
			for i, r := range s.RAM {
				out += infoRow(fmt.Sprintf("Slot %d", i+1), infoValue.Render(fmt.Sprintf("%d GiB  %s  |  %s  |  %s  |  %s", r.SizeGiB, r.TypeSpeed, r.Mfr, r.Part, r.VoltageV)))
			}
		}

		if showAll || flagSSD {
			if s.SSD != nil {
				d := s.SSD
				out += section("SSD (NVMe)")
				out += infoRow("Model", infoValue.Render(d.Model))
				out += infoRow("Serial", infoValue.Render(d.Serial))
				out += infoRow("Firmware", infoValue.Render(d.Firmware))
				out += infoRow("Capacity", infoValue.Render(fmt.Sprintf("%d GB", d.CapacityGB)))
				out += infoRow("PCIe link", infoValue.Render(d.PCIeSpeed))
				out += infoRow("Health", healthStr(d.HealthStatus))
				out += infoRow("Wear level", wearStr(d.WearPct))
				out += infoRow("Spare available", infoValue.Render(fmt.Sprintf("%d%%  (threshold %d%%)", d.SpareAvailPct, d.SpareThreshPct)))
				out += infoRow("Temperature", tempStr(d.TempC, d.WarnTempC, d.CritTempC))
				out += infoRow("Warn / Crit temp", infoValue.Render(fmt.Sprintf("%d°C / %d°C", d.WarnTempC, d.CritTempC)))
				out += infoRow("Data read", infoValue.Render(fmt.Sprintf("%.0f TB", d.ReadTB)))
				out += infoRow("Data written", infoValue.Render(fmt.Sprintf("%.0f TB", d.WrittenTB)))
				out += infoRow("Power on hours", infoValue.Render(fmt.Sprintf("%d h  (%.1f years)", d.PowerOnHours, float64(d.PowerOnHours)/8760)))
				out += infoRow("Power cycles", infoValue.Render(fmt.Sprintf("%d", d.PowerCycles)))
				out += infoRow("Unsafe shutdowns", unsafeStr(d.UnsafeShuts))
				out += infoRow("Media errors", errStr(d.MediaErrors))
			}
		}

		if showAll || flagBattery {
			if s.Battery != nil {
				b := s.Battery
				out += section("Battery")
				out += infoRow("Manufacturer", infoValue.Render(b.Manufacturer))
				out += infoRow("Model", infoValue.Render(b.Model))
				out += infoRow("Technology", infoValue.Render(b.Technology))
				out += infoRow("Design capacity", infoValue.Render(fmt.Sprintf("%d mAh", b.ChargeDesign)))
				out += infoRow("Full charge", infoValue.Render(fmt.Sprintf("%d mAh", b.ChargeFull)))
				out += infoRow("Battery health", battHealthStr(b.HealthPct))
				cycleStr := "N/A (not reported by hardware)"
				if b.CycleCount > 0 {
					cycleStr = fmt.Sprintf("%d", b.CycleCount)
				}
				out += infoRow("Cycle count", infoValue.Render(cycleStr))
				out += infoRow("Voltage now", infoValue.Render(fmt.Sprintf("%.3f V", b.VoltageV)))
				if b.TempC > 0 {
					out += infoRow("Battery area temp", tempStr(int(b.TempC), 40, 50))
				}
			}
		}

		if showAll || flagWiFi {
			if s.WiFi != nil {
				w := s.WiFi
				out += section("WiFi")
				out += infoRow("Card", infoValue.Render(w.Name))
				out += infoRow("Driver", infoValue.Render(w.Driver))
				out += infoRow("Interface", infoValue.Render(w.Interface))
				out += infoRow("MAC", infoValue.Render(w.MAC))
				if w.TempC > 0 {
					out += infoRow("Temperature", tempStr(w.TempC, 70, 85))
				}
				if w.LinkMbps != "" {
					out += infoRow("Link rate", infoValue.Render(w.LinkMbps+" Mb/s"))
				}
				if w.FreqGHz != "" {
					out += infoRow("Frequency", infoValue.Render(w.FreqGHz))
				}
				if w.Signal != "" {
					out += infoRow("Signal", infoValue.Render(w.Signal))
				}
			}
		}

		fmt.Print(out)
		return nil
	},
}

func tempStr(t, warn, crit int) string {
	s := fmt.Sprintf("%d°C", t)
	switch {
	case t >= crit:
		return infoBad.Render(s + "  ⚠ CRITICAL")
	case t >= warn:
		return infoWarn.Render(s + "  ⚠ hot")
	default:
		return infoGood.Render(s)
	}
}

func wearStr(pct int) string {
	s := fmt.Sprintf("%d%% used", pct)
	switch {
	case pct >= 90:
		return infoBad.Render(s + "  ⚠ replace soon")
	case pct >= 70:
		return infoWarn.Render(s)
	default:
		return infoGood.Render(s)
	}
}

func healthStr(status string) string {
	if strings.Contains(strings.ToUpper(status), "PASSED") {
		return infoGood.Render(status)
	}
	return infoBad.Render(status)
}

func battHealthStr(pct int) string {
	s := fmt.Sprintf("%d%%", pct)
	switch {
	case pct >= 80:
		return infoGood.Render(s)
	case pct >= 60:
		return infoWarn.Render(s + "  degraded")
	default:
		return infoBad.Render(s + "  ⚠ replace recommended")
	}
}

func unsafeStr(n int) string {
	if n > 500 {
		return infoWarn.Render(fmt.Sprintf("%d  (high — avoid forced shutdowns)", n))
	}
	return infoValue.Render(fmt.Sprintf("%d", n))
}

func errStr(n int) string {
	if n > 0 {
		return infoBad.Render(fmt.Sprintf("%d  ⚠ errors detected", n))
	}
	return infoGood.Render("0  clean")
}

func init() {
	infoCmd.Flags().BoolVar(&flagCPU, "cpu", false, "Show CPU info only")
	infoCmd.Flags().BoolVar(&flagGPU, "gpu", false, "Show GPU info only")
	infoCmd.Flags().BoolVar(&flagRAM, "ram", false, "Show RAM info only")
	infoCmd.Flags().BoolVar(&flagBattery, "battery", false, "Show battery info only")
	infoCmd.Flags().BoolVar(&flagSSD, "ssd", false, "Show SSD info only")
	infoCmd.Flags().BoolVar(&flagWiFi, "wifi", false, "Show WiFi info only")
	infoCmd.Flags().BoolVar(&flagBIOS, "bios", false, "Show BIOS/system info only")
}
