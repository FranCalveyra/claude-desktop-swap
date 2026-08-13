package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

type tuiMode int

const (
	tuiProgress tuiMode = iota
	tuiDashboard
	tuiInput
	tuiConfirm
	tuiResult
	tuiAddLogin
)

type tuiAction int

const (
	tuiActionNone tuiAction = iota
	tuiActionActivate
	tuiActionSave
	tuiActionAdd
	tuiActionFinishAdd
	tuiActionDelete
)

type tuiSnapshot struct {
	profiles []profile.Meta
	current  string
	status   string
	now      time.Time
}

type tuiSnapshotMsg struct {
	snapshot tuiSnapshot
	err      error
}

type tuiOperationMsg struct {
	action tuiAction
	name   string
	err    error
}

type tuiBackend interface {
	Load() (tuiSnapshot, error)
	Activate(string) error
	Save(string) error
	BeginAdd(string) error
	FinishAdd(string) error
	Delete(string) error
}

type tuiModel struct {
	backend  tuiBackend
	profiles []profile.Meta
	current  string
	status   string
	now      time.Time
	cursor   int
	mode     tuiMode
	pending  tuiAction
	name     string
	message  string
	failed   bool
}

func runTUI() error {
	backend, err := newDefaultTUIBackend()
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(newTUIModel(backend)).Run()
	return err
}

func newTUIModel(backend tuiBackend) tuiModel {
	return tuiModel{backend: backend, mode: tuiProgress, message: "Loading profiles..."}
}

func tuiModelWithSnapshot(backend tuiBackend, snapshot tuiSnapshot) tuiModel {
	m := newTUIModel(backend)
	m.applySnapshot(snapshot)
	return m
}

func (m tuiModel) Init() tea.Cmd {
	return loadTUISnapshotCmd(m.backend)
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tuiSnapshotMsg:
		if msg.err != nil {
			m.mode = tuiResult
			m.message = "Could not refresh: " + msg.err.Error()
			m.failed = true
			return m, nil
		}
		m.applySnapshot(msg.snapshot)
		return m, nil
	case tuiOperationMsg:
		return m.operationFinished(msg)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.mode {
		case tuiDashboard:
			return m.updateDashboard(msg)
		case tuiInput:
			return m.updateInput(msg)
		case tuiConfirm:
			return m.updateConfirm(msg)
		case tuiResult:
			return m.updateResult(msg)
		case tuiAddLogin:
			return m.updateAddLogin(msg)
		}
	}
	return m, nil
}

func (m tuiModel) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.profiles)-1 {
			m.cursor++
		}
	case "enter", "u":
		selected, ok := m.selected()
		if !ok || selected.ObservedHealth != profile.HealthUsable {
			return m, nil
		}
		m.openConfirmation(tuiActionActivate, selected.Name)
	case "s":
		m.openInput(tuiActionSave)
	case "a":
		m.openInput(tuiActionAdd)
	case "d":
		selected, ok := m.selected()
		if !ok {
			return m, nil
		}
		m.openConfirmation(tuiActionDelete, selected.Name)
	case "r":
		m.mode = tuiProgress
		m.message = "Refreshing profiles..."
		return m, loadTUISnapshotCmd(m.backend)
	}
	return m, nil
}

func (m tuiModel) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.resetDashboard()
		return m, nil
	case tea.KeyEnter:
		m.name = strings.TrimSpace(m.name)
		if m.name != "" {
			m.mode = tuiConfirm
		}
		return m, nil
	case tea.KeyBackspace, tea.KeyDelete:
		if m.name != "" {
			runes := []rune(m.name)
			m.name = string(runes[:len(runes)-1])
		}
		return m, nil
	case tea.KeySpace:
		m.name += " "
		return m, nil
	case tea.KeyRunes:
		m.name += string(msg.Runes)
		return m, nil
	}
	return m, nil
}

func (m tuiModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		m.mode = tuiProgress
		m.message = progressMessage(m.pending, m.name)
		return m, runTUIActionCmd(m.backend, m.pending, m.name)
	case "n", "esc":
		m.resetDashboard()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m tuiModel) updateResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "r":
		m.mode = tuiProgress
		m.message = "Refreshing profiles..."
		return m, loadTUISnapshotCmd(m.backend)
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m tuiModel) updateAddLogin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.mode = tuiProgress
		m.pending = tuiActionFinishAdd
		m.message = progressMessage(tuiActionFinishAdd, m.name)
		return m, runTUIActionCmd(m.backend, tuiActionFinishAdd, m.name)
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m tuiModel) operationFinished(msg tuiOperationMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.mode = tuiResult
		m.message = actionLabel(msg.action) + " failed: " + msg.err.Error()
		m.failed = true
		return m, nil
	}
	if msg.action == tuiActionAdd {
		m.mode = tuiAddLogin
		m.pending = tuiActionFinishAdd
		m.name = msg.name
		m.message = fmt.Sprintf("Claude Desktop is ready for %q.", msg.name)
		return m, nil
	}
	m.mode = tuiResult
	m.message = successMessage(msg.action, msg.name)
	m.failed = false
	return m, nil
}

func (m *tuiModel) applySnapshot(snapshot tuiSnapshot) {
	m.profiles = snapshot.profiles
	m.current = snapshot.current
	m.status = snapshot.status
	m.now = snapshot.now
	if m.now.IsZero() {
		m.now = time.Now()
	}
	m.cursor = 0
	for i, p := range m.profiles {
		if p.Name == m.current {
			m.cursor = i
			break
		}
	}
	m.resetDashboard()
}

func (m *tuiModel) openInput(action tuiAction) {
	m.mode = tuiInput
	m.pending = action
	m.name = ""
	m.message = ""
	m.failed = false
}

func (m *tuiModel) openConfirmation(action tuiAction, name string) {
	m.mode = tuiConfirm
	m.pending = action
	m.name = name
	m.message = ""
}

func (m *tuiModel) resetDashboard() {
	m.mode = tuiDashboard
	m.pending = tuiActionNone
	m.name = ""
	m.message = ""
	m.failed = false
}

func (m tuiModel) selected() (profile.Meta, bool) {
	if m.cursor < 0 || m.cursor >= len(m.profiles) {
		return profile.Meta{}, false
	}
	return m.profiles[m.cursor], true
}

func loadTUISnapshotCmd(backend tuiBackend) tea.Cmd {
	return func() tea.Msg {
		snapshot, err := backend.Load()
		return tuiSnapshotMsg{snapshot: snapshot, err: err}
	}
}

func runTUIActionCmd(backend tuiBackend, action tuiAction, name string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch action {
		case tuiActionActivate:
			err = backend.Activate(name)
		case tuiActionSave:
			err = backend.Save(name)
		case tuiActionAdd:
			err = backend.BeginAdd(name)
		case tuiActionFinishAdd:
			err = backend.FinishAdd(name)
		case tuiActionDelete:
			err = backend.Delete(name)
		}
		return tuiOperationMsg{action: action, name: name, err: err}
	}
}
