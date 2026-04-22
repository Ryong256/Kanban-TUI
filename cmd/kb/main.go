package main

import (
	"fmt"
	"os"

	"github.com/Ryong256/kanban/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "kb:", err)
		os.Exit(1)
	}
}
