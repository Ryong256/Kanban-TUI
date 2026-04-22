package cli

import (
	"os"
	"path/filepath"
)

func DetectProject(override string) string {
	if override != "" {
		return override
	}
	if env := os.Getenv("KB_PROJECT"); env != "" {
		return env
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return filepath.Base(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Base(cwd)
}
