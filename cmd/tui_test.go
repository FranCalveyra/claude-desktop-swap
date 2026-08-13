package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

func TestTUIDashboardDoesNotRenderSecretMetadata(t *testing.T) {
	setTestColorProfile(t, termenv.Ascii)
	backend := &fakeTUIBackend{}
	m := tuiModelWithSnapshot(backend, tuiSnapshot{
		profiles: []profile.Meta{{
			Name:           "work",
			Email:          "person@example.com",
			ObservedHealth: profile.HealthUsable,
			CookieDigest:   "secret-cookie-digest",
			AccountUUID:    "sessionKey-secret-value",
		}},
		current: "work",
		status:  "Active profile: work",
	})

	view := m.View()
	for _, forbidden := range []string{"sessionKey", "secret-cookie", "secret-value"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("view exposed %q: %q", forbidden, view)
		}
	}
	for _, expected := range []string{"Profiles", "work", "person@example.com", "Active profile: work", "[ACTIVE]"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view missing %q: %q", expected, view)
		}
	}
}

func TestTUIDashboardUsesCyanHierarchyAndSemanticColors(t *testing.T) {
	setTestColorProfile(t, termenv.ANSI)
	m := tuiModelWithSnapshot(&fakeTUIBackend{}, tuiSnapshot{
		profiles: []profile.Meta{
			{Name: "usable", ObservedHealth: profile.HealthUsable},
			{Name: "expired", ObservedHealth: profile.HealthExpired},
			{Name: "unknown", ObservedHealth: profile.HealthUnknown},
		},
		status: "Active profile: usable\nWarning: session expires soon",
	})

	view := m.View()
	for _, code := range []string{"96m", "36m", "92m", "91m", "93m"} {
		if !strings.Contains(view, code) {
			t.Fatalf("view missing ANSI color %q: %q", code, view)
		}
	}
}

func TestTUIPanelsUseCyanPromptsAndSemanticResults(t *testing.T) {
	setTestColorProfile(t, termenv.ANSI)
	backend := &fakeTUIBackend{}
	m := newTUIModel(backend)
	if view := m.View(); !strings.Contains(view, "96m") {
		t.Fatalf("progress view lacks cyan: %q", view)
	}

	m.mode = tuiInput
	m.pending = tuiActionSave
	if view := m.View(); !strings.Contains(view, "96m") {
		t.Fatalf("input view lacks cyan: %q", view)
	}

	m.mode = tuiConfirm
	m.name = "work"
	if view := m.View(); !strings.Contains(view, "96m") {
		t.Fatalf("confirmation view lacks cyan: %q", view)
	}

	updated, _ := m.Update(tuiOperationMsg{action: tuiActionSave, name: "work"})
	m = updated.(tuiModel)
	if view := m.View(); !strings.Contains(view, "92m") {
		t.Fatalf("success view lacks green: %q", view)
	}

	updated, _ = m.Update(tuiOperationMsg{action: tuiActionSave, name: "work", err: errors.New("offline")})
	m = updated.(tuiModel)
	if view := m.View(); !strings.Contains(view, "91m") {
		t.Fatalf("error view lacks red: %q", view)
	}
}

func TestTUIBlocksUnusableActivationInUpdate(t *testing.T) {
	for _, health := range []profile.Health{profile.HealthExpired, profile.HealthMissing, profile.HealthUnknown} {
		t.Run(string(health), func(t *testing.T) {
			backend := &fakeTUIBackend{}
			m := tuiModelWithSnapshot(backend, tuiSnapshot{profiles: []profile.Meta{{Name: "work", ObservedHealth: health}}})

			updated, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			got := updated.(tuiModel)
			if got.mode != tuiDashboard || got.pending != tuiActionNone || command != nil {
				t.Fatalf("unusable profile advanced: mode=%v pending=%v command=%v", got.mode, got.pending, command)
			}
		})
	}
}

func TestTUIConfirmsAndRunsActivationAsCommand(t *testing.T) {
	setTestColorProfile(t, termenv.Ascii)
	backend := &fakeTUIBackend{}
	m := tuiModelWithSnapshot(backend, tuiSnapshot{profiles: []profile.Meta{{Name: "work", ObservedHealth: profile.HealthUsable}}})

	updated, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tuiModel)
	if m.mode != tuiConfirm || m.pending != tuiActionActivate || command != nil {
		t.Fatalf("activation was not confirmed first: mode=%v pending=%v", m.mode, m.pending)
	}

	updated, command = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(tuiModel)
	if m.mode != tuiProgress || command == nil {
		t.Fatalf("confirmation did not start progress: mode=%v command=%v", m.mode, command)
	}
	if len(backend.calls) != 0 {
		t.Fatalf("operation ran inside Update: %v", backend.calls)
	}

	msg := command()
	if len(backend.calls) != 1 || backend.calls[0] != "activate:work" {
		t.Fatalf("calls = %v", backend.calls)
	}
	updated, _ = m.Update(msg)
	m = updated.(tuiModel)
	if m.mode != tuiResult || !strings.Contains(m.View(), "activated") {
		t.Fatalf("operation result missing: mode=%v view=%q", m.mode, m.View())
	}
}

