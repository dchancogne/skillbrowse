package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dchancogne/skillbrowse/internal/buildinfo"
)

// newVersionCmd implements FR-12: report version, commit, build date, Go
// version, OS, and architecture without launching the TUI.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and build information",
		RunE: func(cmd *cobra.Command, args []string) error {
			info := buildinfo.Get()
			out := cmd.OutOrStdout()
			lines := []string{
				fmt.Sprintf("skillbrowse %s", info.Version),
				fmt.Sprintf("  commit:     %s", info.Commit),
				fmt.Sprintf("  built:      %s", info.Date),
				fmt.Sprintf("  go version: %s", info.GoVersion),
				fmt.Sprintf("  os/arch:    %s/%s", info.OS, info.Arch),
			}
			for _, line := range lines {
				if _, err := fmt.Fprintln(out, line); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
