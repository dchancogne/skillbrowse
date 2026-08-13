package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dchancogne/skillbrowse/internal/buildinfo"
	"github.com/dchancogne/skillbrowse/internal/debug"
	"github.com/dchancogne/skillbrowse/internal/update"
)

// newUpgradeCmd implements the `skillbrowse upgrade [--check] [--yes]`
// surface from docs/superpowers/specs/2026-08-12-skillbrowse-design.md
// §4 and §12.1.
func newUpgradeCmd() *cobra.Command {
	var checkOnly, assumeYes bool

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Check for and install skillbrowse updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(cmd, checkOnly, assumeYes)
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "report update availability without installing")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "install without interactive confirmation")

	return cmd
}

func runUpgrade(cmd *cobra.Command, checkOnly, assumeYes bool) error {
	out := cmd.OutOrStdout()
	ctx := context.Background()

	client := update.NewClient()
	debug.Log("upgrade: checking %s/%s for a release newer than %s", client.Owner, client.Repo, buildinfo.Version)
	check, err := update.CheckForUpdate(ctx, client, buildinfo.Version)
	if err != nil {
		debug.Log("upgrade: check failed: %s", err)
		return err
	}
	debug.Log("upgrade: latest=%s available=%t asset=%s", check.Latest, check.UpdateAvailable, check.Asset.Name)

	if !check.UpdateAvailable {
		_, err := fmt.Fprintf(out, "skillbrowse %s is up to date.\n", check.Current)
		return err
	}

	if _, err := fmt.Fprintf(out, "Update available: %s -> %s\n", check.Current, check.Latest); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Release: %s\n", check.ReleaseURL); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Download size: %d bytes\n", check.Asset.Size); err != nil {
		return err
	}

	if checkOnly {
		return nil
	}

	if !assumeYes {
		confirmed, err := confirmInstall(cmd)
		if err != nil {
			return err
		}
		if !confirmed {
			_, err := fmt.Fprintln(out, "Update cancelled.")
			return err
		}
	}

	opts := update.Options{Client: client, Verifier: update.DefaultVerifier()}
	if err := update.Apply(ctx, opts, check); err != nil {
		debug.Log("upgrade: apply failed: %s", err)
		return err
	}
	debug.Log("upgrade: apply succeeded")

	_, err = fmt.Fprintf(out, "Updated to %s. Restart skillbrowse to use it.\n", check.Latest)
	return err
}

// confirmInstall prompts the user on an interactive terminal, per design
// doc §12.1. On a non-interactive terminal without --yes, it errors
// rather than blocking on a read that will never complete.
func confirmInstall(cmd *cobra.Command) (bool, error) {
	in := cmd.InOrStdin()
	if !isInteractive(in) {
		return false, fmt.Errorf("confirmation required to install; rerun with --yes for non-interactive use")
	}

	if _, err := fmt.Fprint(cmd.OutOrStdout(), "Install this update? [y/N] "); err != nil {
		return false, err
	}

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes", nil
}

func isInteractive(in io.Reader) bool {
	f, ok := in.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
