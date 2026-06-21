package info

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ── Types ────────────────────────────────────────────────────────────────────

type CoreTemp struct {
	Label   string
	TempC   int
	MaxC    int
	CritC   int
}

type CPUInfo struct {
	Model     string
	Cores     int
	Threads   int
	BaseMHz   int
	MaxMHz    int
	MinMHz    int
	CurMHz    float64
	L1d, L1i string
	L2, L3   string
	Governor  string
	TempPkg   int
	TempMaxC  int
	TempCritC int
	Cores_    []CoreTemp
	FanRPM    int
	PL1Watts  int
	PL2Watts  int
}

type GPUInfo struct {
	Name          string
	DriverVersion string
	VBIOSVersion  string
	MemTotalMB    int
	MemUsedMB     int
	MemFreeMB     int
	MaxClockMHz   int
	MaxMemMHz     int
	CurClockMHz   int
	TempC         int
	MaxTempC      int // from nvidia-smi slowdown threshold
	FanPct        string
	PState        string
	IsIntegrated  bool
}

type RAMSlot struct {
	SizeGiB  int
	TypeSpeed string
	Mfr      string
	Part     string
	VoltageV string
}

type BatteryInfo struct {
	Manufacturer string
	Model        string
	Technology   string
	HealthPct    int
	CycleCount   int
	ChargeFull   int
	ChargeDesign int
	VoltageV     float64
	TempC        float64
}

type SSDInfo struct {
	Model          string
	Serial         string
	Firmware       string
	CapacityGB     int
	TempC          int
	WarnTempC      int
	CritTempC      int
	HealthStatus   string
	WearPct        int
	SpareAvailPct  int
	SpareThreshPct int
	ReadTB         float64
	WrittenTB      float64
	PowerOnHours   int
	PowerCycles    int
	UnsafeShuts    int
	MediaErrors    int
	PCIeSpeed      string // e.g. "8GT/s x4"
}

type WiFiInfo struct {
	Name      string
	Driver    string
	Interface string
	MAC       string
	TempC     int
	LinkMbps  string
	FreqGHz   string
	Signal    string
}

type BIOSInfo struct {
	Vendor   string
	Version  string
	Released string
}

type SystemInfo struct {
	CPU     CPUInfo
	GPUs    []GPUInfo
	RAM     []RAMSlot
	Battery *BatteryInfo
	SSD     *SSDInfo
	WiFi    *WiFiInfo
	BIOS    BIOSInfo
	Product string
}

// ── Gather ───────────────────────────────────────────────────────────────────

func Gather() SystemInfo {
	var s SystemInfo
	s.CPU = gatherCPU()
	s.GPUs = gatherGPUs()
	s.RAM = gatherRAM()
	s.Battery = gatherBattery()
	s.SSD = gatherSSD()
	s.WiFi = gatherWiFi()
	s.BIOS = gatherBIOS()
	s.Product = readDMI("system", "Product Name")
	return s
}

func gatherCPU() CPUInfo {
	c := CPUInfo{}
	c.Model = lscpuField("Model name")
	c.Cores, _ = strconv.Atoi(lscpuField("Core(s) per socket"))
	threads, _ := strconv.Atoi(lscpuField("Thread(s) per core"))
	c.Threads = c.Cores * threads
	c.MaxMHz = readSysInt("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq") / 1000
	c.MinMHz = readSysInt("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_min_freq") / 1000
	c.CurMHz = avgCPUMHz()
	c.BaseMHz = baseClockMHz(c.Model)
	c.L1d = lscpuField("L1d cache")
	c.L1i = lscpuField("L1i cache")
	c.L2 = lscpuField("L2 cache")
	c.L3 = lscpuField("L3 cache")
	c.Governor = readSysTrim("/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor")
	c.PL1Watts = readSysInt("/sys/class/powercap/intel-rapl:0/constraint_0_power_limit_uw") / 1_000_000
	c.PL2Watts = readSysInt("/sys/class/powercap/intel-rapl:0/constraint_1_power_limit_uw") / 1_000_000

	// find coretemp hwmon
	hwmon := findHwmon("coretemp")
	if hwmon != "" {
		c.TempPkg = readSysInt(hwmon+"/temp1_input") / 1000
		c.TempMaxC = readSysInt(hwmon+"/temp1_max") / 1000
		c.TempCritC = readSysInt(hwmon+"/temp1_crit") / 1000
		for i := 2; ; i++ {
			tin := readSysInt(fmt.Sprintf("%s/temp%d_input", hwmon, i))
			if tin == 0 {
				break
			}
			c.Cores_ = append(c.Cores_, CoreTemp{
				Label: readSysTrim(fmt.Sprintf("%s/temp%d_label", hwmon, i)),
				TempC: tin / 1000,
				MaxC:  readSysInt(fmt.Sprintf("%s/temp%d_max", hwmon, i)) / 1000,
				CritC: readSysInt(fmt.Sprintf("%s/temp%d_crit", hwmon, i)) / 1000,
			})
		}
	}

	asusHwmon := findHwmon("asus")
	if asusHwmon != "" {
		c.FanRPM = readSysInt(asusHwmon + "/fan1_input")
	}
	return c
}

