// Package buildinfo holds build-time metadata injected via -ldflags -X.
// See the Makefile's LDFLAGS and `kb update` for how these are stamped.
package buildinfo

var (
	// Version is a git describe string (e.g. "abc1234" or "abc1234-dirty").
	Version = "dev"
	// Date is the build date in YYYY-MM-DD.
	Date = "unknown"
	// SourceDir is the absolute path of the repo this binary was built from.
	// `kb update` rebuilds from here; empty means the binary was not stamped
	// (bootstrap once with `make install` from the repo).
	SourceDir = ""
)
