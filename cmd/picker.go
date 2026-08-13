package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	pickerTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	pickerHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	pickerHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	pickerBadgeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	pickerCursorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	pickerSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	pickerPromptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	pickerProgressStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	pickerUsableStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	pickerWarningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	pickerErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
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
	accountWidth, planWidth, healthWidth, expiresWidth, lastWidth := m.columnWidths()

	b.WriteString("\n  ")
	b.WriteString(pickerTitleStyle.Render("Select account to activate:"))
	b.WriteString("\n\n")
	b.WriteString(pickerHeaderStyle.Render(fmt.Sprintf(
		"       %s  %s  %s  %s  %s",
		padRight("ACCOUNT", accountWidth),
		padRight("PLAN", planWidth),
		padRight("HEALTH", healthWidth),
		padRight("EXPIRES", expiresWidth),
		padRight("LAST", lastWidth),
	)))
	b.WriteString("\n")

	for i, p := range m.profiles {
		cursor := " "
		if i == m.cursor {
			cursor = pickerCursorStyle.Render(">")
		}
		rowNumber := fmt.Sprintf("%02d", i+1)
		account := padRight(accountLabel(p), accountWidth)
		plan := padRight(planLabel(p), planWidth)
		last := padRight(relativeLastUsed(p.LastUsed), lastWidth)
		if i == m.cursor {
			rowNumber = pickerSelectedStyle.Render(rowNumber)
			account = pickerSelectedStyle.Render(account)
			plan = pickerSelectedStyle.Render(plan)
			last = pickerSelectedStyle.Render(last)
		}

		badge := ""
		if p.Name == m.current {
			badge = "  " + pickerBadgeStyle.Render("[ACTIVE]")
		}

		fmt.Fprintf(&b,
			"  %s %s %s  %s  %s  %s  %s%s\n",
			cursor,
			rowNumber,
			account,
			plan,
			padRight(styledHealthLabel(p.ObservedHealth), healthWidth),
			padRight(styledExpiryLabel(p, m.now), expiresWidth),
			last,
			badge,
		)
	}

	b.WriteString("\n  ")
	b.WriteString(pickerHelpStyle.Render("Keys: Up/Down or j/k, Enter select, 1-9 type, Backspace edit, Esc or q quit"))
	b.WriteString("\n")
	return b.String()
}

func (m pickerModel) columnWidths() (accountWidth, planWidth, healthWidth, expiresWidth, lastWidth int) {
	accountWidth = lipgloss.Width("ACCOUNT")
	planWidth = lipgloss.Width("PLAN")
	healthWidth = lipgloss.Width("HEALTH")
	expiresWidth = lipgloss.Width("EXPIRES")
	lastWidth = lipgloss.Width("LAST")

	for _, p := range m.profiles {
		accountWidth = max(accountWidth, lipgloss.Width(accountLabel(p)))
		planWidth = max(planWidth, lipgloss.Width(planLabel(p)))
		healthWidth = max(healthWidth, lipgloss.Width(healthLabel(p.ObservedHealth)))
		expiresWidth = max(expiresWidth, lipgloss.Width(asciiExpiryLabel(p, m.now)))
		lastWidth = max(lastWidth, lipgloss.Width(relativeLastUsed(p.LastUsed)))
	}

	planWidth = max(planWidth, 8)
	lastWidth = max(lastWidth, 6)
	return accountWidth, planWidth, healthWidth, expiresWidth, lastWidth
}

func styledHealthLabel(health profile.Health) string {
	label := healthLabel(health)
	switch health {
	case profile.HealthUsable:
		return pickerUsableStyle.Render(label)
	case profile.HealthExpired, profile.HealthMissing:
		return pickerErrorStyle.Render(label)
	case profile.HealthUnknown:
		return pickerWarningStyle.Render(label)
	default:
		return label
	}
}

func asciiExpiryLabel(p profile.Meta, now time.Time) string {
	return strings.ReplaceAll(metaExpiryLabel(p, now), " ⚠", " !")
}

func styledExpiryLabel(p profile.Meta, now time.Time) string {
	label := asciiExpiryLabel(p, now)
	switch renewalFor(p, now) {
	case profile.RenewalExpired:
		return pickerErrorStyle.Render(label)
	case profile.RenewalSoon:
		return pickerWarningStyle.Render(label)
	default:
		return label
	}
}

func accountLabel(p profile.Meta) string {
	if p.Email != "" {
		return p.Email
	}
	return p.Name
}

func planLabel(p profile.Meta) string {
	if p.Plan == "" {
		return "-"
	}
	return p.Plan
}

func relativeLastUsed(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}

	elapsed := time.Since(t)
	if elapsed < time.Minute {
		return "Now"
	}
	if elapsed < time.Hour {
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	}
	if elapsed < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
}

func padRight(s string, width int) string {
	padding := width - lipgloss.Width(s)
	if padding <= 0 {
		return s
	}
	return s + strings.Repeat(" ", padding)
}
