package disk

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Mount struct {
	Device string
	Size   string
	Used   string
	Avail  string
	UsePct int
	Mount  string   // primary mount + "\x00" separated extras
	FSType string
}

type BlockDevice struct {
	Name    string
	Size    string
	DevType string
	FSType  string
	Mount   string
	Model   string
	Tran    string
	Rota    bool
}

type SSDHealth struct {
	Device       string
	Model        string
	Health       string
	TempC        int
	WarnTempC    int
	CritTempC    int
	WearPct      int
	SpareAvail   string
	SpareThresh  string
	DataRead     string
	DataWritten  string
	PowerOnHours string
	PowerCycles  string
	UnsafeShuts  int
	MediaErrors  int
}

func GetMounts() []Mount {
	out, err := exec.Command("df", "-h", "--output=source,size,used,avail,pcent,target,fstype").Output()
	if err != nil {
		return nil
	}
	skip := map[string]bool{
		"tmpfs": true, "devtmpfs": true, "efivarfs": true,
		"none": true, "overlay": true, "squashfs": true,
	}
	type group struct {
		entry  Mount
		extras []string
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
		if skip[fields[6]] {
			continue
		}
		pct, _ := strconv.Atoi(strings.TrimSuffix(fields[4], "%"))
		dev := fields[0]
		if g, ok := byDevice[dev]; ok {
			g.extras = append(g.extras, fields[5])
		} else {
			byDevice[dev] = &group{entry: Mount{
				Device: dev, Size: fields[1], Used: fields[2],
				Avail: fields[3], UsePct: pct, Mount: fields[5], FSType: fields[6],
			}}
			order = append(order, dev)
		}
	}
	var entries []Mount
	for _, dev := range order {
		g := byDevice[dev]
		if len(g.extras) > 0 {
			g.entry.Mount = g.entry.Mount + "\x00" + strings.Join(g.extras, "\x00")
		}
		entries = append(entries, g.entry)
	}
	return entries
}

func GetBlockDevices() []BlockDevice {
	out, err := exec.Command("lsblk", "-o", "NAME,SIZE,TYPE,FSTYPE,MOUNTPOINT,MODEL,ROTA,TRAN",
		"--noheadings", "--pairs").Output()
	if err != nil {
		return nil
	}
	var devices []BlockDevice
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		b := BlockDevice{}
		for _, pair := range strings.Fields(scanner.Text()) {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) != 2 {
				continue
			}
			val := strings.Trim(kv[1], `"`)
			switch kv[0] {
			case "NAME":
				b.Name = val
			case "SIZE":
				b.Size = val
			case "TYPE":
				b.DevType = val
			case "FSTYPE":
				b.FSType = val
			case "MOUNTPOINT":
				b.Mount = val
			case "MODEL":
				b.Model = strings.TrimSpace(val)
			case "ROTA":
				b.Rota = val == "1"
			case "TRAN":
				b.Tran = val
			}
		}
		if b.DevType == "disk" || b.DevType == "part" {
			devices = append(devices, b)
		}
	}
	return devices
}

func GetSSDHealth() []SSDHealth {
	lsOut, err := exec.Command("lsblk", "-o", "NAME,TYPE", "--noheadings").Output()
	if err != nil {
		return nil
	}
	var drives []string
	scanner := bufio.NewScanner(strings.NewReader(string(lsOut)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[1] == "disk" {
			drives = append(drives, "/dev/"+strings.TrimLeft(fields[0], "└─├─"))
		}
	}

	var results []SSDHealth
	for _, dev := range drives {
		cmd := exec.Command("sudo", "smartctl", "-a", dev)
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		info := map[string]string{}
		sc := bufio.NewScanner(strings.NewReader(string(out)))
		for sc.Scan() {
			parts := strings.SplitN(sc.Text(), ":", 2)
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
		wear, _ := strconv.Atoi(strings.TrimSuffix(info["Percentage Used"], "%"))
		unsafe, _ := strconv.Atoi(strings.ReplaceAll(info["Unsafe Shutdowns"], ",", ""))
		media, _ := strconv.Atoi(info["Media and Data Integrity Errors"])
		warnT, _ := strconv.Atoi(strings.Fields(strings.TrimSuffix(info["Warning  Comp. Temp. Threshold"], " Celsius"))[0])
		critT, _ := strconv.Atoi(strings.Fields(strings.TrimSuffix(info["Critical Comp. Temp. Threshold"], " Celsius"))[0])
		var temp int
		fmt.Sscanf(info["Temperature"], "%d", &temp)

		results = append(results, SSDHealth{
			Device: dev, Model: model,
			Health:       info["SMART overall-health self-assessment test result"],
			TempC:        temp,
			WarnTempC:    warnT,
			CritTempC:    critT,
			WearPct:      wear,
			SpareAvail:   info["Available Spare"],
			SpareThresh:  info["Available Spare Threshold"],
			DataRead:     info["Data Units Read"],
			DataWritten:  info["Data Units Written"],
			PowerOnHours: info["Power On Hours"],
			PowerCycles:  info["Power Cycles"],
			UnsafeShuts:  unsafe,
			MediaErrors:  media,
		})
	}
	return results
}
