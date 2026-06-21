package power

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	raplPath = "/sys/class/powercap/intel-rapl:0"
	cpuGlob  = "/sys/devices/system/cpu/cpu*/cpufreq/energy_performance_preference"
)

func writeFile(path, value string) {
	os.WriteFile(path, []byte(value), 0644) //nolint — best effort
}

func cpuPref(pref string) {
	matches, _ := filepath.Glob(cpuGlob)
	for _, f := range matches {
		writeFile(f, pref)
	}
}

func rapl(pl1, pl2 string) {
	if _, err := os.Stat(raplPath); err != nil {
		return
	}
	writeFile(raplPath+"/constraint_0_power_limit_uw", pl1)
	writeFile(raplPath+"/constraint_1_power_limit_uw", pl2)
}

func run(name string, args ...string) error {
	return runCmd(name, args...)
}

// Apply sets underclocking profile: low power, full fans.
func Apply() error {
	fmt.Println("==> CPU governor → powersave")
	if err := run("cpupower", "frequency-set", "-g", "powersave"); err != nil {
		return err
	}
	fmt.Println("==> CPU max freq → 4.0 GHz")
	if err := run("cpupower", "frequency-set", "-u", "4GHz"); err != nil {
		return err
	}
	fmt.Println("==> CPU energy preference → power")
	cpuPref("power")

	fmt.Println("==> CPU TDP → PL1=35W PL2=45W")
	rapl("35000000", "45000000")

	fmt.Println("==> NVIDIA clocks → max 1500 MHz")
	if err := run("nvidia-smi", "--lock-gpu-clocks=1300,1500"); err != nil {
		return err
	}
	fmt.Println("==> NVIDIA power limit → 50W")
	if err := run("nvidia-smi", "--power-limit=50"); err != nil {
		return err
	}
	fmt.Println("==> Fan → 100%")
	_ = run("nvidia-settings",
		"-a", "[gpu:0]/GPUFanControlState=1",
		"-a", "[fan:0]/GPUTargetFanSpeed=100",
	)

	fmt.Println("\n==> underclock profile active!")
	fmt.Println("    CPU: max 4.0 GHz | powersave | PL1=35W PL2=45W")
	fmt.Println("    GPU: max 1500 MHz | 50W")
	fmt.Println("    Fan: 100%")
	return nil
}

// Reset restores default performance profile.
func Reset() error {
	fmt.Println("==> CPU governor → performance")
	if err := run("cpupower", "frequency-set", "-g", "performance"); err != nil {
		return err
	}
	if err := run("cpupower", "frequency-set", "-u", "4.5GHz"); err != nil {
		return err
	}
	fmt.Println("==> CPU energy preference → performance")
	cpuPref("performance")

	fmt.Println("==> CPU TDP → PL1=45W PL2=60W")
	rapl("45000000", "60000000")

	fmt.Println("==> NVIDIA clocks → reset")
	if err := run("nvidia-smi", "--reset-gpu-clocks"); err != nil {
		return err
	}
	fmt.Println("==> NVIDIA power limit → 60W")
	if err := run("nvidia-smi", "--power-limit=60"); err != nil {
		return err
	}
	fmt.Println("==> Fan → auto")
	_ = run("nvidia-settings", "-a", "[gpu:0]/GPUFanControlState=0")

	fmt.Println("\n==> All settings restored to defaults.")
	return nil
}
