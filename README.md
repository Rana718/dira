# dira

A Linux system helper tool for ASUS TUF laptops (and compatible hardware).

## Install

```bash
git clone <repo>
cd dira
go install
```

> Requires `python3`, `sudo`, `smartctl`, `nvidia-smi`, `cpupower` for full functionality.

## Commands

### `keycolor`
Set keyboard backlight color. Remembers last color per mode.

```bash
dira keycolor ff0000          # hex (no # needed)
dira keycolor 255 0 0         # RGB values
dira keycolor ff0000 -m       # + pick mode via TUI
dira keycolor ff0000 -s       # + pick speed via TUI
dira keycolor ff0000 -m -s    # + both
dira keycolor ff0000 --save   # persist to BIOS
```

### `keymode`
Pick animation mode and apply with last saved color for that mode.

```bash
dira keymode
# options: static | breathing | cycle
```

### `keyspeed`
Pick animation speed.

```bash
dira keyspeed
# options: slow | med | fast
```

### `keylight`
Pick keyboard brightness.

```bash
dira keylight
# options: 0 (off) | 1 (low) | 2 (medium) | 3 (high)
```

### `keystate`
Set keyboard LED on/off per power state.

```bash
dira keystate
# sets: boot | awake | sleep | keyboard
```

### `power`
Interactive power profile manager with live system stats.

```bash
dira power
# profiles: performance | balanced | underclock | power-saver | custom
```

Shows CPU/GPU clocks, TDP, temps, and fan speeds alongside each profile's config.
Custom profiles are saved to SQLite and persist across sessions.

### `info`
Show detailed hardware information.

```bash
dira info                 # all sections
dira info --cpu           # CPU only
dira info --gpu           # GPU only
dira info --ram           # RAM only
dira info --ssd           # SSD only
dira info --battery       # battery only
dira info --wifi          # WiFi only
dira info --bios          # system/BIOS only
```

Covers: CPU clocks/temps/TDP, dual GPU support, VRAM, NVMe health/wear/PCIe speed, battery health %, WiFi card + temp, BIOS version.

## TUI Navigation

```
↑ / k   move up
↓ / j   move down
enter   select
q       cancel
```

## State

Per-mode keyboard colors and power profiles are stored in `~/.config/dira/state.db` (SQLite).

## License

MIT
