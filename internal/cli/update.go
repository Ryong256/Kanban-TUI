package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Ryong256/kanban/internal/buildinfo"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Rebuild and reinstall kb from its source tree",
		Long: "Rebuild kb from the repo it was built from and reinstall it.\n" +
			"Picks up uncommitted changes in the working tree. The new binary is\n" +
			"re-stamped so `kb update` keeps working. Runnable from anywhere.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			src := buildinfo.SourceDir
			if src == "" {
				return fmt.Errorf("this kb has no stamped source dir — bootstrap once with `make install` from the repo")
			}
			if _, err := os.Stat(src); err != nil {
				return fmt.Errorf("source dir %q not found (repo moved?) — re-run `make install` from the repo: %w", src, err)
			}
			if _, err := exec.LookPath("go"); err != nil {
				return fmt.Errorf("`go` not found in PATH — install Go to rebuild kb")
			}

			version := gitVersion(src)
			date := time.Now().Format("2006-01-02")
			ldflags := strings.Join([]string{
				"-X github.com/Ryong256/kanban/internal/buildinfo.Version=" + version,
				"-X github.com/Ryong256/kanban/internal/buildinfo.Date=" + date,
				"-X github.com/Ryong256/kanban/internal/buildinfo.SourceDir=" + src,
			}, " ")

			fmt.Printf("kb: rebuilding from %s ...\n", src)
			c := exec.Command("go", "install", "-ldflags", ldflags, "./cmd/kb")
			c.Dir = src
			c.Env = os.Environ()
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("go install failed: %w", err)
			}
			fmt.Printf("kb: installed %s (%s)\n", version, date)
			return nil
		},
	}
}

// gitVersion returns `git describe --always --dirty` for dir, or "dev" if git
// is unavailable or the dir is not a repo.
func gitVersion(dir string) string {
	out, err := exec.Command("git", "-C", dir, "describe", "--always", "--dirty").Output()
	if err != nil {
		return "dev"
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "dev"
	}
	return v
}
