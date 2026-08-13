package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// rootFlags holds the persistent flags described in
// docs/superpowers/specs/2026-08-12-skillbrowse-design.md §4.
type rootFlags struct {
	config     string
	paths      []string
	noDefaults bool
	noColor    bool
}

// usageError marks an error as an invalid-argument/configuration failure,
// which maps to exit code 2 instead of the default operational-failure
// exit code 1.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

func exitCodeFor(err error) int {
	var u *usageError
	if asUsageError(err, &u) {
		return 2
	}
	return 1
}

func asUsageError(err error, target **usageError) bool {
	for err != nil {
		if u, ok := err.(*usageError); ok {
			*target = u
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}

func newRootCmd() *cobra.Command {
	flags := &rootFlags{}

	cmd := &cobra.Command{
		Use:           "skillbrowse",
		Short:         "Browse locally installed AI-agent skills",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI(cmd)
		},
	}

	cmd.PersistentFlags().StringVar(&flags.config, "config", "", "path to config.toml")
	cmd.PersistentFlags().StringArrayVar(&flags.paths, "path", nil, "additional unlabeled custom source (repeatable)")
	cmd.PersistentFlags().BoolVar(&flags.noDefaults, "no-defaults", false, "scan only command-line and configured sources")
	cmd.PersistentFlags().BoolVar(&flags.noColor, "no-color", false, "disable ANSI styling")

	cmd.AddCommand(newUpgradeCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

// runTUI will launch the Bubble Tea catalog browser once
// internal/catalog and internal/ui exist (Phase 1 and Phase 2 of
// docs/skillbrowse-implementation-plan.md). It will take rootFlags once
// they're needed to build the source list. For now it reports that the
// scaffolding is not yet wired up.
func runTUI(cmd *cobra.Command) error {
	_, err := fmt.Fprintln(cmd.OutOrStdout(), "skillbrowse: catalog and TUI are not implemented yet (Phase 0 scaffolding only)")
	return err
}
