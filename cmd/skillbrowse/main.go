// Command skillbrowse is a read-only terminal browser for locally
// installed AI-agent skills.
package main

import "os"

func main() {
	os.Exit(run(os.Args[1:]))
}

// run builds and executes the root command, translating errors into the
// exit-code contract from docs/skillbrowse-implementation-plan.md Phase 4:
// 0 success, 1 operational failure, 2 invalid arguments or configuration.
func run(args []string) int {
	root := newRootCmd()
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		return exitCodeFor(err)
	}
	return 0
}
