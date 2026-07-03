// Package state persists per-mode keyboard RGB state and active power profile
// in a SQLite database at ~/.config/dira/state.db.
//
// # Why SQLite (and not a config file)
//
// SQLite gives us ACID writes, schema migration via CREATE TABLE IF NOT EXISTS,
// and zero config — no YAML, no JSON parsing, no file locking bugs. The DB file
// is auto-created on first use.
//
// # How to add new persistent data
//
// Example: you want to save user-defined port watch rules.
//
//  1. Add a CREATE TABLE in open():
//
//     CREATE TABLE IF NOT EXISTS port_watch (
//     port INTEGER PRIMARY KEY,
//     label TEXT
//     );
//
//  2. Add Save/Load functions following the same pattern as Save() and Load():
//
//     func SavePortWatch(port int, label string) error {
//     db, err := open()
//     if err != nil { return err }
//     defer db.Close()
//     _, err = db.Exec(`INSERT OR REPLACE INTO port_watch(port,label) VALUES(?,?)`, port, label)
//     return err
//     }
//
// That's it — no migrations tool, no ORM, no config. The table is created
// automatically on first run after you add the CREATE TABLE statement.
//
// # Connection pattern
//
// Every function opens its own connection and closes it immediately. This is
// fine for a CLI tool (not a server). SQLite handles the locking internally.
// We don't keep a global *sql.DB because the process is short-lived.
//
// # Adding Redis, Postgres, or other DBs
//
// You probably don't need them for a local CLI tool. But if you do:
//
//   - For monitoring (read-only): exec.Command("redis-cli", "info") is simpler
//     than importing a Go client library
//   - For actual storage: just add more tables to this SQLite — it can handle
//     millions of rows without breaking a sweat
//   - If you truly need a separate DB: create internal/mydb/mydb.go with its
//     own open() and connection logic, following the same pattern
package state
