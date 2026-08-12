package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// outf and outln write to the command's real stdout.
//
// They exist because cobra's cmd.Print* family resolves through OutOrStderr(),
// which falls back to os.Stderr whenever no writer was injected — that is, in
// production. Reporting through it makes `kb list | grep` and
// `kb reconcile --json | jq` receive nothing, while tests that inject a writer
// see everything working.
func outf(cmd *cobra.Command, format string, a ...any) {
	fmt.Fprintf(cmd.OutOrStdout(), format, a...)
}

func outln(cmd *cobra.Command, a ...any) {
	fmt.Fprintln(cmd.OutOrStdout(), a...)
}