func gatherGPUs() []GPUInfo {
	var gpus []GPUInfo

	// NVIDIA discrete GPUs
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=name,driver_version,vbios_version,memory.total,memory.used,memory.free,clocks.max.gr,clocks.max.mem,clocks.gr,temperature.gpu,temperature.gpu.tlimit,fan.speed,pstate",
		"--format=csv,noheader,nounits").Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			p := splitCSV(line)
			if len(p) < 13 {
				continue
			}
			g := GPUInfo{}
			g.Name = p[0]
			g.DriverVersion = p[1]
			g.VBIOSVersion = p[2]
			g.MemTotalMB, _ = strconv.Atoi(p[3])
			g.MemUsedMB, _ = strconv.Atoi(p[4])
			g.MemFreeMB, _ = strconv.Atoi(p[5])
			g.MaxClockMHz, _ = strconv.Atoi(p[6])
			g.MaxMemMHz, _ = strconv.Atoi(p[7])
			g.CurClockMHz, _ = strconv.Atoi(p[8])
			g.TempC, _ = strconv.Atoi(p[9])
			g.MaxTempC, _ = strconv.Atoi(p[10])
			if g.MaxTempC == 0 {
				g.MaxTempC = 95 // default for laptop GPUs where tlimit is N/A
			}
			g.FanPct = p[11]
			g.PState = p[12]
			gpus = append(gpus, g)
		}
	}

	// Intel integrated GPU (i915)
	if name := intelGPUName(); name != "" {
		ig := GPUInfo{Name: name, IsIntegrated: true}
		gpus = append(gpus, ig)
	}

	return gpus
}

func gatherRAM() []RAMSlot {
	out, err := exec.Command("sudo", "dmidecode", "-t", "memory").Output()
	if err != nil {
		return nil
	}
	var slots []RAMSlot
	var cur *RAMSlot
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "Memory Device" {
			if cur != nil && cur.SizeGiB > 0 {
				slots = append(slots, *cur)
			}
			cur = &RAMSlot{}
			continue
		}
		if cur == nil {
			continue
		}
		kv := strings.SplitN(line, ": ", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := kv[0], strings.TrimSpace(kv[1])
		switch k {
		case "Size":
			if strings.Contains(v, "GiB") {
				cur.SizeGiB, _ = strconv.Atoi(strings.Fields(v)[0])
			}
		case "Type":
			if v != "Unknown" && cur.TypeSpeed == "" {
				cur.TypeSpeed = v
			}
		case "Speed":
			if v != "Unknown" {
				cur.TypeSpeed += " " + v
			}
		case "Manufacturer":
			cur.Mfr = v
		case "Part Number":
			cur.Part = strings.TrimSpace(v)
		case "Configured Voltage":
			cur.VoltageV = v
		}
	}
	if cur != nil && cur.SizeGiB > 0 {
		slots = append(slots, *cur)
	}
	return slots
}

