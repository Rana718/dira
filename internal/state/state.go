package state

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type ModeState struct {
	R, G, B, Speed int
}

var defaults = map[string]ModeState{
	"static":    {255, 255, 255, 1},
	"breathing": {255, 255, 255, 1},
	"cycle":     {0, 0, 0, 1},
}

func dbPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "dira", "state.db")
}

func open() (*sql.DB, error) {
	p := dbPath()
	os.MkdirAll(filepath.Dir(p), 0755) //nolint:errcheck
	db, err := sql.Open("sqlite", p)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS mode_state (
			mode  TEXT PRIMARY KEY,
			r     INTEGER, g INTEGER, b INTEGER, speed INTEGER
		);
		CREATE TABLE IF NOT EXISTS active (
			id   INTEGER PRIMARY KEY CHECK(id=1),
			mode TEXT NOT NULL DEFAULT 'static'
		);
		INSERT OR IGNORE INTO active(id,mode) VALUES(1,'static');
	`)
	return db, err
}

func Load(mode string) (ModeState, error) {
	db, err := open()
	if err != nil {
		return defaults[mode], err
	}
	defer db.Close()

	var s ModeState
	err = db.QueryRow(`SELECT r,g,b,speed FROM mode_state WHERE mode=?`, mode).
		Scan(&s.R, &s.G, &s.B, &s.Speed)
	if err == sql.ErrNoRows {
		return defaults[mode], nil
	}
	return s, err
}

func Save(mode string, r, g, b, speed int) error {
	db, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(
		`INSERT INTO mode_state(mode,r,g,b,speed) VALUES(?,?,?,?,?)
		 ON CONFLICT(mode) DO UPDATE SET r=?,g=?,b=?,speed=?`,
		mode, r, g, b, speed, r, g, b, speed,
	)
	return err
}

func SetActiveMode(mode string) error {
	db, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`UPDATE active SET mode=? WHERE id=1`, mode)
	return err
}

func GetActiveMode() string {
	db, err := open()
	if err != nil {
		return "static"
	}
	defer db.Close()
	var mode string
	if err := db.QueryRow(`SELECT mode FROM active WHERE id=1`).Scan(&mode); err != nil {
		return "static"
	}
	return mode
}