func TestTUICollectsNameAndConfirmsSave(t *testing.T) {
	backend := &fakeTUIBackend{}
	m := tuiModelWithSnapshot(backend, tuiSnapshot{})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(tuiModel)
	if m.mode != tuiInput || m.pending != tuiActionSave {
		t.Fatalf("save input not opened: mode=%v pending=%v", m.mode, m.pending)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("work")})
	m = updated.(tuiModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tuiModel)
	if m.mode != tuiConfirm || m.name != "work" {
		t.Fatalf("save not confirmed: mode=%v name=%q", m.mode, m.name)
	}
}

func TestTUIDeleteRequiresConfirmation(t *testing.T) {
	backend := &fakeTUIBackend{}
	m := tuiModelWithSnapshot(backend, tuiSnapshot{profiles: []profile.Meta{{Name: "work", ObservedHealth: profile.HealthExpired}}})

	updated, command := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(tuiModel)
	if m.mode != tuiConfirm || m.pending != tuiActionDelete || m.name != "work" || command != nil {
		t.Fatalf("delete skipped confirmation: mode=%v pending=%v name=%q", m.mode, m.pending, m.name)
	}
}

func TestTUIAddWaitsForExternalLoginBeforeFinishing(t *testing.T) {
	setTestColorProfile(t, termenv.Ascii)
	backend := &fakeTUIBackend{}
	m := tuiModelWithSnapshot(backend, tuiSnapshot{})
	m.mode = tuiProgress
	m.pending = tuiActionAdd
	m.name = "personal"

	updated, _ := m.Update(tuiOperationMsg{action: tuiActionAdd, name: "personal"})
	m = updated.(tuiModel)
	if m.mode != tuiAddLogin || !strings.Contains(m.View(), "Claude Desktop") {
		t.Fatalf("add login handoff missing: mode=%v view=%q", m.mode, m.View())
	}

	updated, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tuiModel)
	if m.mode != tuiProgress || command == nil {
		t.Fatalf("finish add did not start: mode=%v command=%v", m.mode, command)
	}
	_ = command()
	if len(backend.calls) != 1 || backend.calls[0] != "finish-add:personal" {
		t.Fatalf("calls = %v", backend.calls)
	}
}

func TestTUIResultRefreshesOnlyAfterAcknowledgement(t *testing.T) {
	backend := &fakeTUIBackend{snapshot: tuiSnapshot{status: "refreshed"}}
	m := tuiModelWithSnapshot(backend, tuiSnapshot{})
	m.mode = tuiProgress
	m.pending = tuiActionDelete
	m.name = "work"

	updated, command := m.Update(tuiOperationMsg{action: tuiActionDelete, name: "work"})
	m = updated.(tuiModel)
	if m.mode != tuiResult || command != nil {
		t.Fatalf("result not shown: mode=%v command=%v", m.mode, command)
	}
	updated, command = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tuiModel)
	if m.mode != tuiProgress || command == nil {
		t.Fatalf("refresh not scheduled: mode=%v command=%v", m.mode, command)
	}
	updated, _ = m.Update(command())
	m = updated.(tuiModel)
	if m.mode != tuiDashboard || m.status != "refreshed" {
		t.Fatalf("dashboard not refreshed: mode=%v status=%q", m.mode, m.status)
	}
}

func TestTUIOperationErrorBecomesResult(t *testing.T) {
	setTestColorProfile(t, termenv.Ascii)
	backend := &fakeTUIBackend{err: errors.New("offline")}
	m := tuiModelWithSnapshot(backend, tuiSnapshot{profiles: []profile.Meta{{Name: "work", ObservedHealth: profile.HealthUsable}}})
	m.mode = tuiConfirm
	m.pending = tuiActionActivate
	m.name = "work"

	updated, command := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(tuiModel)
	updated, _ = m.Update(command())
	m = updated.(tuiModel)
	if m.mode != tuiResult || !strings.Contains(m.View(), "offline") {
		t.Fatalf("error result missing: mode=%v view=%q", m.mode, m.View())
	}
}

type fakeTUIBackend struct {
	snapshot tuiSnapshot
	err      error
	calls    []string
}

func (b *fakeTUIBackend) Load() (tuiSnapshot, error) {
	b.calls = append(b.calls, "load")
	return b.snapshot, b.err
}

func (b *fakeTUIBackend) Activate(name string) error {
	b.calls = append(b.calls, "activate:"+name)
	return b.err
}

func (b *fakeTUIBackend) Save(name string) error {
	b.calls = append(b.calls, "save:"+name)
	return b.err
}

func (b *fakeTUIBackend) BeginAdd(name string) error {
	b.calls = append(b.calls, "begin-add:"+name)
	return b.err
}

func (b *fakeTUIBackend) FinishAdd(name string) error {
	b.calls = append(b.calls, "finish-add:"+name)
	return b.err
}

func (b *fakeTUIBackend) Delete(name string) error {
	b.calls = append(b.calls, "delete:"+name)
	return b.err
}