func gatherBattery() *BatteryInfo {
	base := "/sys/class/power_supply/BAT1"
	if _, err := os.Stat(base); err != nil {
		return nil
	}
	b := &BatteryInfo{}
	b.Manufacturer = strings.TrimSpace(readSysTrim(base + "/manufacturer"))
	b.Model = readSysTrim(base + "/model_name")
	b.Technology = readSysTrim(base + "/technology")
	b.CycleCount = readSysInt(base + "/cycle_count")
	b.ChargeFull = readSysInt(base+"/charge_full") / 1000
	b.ChargeDesign = readSysInt(base+"/charge_full_design") / 1000
	b.VoltageV = float64(readSysInt(base+"/voltage_now")) / 1_000_000
	if b.ChargeDesign > 0 {
		b.HealthPct = b.ChargeFull * 100 / b.ChargeDesign
	}
	// acpitz temp2 is battery area temp
	hw := findHwmon("acpitz")
	if hw != "" {
		t := readSysInt(hw + "/temp2_input")
		if t > 0 {
			b.TempC = float64(t) / 1000
		}
	}
	return b
}

func gatherSSD() *SSDInfo {
	out, err := exec.Command("sudo", "smartctl", "-a", "/dev/nvme0").Output()
	if err != nil {
		return nil
	}
	s := &SSDInfo{}

	// hwmon for NVMe
	hw := findHwmon("nvme")
	if hw != "" {
		s.TempC = readSysInt(hw+"/temp1_input") / 1000
		s.WarnTempC = readSysInt(hw+"/temp1_max") / 1000
		s.CritTempC = readSysInt(hw+"/temp1_crit") / 1000
	}

	// PCIe link speed
	s.PCIeSpeed = nvmePCIeSpeed()

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		switch k {
		case "Model Number":
			s.Model = v
		case "Serial Number":
			s.Serial = v
		case "Firmware Version":
			s.Firmware = v
		case "Namespace 1 Size/Capacity":
			if idx := strings.Index(v, "["); idx >= 0 {
				gb := strings.TrimSuffix(strings.TrimSpace(v[idx+1:strings.Index(v, "]")]), " GB")
				s.CapacityGB, _ = strconv.Atoi(strings.TrimSpace(gb))
			}
		case "SMART overall-health self-assessment test result":
			s.HealthStatus = v
		case "Percentage Used":
			fmt.Sscanf(v, "%d", &s.WearPct)
		case "Available Spare":
			fmt.Sscanf(v, "%d", &s.SpareAvailPct)
		case "Available Spare Threshold":
			fmt.Sscanf(v, "%d", &s.SpareThreshPct)
		case "Data Units Read":
			s.ReadTB = parseTB(v)
		case "Data Units Written":
			s.WrittenTB = parseTB(v)
		case "Power On Hours":
			fmt.Sscanf(strings.ReplaceAll(v, ",", ""), "%d", &s.PowerOnHours)
		case "Power Cycles":
			fmt.Sscanf(strings.ReplaceAll(v, ",", ""), "%d", &s.PowerCycles)
		case "Unsafe Shutdowns":
			fmt.Sscanf(strings.ReplaceAll(v, ",", ""), "%d", &s.UnsafeShuts)
		case "Media and Data Integrity Errors":
			fmt.Sscanf(v, "%d", &s.MediaErrors)
		}
	}
	return s
}

