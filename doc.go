// Package dira is a Linux system helper tool for ASUS TUF laptops.
//
// It provides keyboard RGB control, power profile management,
// hardware info, disk health, systemd service management, container
// management (Docker/Podman), and open port scanning — all from one CLI.
//
// # Architecture (simple, no frameworks)
//
//   - CLI layer:      cmd/         → cobra commands, flag parsing, output formatting
//   - Logic layer:    internal/    → pure Go packages, each does one thing
//   - State layer:    internal/state → SQLite via modernc.org/sqlite (pure Go, no CGo)
//   - TUI layer:      internal/tui → shared bubbletea/lipgloss components
//
// # How state works (no config files needed)
//
// All persistent state lives in a single SQLite file:
//
//	~/.config/dira/state.db
//
// Tables are auto-created on first use — zero setup required. If you want to
// add new persistent data (e.g. saved port rules, container presets), just add
// a new table in internal/state/state.go using the same open() helper.
//
// # Adding a new command (step by step)
//
//  1. Create internal/<feature>/<feature>.go  — logic, no TUI/CLI deps
//  2. (Optional) internal/<feature>/tui.go    — if it needs interactive UI
//  3. Create cmd/<feature>.go                 — cobra command + init() register
//  4. Register in cmd/root.go → init() → rootCmd.AddCommand(...)
//  5. That's it. No config, no DI container, no wiring.
//
// # Adding external services (Redis, Postgres, etc.)
//
// dira talks to system tools (ss, smartctl, systemctl, docker/podman) via
// exec.Command — no client libraries, no connection pools. If you want to add
// a new service integration:
//
//   - For read-only monitoring: just shell out to the CLI tool and parse output
//     (see internal/ports for ss, internal/disk for smartctl/lsblk)
//   - For persistent storage: use the existing SQLite via internal/state
//   - For Redis/Postgres monitoring: exec.Command("redis-cli", "info") works,
//     no need for a Go redis client library for simple status checks
//
// # Dependencies are intentionally minimal
//
//   - cobra          → CLI framework
//   - bubbletea      → TUI framework
//   - lipgloss       → TUI styling
//   - modernc.org/sqlite → pure-Go SQLite (no CGo, no libsqlite3-dev needed)
//
// No HTTP servers, no REST APIs, no config files, no .env, no YAML.
// Everything is local CLI + sysfs + system tool wrappers.
//
// Usage:
//
//	dira keycolor ff0000
//	dira keymode
//	dira power
//	dira info
//	dira disk
//	dira service
//	dira container
//	dira ports
package main
