package power

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// ── Types ────────────────────────────────────────────────────────────────────

type Profile struct {
	Name      string
	CPUMaxMHz int // 0 = no limit
	PL1Watts  int // long-term TDP, 0 = no limit
	PL2Watts  int // short-term TDP, 0 = no limit
	GPUMaxMHz int // 0 = no limit
	GPUWatts  int // 0 = no limit
}

type SysInfo struct {
	CPUMinMHz   int
	CPUMaxMHz   int
	CPUCurMHz   int
	CPUGovernor string
	PL1Watts    int
	PL2Watts    int
	GPUName     string
	GPUCurMHz   int
	GPUMaxMHz   int
	GPUWatts    float64
	GPUMaxWatts float64
	GPUTemp     int
	CPUFanRPM   int
	GPUFanRPM   int
	CPUTempPkg  int
}

// ── Built-in profiles ────────────────────────────────────────────────────────

var BuiltinProfiles = []Profile{
	{
		Name:      "performance",
		CPUMaxMHz: 4500,
		PL1Watts:  45,
		PL2Watts:  60,
		GPUMaxMHz: 0,
		GPUWatts:  75,
	},
	{
		Name:      "balanced",
		CPUMaxMHz: 4500,
		PL1Watts:  35,
		PL2Watts:  50,
		GPUMaxMHz: 0,
		GPUWatts:  60,
	},
	{
		Name:      "underclock",
		CPUMaxMHz: 4000,
		PL1Watts:  25,
		PL2Watts:  35,
		GPUMaxMHz: 1500,
		GPUWatts:  50,
	},
	{
		Name:      "power-saver",
		CPUMaxMHz: 2000,
		PL1Watts:  15,
		PL2Watts:  20,
		GPUMaxMHz: 1000,
		GPUWatts:  30,
	},
}

// ── DB ───────────────────────────────────────────────────────────────────────

func dbPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "dira", "state.db")
}

func openDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath())
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS power_profiles (
			name        TEXT PRIMARY KEY,
			cpu_max_mhz INTEGER,
			pl1_watts   INTEGER,
			pl2_watts   INTEGER,
			gpu_max_mhz INTEGER,
			gpu_watts   INTEGER
		);
		CREATE TABLE IF NOT EXISTS power_active (
			id   INTEGER PRIMARY KEY CHECK(id=1),
			name TEXT NOT NULL DEFAULT 'balanced'
		);
		INSERT OR IGNORE INTO power_active(id,name) VALUES(1,'balanced');
	`)
	return db, err
}

func SaveCustomProfile(p Profile) error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`
		INSERT INTO power_profiles(name,cpu_max_mhz,pl1_watts,pl2_watts,gpu_max_mhz,gpu_watts)
		VALUES(?,?,?,?,?,?)
		ON CONFLICT(name) DO UPDATE SET
			cpu_max_mhz=?,pl1_watts=?,pl2_watts=?,gpu_max_mhz=?,gpu_watts=?`,
		p.Name, p.CPUMaxMHz, p.PL1Watts, p.PL2Watts, p.GPUMaxMHz, p.GPUWatts,
		p.CPUMaxMHz, p.PL1Watts, p.PL2Watts, p.GPUMaxMHz, p.GPUWatts,
	)
	return err
}

func LoadCustomProfiles() ([]Profile, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT name,cpu_max_mhz,pl1_watts,pl2_watts,gpu_max_mhz,gpu_watts FROM power_profiles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Profile
	for rows.Next() {
		var p Profile
		if err := rows.Scan(&p.Name, &p.CPUMaxMHz, &p.PL1Watts, &p.PL2Watts, &p.GPUMaxMHz, &p.GPUWatts); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func SetActiveProfile(name string) error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`UPDATE power_active SET name=? WHERE id=1`, name)
	return err
}

func GetActiveProfile() string {
	db, err := openDB()
	if err != nil {
		return "balanced"
	}
	defer db.Close()
	var name string
	if err := db.QueryRow(`SELECT name FROM power_active WHERE id=1`).Scan(&name); err != nil {
		return "balanced"
	}
	return name
}

// ── Apply ────────────────────────────────────────────────────────────────────

func runSudo(name string, args ...string) error {
	cmd := exec.Command("sudo", append([]string{name}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Apply(p Profile) error {
	if p.CPUMaxMHz > 0 {
		runSudo("cpupower", "frequency-set", "-u", fmt.Sprintf("%dMHz", p.CPUMaxMHz)) //nolint:errcheck
	}

	raplBase := "/sys/class/powercap/intel-rapl:0"
	if _, err := os.Stat(raplBase); err == nil {
		if p.PL1Watts > 0 {
			sudoWrite(raplBase+"/constraint_0_power_limit_uw", fmt.Sprintf("%d", p.PL1Watts*1_000_000))
		}
		if p.PL2Watts > 0 {
			sudoWrite(raplBase+"/constraint_1_power_limit_uw", fmt.Sprintf("%d", p.PL2Watts*1_000_000))
		}
	}

	if p.GPUMaxMHz > 0 {
		runSudo("nvidia-smi", fmt.Sprintf("--lock-gpu-clocks=1100,%d", p.GPUMaxMHz)) //nolint:errcheck
	} else {
		runSudo("nvidia-smi", "--reset-gpu-clocks") //nolint:errcheck
	}
	if p.GPUWatts > 0 {
		_ = runSudo("nvidia-smi", fmt.Sprintf("--power-limit=%d", p.GPUWatts))
	}

	_ = SetActiveProfile(p.Name)
	return nil
}

// ── SysInfo ──────────────────────────────────────────────────────────────────

func ReadSysInfo() SysInfo {
	var s SysInfo

	s.CPUMinMHz = readIntFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_min_freq") / 1000
	s.CPUMaxMHz = readIntFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq") / 1000
	s.CPUCurMHz = readIntFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq") / 1000
	s.CPUGovernor = readStrFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor")
	s.PL1Watts = readIntFile("/sys/class/powercap/intel-rapl:0/constraint_0_power_limit_uw") / 1_000_000
	s.PL2Watts = readIntFile("/sys/class/powercap/intel-rapl:0/constraint_1_power_limit_uw") / 1_000_000
	s.CPUTempPkg = readIntFile(findHwmon("coretemp")+"/temp1_input") / 1000
	s.CPUFanRPM = readIntFile(findHwmon("asus")+"/fan1_input")
	s.GPUFanRPM = readIntFile(findHwmon("asus")+"/fan2_input")

	out, err := exec.Command("nvidia-smi",
		"--query-gpu=name,clocks.gr,clocks.max.gr,power.draw,power.max_limit,temperature.gpu",
		"--format=csv,noheader,nounits").Output()
	if err == nil {
		parts := strings.Split(strings.TrimSpace(string(out)), ", ")
		if len(parts) == 6 {
			s.GPUName = parts[0]
			s.GPUCurMHz, _ = strconv.Atoi(parts[1])
			s.GPUMaxMHz, _ = strconv.Atoi(parts[2])
			s.GPUWatts, _ = strconv.ParseFloat(parts[3], 64)
			s.GPUWatts = math.Round(s.GPUWatts*10) / 10
			s.GPUMaxWatts, _ = strconv.ParseFloat(parts[4], 64)
			s.GPUTemp, _ = strconv.Atoi(parts[5])
		}
	}

	return s
}

// ── helpers ──────────────────────────────────────────────────────────────────

func readIntFile(path string) int {
	if path == "" {
		return 0
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return v
}

func readStrFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func findHwmon(name string) string {
	matches, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, d := range matches {
		if readStrFile(d+"/name") == name {
			return d
		}
	}
	return ""
}

func sudoWrite(path, value string) {
	script := fmt.Sprintf("import os;fd=os.open(%q,os.O_WRONLY);os.write(fd,%q.encode());os.close(fd)", path, value)
	cmd := exec.Command("sudo", "python3", "-c", script)
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
