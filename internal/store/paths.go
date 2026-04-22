package store

import (
	"os"
	"path/filepath"
)

func dataDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "kanban")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "kanban")
}

func DBPath() string {
	if p := os.Getenv("KB_DB"); p != "" {
		return p
	}
	return filepath.Join(dataDir(), "db.sqlite")
}

func EnsureDataDir() error {
	return os.MkdirAll(dataDir(), 0o755)
}
