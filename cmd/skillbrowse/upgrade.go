package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newUpgradeCmd implements the `skillbrowse upgrade [--check] [--yes]`
// surface from docs/superpowers/specs/2026-08-12-skillbrowse-design.md §4.
// The actual release lookup and verified replacement land in
// internal/update (Phase 3 of docs/skillbrowse-implementation-plan.md).
func newUpgradeCmd() *cobra.Command {
	var checkOnly, assumeYes bool

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Check for and install skillbrowse updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "skillbrowse: the updater is not implemented yet (Phase 3 of the implementation plan)")
			_ = checkOnly
			_ = assumeYes
			return err
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "report update availability without installing")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "install without interactive confirmation")

	return cmd
}
