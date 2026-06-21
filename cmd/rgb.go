package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const sysBase = "/sys/class/leds/asus::kbd_backlight"

var (
	modes       = []string{"static", "breathing", "cycle"}
	speeds      = []string{"slow", "med", "fast"}
	brightItems = []string{"0 - off", "1 - low", "2 - medium", "3 - high"}
	stateItems  = []string{"boot", "awake", "sleep", "keyboard"}

	modeVal  = map[string]int{"static": 0, "breathing": 1, "cycle": 2}
	speedVal = map[string]int{"slow": 0, "med": 1, "fast": 2}
)

// sysfs format: "<save> <mode> <R> <G> <B> <speed>"
func rgbVal(mode, r, g, b, speed, save int) string {
	return fmt.Sprintf("%d %d %d %d %d %d", save, mode, r, g, b, speed)
}

// sysWrite uses raw O_WRONLY via python3 — sysfs rejects O_TRUNC used by tee/os.WriteFile
func sysWrite(path, value string) error {
	script := fmt.Sprintf("import os;fd=os.open(%q,os.O_WRONLY);os.write(fd,%q.encode());os.close(fd)", path, value)
	cmd := exec.Command("sudo", "python3", "-c", script)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func parseHex(h string) (r, g, b uint8, err error) {
	h = strings.TrimPrefix(h, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex %q", h)
	}
	d, e := hex.DecodeString(h)
	if e != nil {
		return 0, 0, 0, e
	}
	return d[0], d[1], d[2], nil
}

var keycolorCmd = &cobra.Command{
	Use:   "keycolor <hex | R G B>",
	Short: "Set keyboard backlight color",
	Example: `  dira keycolor ff0000
  dira keycolor 00ff80 -m
  dira keycolor ff0000 -m -s --save`,
	Args: cobra.RangeArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		var r, g, b uint8
		var err error

		if len(args) == 1 {
			r, g, b, err = parseHex(args[0])
			if err != nil {
				return err
			}
		} else if len(args) == 3 {
			vals := make([]uint64, 3)
			for i, a := range args {
				if vals[i], err = strconv.ParseUint(a, 10, 8); err != nil {
					return fmt.Errorf("invalid value %q", a)
				}
			}
			r, g, b = uint8(vals[0]), uint8(vals[1]), uint8(vals[2])
		} else {
			return fmt.Errorf("provide hex or R G B")
		}

		mode, speed, sv := "static", "med", 0

		if cmd.Flags().Changed("m") {
			if mode, err = pick("Animation mode", modes); err != nil {
				return err
			}
		}
		if cmd.Flags().Changed("s") {
			if speed, err = pick("Animation speed", speeds); err != nil {
				return err
			}
		}
		if cmd.Flags().Changed("save") {
			sv = 1
		}

		if err := sysWrite(sysBase+"/kbd_rgb_mode", rgbVal(modeVal[mode], int(r), int(g), int(b), speedVal[speed], sv)); err != nil {
			return err
		}
		fmt.Printf("color #%02x%02x%02x  mode=%s  speed=%s  save=%v\n", r, g, b, mode, speed, sv == 1)
		return nil
	},
}

var keymodeCmd = &cobra.Command{
	Use:   "keymode",
	Short: "Choose keyboard animation mode",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		chosen, err := pick("Animation mode", modes)
		if err != nil {
			return err
		}
		if err := sysWrite(sysBase+"/kbd_rgb_mode", rgbVal(modeVal[chosen], 255, 255, 255, 1, 0)); err != nil {
			return err
		}
		fmt.Printf("mode = %s\n", chosen)
		return nil
	},
}

var keyspeedCmd = &cobra.Command{
	Use:   "keyspeed",
	Short: "Choose keyboard animation speed",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		chosen, err := pick("Animation speed", speeds)
		if err != nil {
			return err
		}
		if err := sysWrite(sysBase+"/kbd_rgb_mode", rgbVal(1, 255, 255, 255, speedVal[chosen], 0)); err != nil {
			return err
		}
		fmt.Printf("speed = %s\n", chosen)
		return nil
	},
}

var keylightCmd = &cobra.Command{
	Use:   "keylight",
	Short: "Choose keyboard brightness level",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		chosen, err := pick("Brightness level", brightItems)
		if err != nil {
			return err
		}
		if err := sysWrite(sysBase+"/brightness", string(chosen[0])); err != nil {
			return err
		}
		fmt.Printf("brightness = %c\n", chosen[0])
		return nil
	},
}

var keystateCmd = &cobra.Command{
	Use:   "keystate",
	Short: "Choose keyboard LED state (boot/awake/sleep/kbd)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		vals := make([]int, 4)
		for i, field := range stateItems {
			chosen, err := pick("LED: "+field, []string{"on", "off"})
			if err != nil {
				return err
			}
			if chosen == "on" {
				vals[i] = 1
			}
		}
		val := fmt.Sprintf("%d %d %d %d", vals[0], vals[1], vals[2], vals[3])
		if err := sysWrite(sysBase+"/kbd_rgb_state", val); err != nil {
			return err
		}
		fmt.Printf("state: boot=%d awake=%d sleep=%d kbd=%d\n", vals[0], vals[1], vals[2], vals[3])
		return nil
	},
}

func init() {
	keycolorCmd.Flags().BoolP("m", "m", false, "Pick animation mode")
	keycolorCmd.Flags().BoolP("s", "s", false, "Pick animation speed")
	keycolorCmd.Flags().Bool("save", false, "Persist to BIOS")
}
