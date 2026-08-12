package cli

import (
	"database/sql"
	"io"

	"github.com/spf13/cobra"
)

// NewTestRoot returns a root command backed by the provided database.
func NewTestRoot(d *sql.DB) *cobra.Command {
	testDB = d
	root := NewRoot()
	propagateOutput(root)
	return root
}

// NewTestRootWithOutput is like NewTestRoot but also captures command output.
func NewTestRootWithOutput(d *sql.DB, out, err io.Writer) *cobra.Command {
	testDB = d
	root := NewRoot()
	root.SetOut(out)
	root.SetErr(err)
	propagateOutput(root)
	return root
}

func propagateOutput(root *cobra.Command) {
	out, errOut := root.OutOrStdout(), root.ErrOrStderr()
	for _, c := range root.Commands() {
		c.SetOut(out)
		c.SetErr(errOut)
	}
}
