package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

type pickerModel struct {
	profiles  []profile.Meta
	current   string
	cursor    int
	chosen    string
	cancelled bool
	typed     string
	now       time.Time
}

func runPicker(profiles []profile.Meta, current string) (string, error) {
	if len(profiles) == 0 {
		return "", nil
	}

	final, err := tea.NewProgram(newPickerModel(profiles, current)).Run()
	if err != nil {
		return "", err
	}

	m, ok := final.(pickerModel)
	if !ok || m.cancelled {
		return "", nil
	}
	return m.chosen, nil
}

func newPickerModel(profiles []profile.Meta, current string) pickerModel {
	m := pickerModel{profiles: profiles, current: current, now: time.Now()}
	for i, p := range profiles {
		if p.Name == current {
			m.cursor = i
			break
		}
	}
	return m
}

func (m pickerModel) Init() tea.Cmd {
	return nil
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.typed = ""
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			m.typed = ""
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			m.typed = ""
			if m.cursor < len(m.profiles)-1 {
				m.cursor++
			}
		case "enter":
			m.typed = ""
			if m.profiles[m.cursor].ObservedHealth != profile.HealthUsable {
				return m, nil
			}
			m.chosen = m.profiles[m.cursor].Name
			return m, tea.Quit
		case "backspace":
			if m.typed != "" {
				m.typed = m.typed[:len(m.typed)-1]
				m.jumpToTypedRow()
			}
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			m.typeRowNumber(msg.String())
		}
	}
	return m, nil
}

func (m *pickerModel) typeRowNumber(digit string) {
	if digit == "0" && m.typed == "" {
		return
	}

	maxDigits := len(strconv.Itoa(len(m.profiles)))
	if len(m.typed) >= maxDigits {
		m.typed = ""
	}

	m.typed += digit
	m.jumpToTypedRow()
}

func (m *pickerModel) jumpToTypedRow() {
	if m.typed == "" {
		return
	}

	row, err := strconv.Atoi(m.typed)
	if err != nil || row < 1 || row > len(m.profiles) {
		return
	}
	m.cursor = row - 1
}

func (m pickerModel) View() string {
	var b strings.Builder
	b.WriteString("\n  ")
	b.WriteString(uiTitleStyle.Render("Select account to activate:"))
	b.WriteString("\n\n")
	profileTable{
		profiles: m.profiles,
		current:  m.current,
		cursor:   m.cursor,
		now:      m.now,
		lead:     func(i int, _ profile.Meta) string { return fmt.Sprintf("%02d", i+1) },
	}.render(&b)
	b.WriteString("\n  ")
	b.WriteString(uiHelpStyle.Render("Keys: Up/Down or j/k, Enter select, 1-9 type, Backspace edit, Esc or q quit"))
	b.WriteString("\n")
	return b.String()
}
