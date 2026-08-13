package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dchancogne/skillbrowse/internal/update"
)

const (
	updateCheckTimeout = 20 * time.Second
	updateApplyTimeout = 5 * time.Minute
)

// updateCheckMsg and updateApplyMsg carry the results of the background
// update-check and update-install Cmds back into Update.
type updateCheckMsg struct {
	check *update.CheckResult
	err   error
}

type updateApplyMsg struct {
	err error
}

func (m *Model) checkUpdateCmd() tea.Cmd {
	client := m.updateClient
	current := m.currentVersion
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()
		check, err := update.CheckForUpdate(ctx, client, current)
		return updateCheckMsg{check: check, err: err}
	}
}

func (m *Model) applyUpdateCmd(check *update.CheckResult) tea.Cmd {
	client := m.updateClient
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), updateApplyTimeout)
		defer cancel()
		err := update.Apply(ctx, update.Options{Client: client, Verifier: update.DefaultVerifier()}, check)
		return updateApplyMsg{err: err}
	}
}
