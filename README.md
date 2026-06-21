# dira

A Linux system helper tool. Currently provides keyboard RGB, brightness, and LED state controls for ASUS TUF laptops.

## Install

```bash
git clone <repo>
cd dira
go install
```

> Requires `python3` and `sudo` for writing to sysfs.

## Commands

### `keycolor`
Set keyboard backlight color.

```bash
dira keycolor ff0000          # hex color (no # needed)
dira keycolor 255 0 0         # RGB values
dira keycolor ff0000 -m       # + pick animation mode via TUI
dira keycolor ff0000 -s       # + pick animation speed via TUI
dira keycolor ff0000 -m -s    # + both
dira keycolor ff0000 --save   # persist to BIOS
```

### `keymode`
Interactively pick and apply animation mode.

```bash
dira keymode
# options: static | breathing | cycle
```

### `keyspeed`
Interactively pick and apply animation speed.

```bash
dira keyspeed
# options: slow | med | fast
```

### `keylight`
Interactively pick keyboard brightness level.

```bash
dira keylight
# options: 0 (off) | 1 (low) | 2 (medium) | 3 (high)
```

### `keystate`
Interactively set keyboard LED on/off per power state.

```bash
dira keystate
# sets: boot | awake | sleep | keyboard
```

## TUI Navigation

```
↑ / k   move up
↓ / j   move down
enter   select
q       cancel
```

## License

MIT
