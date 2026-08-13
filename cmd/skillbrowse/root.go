package main

import (
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"

	"github.com/dchancogne/skillbrowse/internal/buildinfo"
	"github.com/dchancogne/skillbrowse/internal/config"
	"github.com/dchancogne/skillbrowse/internal/debug"
	"github.com/dchancogne/skillbrowse/internal/sources"
	"github.com/dchancogne/skillbrowse/internal/ui"
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
	if asUsageError(err, &u) || looksLikeUsageError(err) {
		return 2
	}
	return 1
}

// looksLikeUsageError recognizes Cobra's own pre-RunE errors (unknown
// command, unknown flag, wrong argument count, ...), which are always
// invalid-syntax problems per design doc §4 but arrive as plain,
// untyped errors — Cobra has no typed error for them to check with
// errors.As, and SetFlagErrorFunc only covers flag-parsing errors, not
// command-routing ones like "unknown command". Matching on Cobra's own
// stable error-message prefixes is the pragmatic alternative.
func looksLikeUsageError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, prefix := range []string{
		"unknown command",
		"unknown flag",
		"unknown shorthand flag",
		"flag needs an argument",
		"invalid argument",
		"requires at least",
		"requires exactly",
		"accepts at most",
		"accepts between",
		"bad flag syntax",
	} {
		if strings.HasPrefix(msg, prefix) {
			return true
		}
	}
	return false
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
			return runTUI(cmd, flags)
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

// runTUI resolves sources (built-in registry + config + --path) and
// launches the Bubble Tea catalog browser.
func runTUI(cmd *cobra.Command, flags *rootFlags) error {
	configPath := flags.config
	if configPath == "" {
		p, err := config.DefaultPath()
		if err != nil {
			return err
		}
		configPath = p
	}

	debug.Log("config path: %s", configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		return &usageError{err: err}
	}
	debug.Log("config: version=%d sources=%d", cfg.Version, len(cfg.Sources))

	srcs, err := sources.Load(cfg, flags.paths, flags.noDefaults)
	if err != nil {
		return err
	}
	for _, s := range srcs {
		debug.Log("source: label=%q origin=%s root=%s maxDepth=%d agents=%v", s.Label, s.Origin, s.Root, s.MaxDepth, s.Agents)
	}

	noColor := flags.noColor || os.Getenv("NO_COLOR") != ""

	model := ui.New(srcs, ui.WithNoColor(noColor), ui.WithVersion(buildinfo.Version))
	progOpts := []tea.ProgramOption{
		tea.WithInput(cmd.InOrStdin()),
		tea.WithOutput(cmd.OutOrStdout()),
	}
	if noColor {
		// Force the whole program (list, help, viewport styling — not
		// just skillbrowse's own header/footer strings) to emit no ANSI
		// codes at all, per design doc §13 ("NO_COLOR and --no-color
		// are honored").
		progOpts = append(progOpts, tea.WithColorProfile(colorprofile.Ascii))
	}
	_, err = tea.NewProgram(model, progOpts...).Run()
	return err
}