func gatherWiFi() *WiFiInfo {
	// find WiFi interface
	iface := ""
	out, _ := exec.Command("iw", "dev").Output()
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Interface ") {
			iface = strings.TrimPrefix(line, "Interface ")
		}
	}
	if iface == "" {
		return nil
	}

	w := &WiFiInfo{Interface: iface}

	// card name from lspci
	lspciOut, _ := exec.Command("lspci").Output()
	for _, line := range strings.Split(string(lspciOut), "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "network") || strings.Contains(lower, "wireless") {
			// strip PCI address
			if idx := strings.Index(line, ": "); idx >= 0 {
				w.Name = strings.TrimSpace(line[idx+2:])
			}
			break
		}
	}

	// driver
	driverLink, _ := os.Readlink(fmt.Sprintf("/sys/class/net/%s/device/driver", iface))
	w.Driver = filepath.Base(driverLink)

	// MAC
	w.MAC = readSysTrim(fmt.Sprintf("/sys/class/net/%s/address", iface))

	// temp from mt7921_phy0 hwmon
	hw := findHwmon("mt7921_phy0")
	if hw != "" {
		w.TempC = readSysInt(hw+"/temp1_input") / 1000
	}

	// current link rate, freq, signal via iwconfig
	iwOut, _ := exec.Command("iwconfig", iface).Output()
	for _, line := range strings.Split(string(iwOut), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Bit Rate=") {
			if idx := strings.Index(line, "Bit Rate="); idx >= 0 {
				part := line[idx+9:]
				if sp := strings.Index(part, " "); sp >= 0 {
					w.LinkMbps = part[:sp]
				}
			}
		}
		if strings.Contains(line, "Frequency:") {
			if idx := strings.Index(line, "Frequency:"); idx >= 0 {
				part := line[idx+10:]
				if sp := strings.Index(part, " "); sp >= 0 {
					w.FreqGHz = part[:sp] + " GHz"
				}
			}
		}
		if strings.Contains(line, "Signal level=") {
			if idx := strings.Index(line, "Signal level="); idx >= 0 {
				part := line[idx+13:]
				if sp := strings.Index(part, " "); sp >= 0 {
					w.Signal = part[:sp] + " dBm"
				}
			}
		}
	}

	return w
}

func gatherBIOS() BIOSInfo {
	return BIOSInfo{
		Vendor:   readDMI("bios", "Vendor"),
		Version:  readDMI("bios", "Version"),
		Released: readDMI("bios", "Release Date"),
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func findHwmon(name string) string {
	matches, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, d := range matches {
		if readSysTrim(d+"/name") == name {
			return d
		}
	}
	return ""
}

func intelGPUName() string {
	out, err := exec.Command("lspci").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		lower := strings.ToLower(line)
		if (strings.Contains(lower, "vga") || strings.Contains(lower, "display")) &&
			strings.Contains(lower, "intel") {
			if idx := strings.Index(line, ": "); idx >= 0 {
				return strings.TrimSpace(line[idx+2:])
			}
		}
	}
	return ""
}

func nvmePCIeSpeed() string {
	out, err := exec.Command("sudo", "lspci", "-vv").Output()
	if err != nil {
		return ""
	}
	inNVMe := false
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "Non-Volatile") {
			inNVMe = true
		}
		if inNVMe && strings.Contains(line, "LnkSta:") {
			// "LnkSta: Speed 8GT/s, Width x4"
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "LnkSta:")
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func readSysInt(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return v
}

func readSysTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func lscpuField(key string) string {
	out, err := exec.Command("lscpu").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, key) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func avgCPUMHz() float64 {
	total, count := 0.0, 0
	matches, _ := filepath.Glob("/sys/devices/system/cpu/cpu*/cpufreq/scaling_cur_freq")
	for _, f := range matches {
		v := readSysInt(f)
		if v > 0 {
			total += float64(v) / 1000
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func baseClockMHz(model string) int {
	if idx := strings.Index(model, "@ "); idx >= 0 {
		part := strings.TrimSuffix(model[idx+2:], "GHz")
		var f float64
		fmt.Sscanf(part, "%f", &f)
		return int(f * 1000)
	}
	return 0
}

func readDMI(dtype, field string) string {
	out, err := exec.Command("sudo", "dmidecode", "-t", dtype).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, field+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, field+":"))
		}
	}
	return ""
}

func splitCSV(s string) []string {
	parts := strings.Split(strings.TrimSpace(s), ", ")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func parseTB(s string) float64 {
	if idx := strings.Index(s, "["); idx >= 0 {
		inner := s[idx+1 : strings.Index(s, "]")]
		inner = strings.TrimSuffix(strings.TrimSpace(inner), " TB")
		v, _ := strconv.ParseFloat(strings.TrimSpace(inner), 64)
		return v
	}
	return 0
}
