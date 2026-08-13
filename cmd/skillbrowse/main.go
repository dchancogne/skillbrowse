// Command skillbrowse is a read-only terminal browser for locally
// installed AI-agent skills.
package main

import (
	"fmt"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run builds and executes the root command, translating errors into the
// exit-code contract from docs/skillbrowse-implementation-plan.md Phase 4:
// 0 success, 1 operational failure, 2 invalid arguments or configuration.
// The root command sets SilenceErrors, so this is the only place an
// error is ever printed.
func run(args []string) int {
	root := newRootCmd()
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(root.ErrOrStderr(), "Error:", err)
		return exitCodeFor(err)
	}
	return 0
}
