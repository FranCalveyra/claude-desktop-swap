package cmd

import (
	"fmt"
	"strings"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	"github.com/charmbracelet/lipgloss"
)

func (m tuiModel) View() string {
	switch m.mode {
	case tuiDashboard:
		return m.dashboardView()
	case tuiInput:
		return m.inputView()
	case tuiConfirm:
		return m.confirmView()
	case tuiResult:
		return m.resultView()
	case tuiAddLogin:
		return m.addLoginView()
	default:
		help := "Please wait. Ctrl+C quits."
		if m.running() {
			help = "Please wait. Quitting is disabled until this finishes."
		}
		return panelView("Claude Desktop Swap", m.message, help, uiProgressStyle)
	}
}

func (m tuiModel) dashboardView() string {
	var b strings.Builder
	b.WriteString("\n  ")
	b.WriteString(uiTitleStyle.Render("Claude Desktop Swap"))
	b.WriteString("\n  ")
	b.WriteString(styledStatus(m.status))
	b.WriteString("\n\n  ")
	b.WriteString(uiHeaderStyle.Render("Profiles"))
	b.WriteString("\n")
	profileTable{
		profiles:  m.profiles,
		current:   m.current,
		cursor:    m.cursor,
		now:       m.now,
		leadTitle: "PROFILE",
		lead:      func(_ int, p profile.Meta) string { return p.Name },
	}.render(&b)

	if len(m.profiles) == 0 {
		b.WriteString("\n  ")
		b.WriteString(uiPromptStyle.Render("No profiles saved. Press s to save the current session or a to add an account."))
		b.WriteString("\n")
	}

	b.WriteString("\n  ")
	b.WriteString(uiHelpStyle.Render("Keys: Up/Down or j/k, Enter/u activate, s save, a add, d delete, r refresh, q quit"))
	b.WriteString("\n")
	return b.String()
}

func (m tuiModel) inputView() string {
	return panelView("Claude Desktop Swap", fmt.Sprintf("%s profile name: %s", tuiActions[m.pending].label, m.name), "Enter continues. Esc cancels.", uiPromptStyle)
}

func (m tuiModel) confirmView() string {
	spec := tuiActions[m.pending]
	help := "Press y or Enter to continue. Press n or Esc to cancel. q quits."
	if spec.destructive {
		help = "Press y to " + strings.ToLower(spec.label) + ". Press n or Esc to cancel. q quits."
	}
	return panelView("Confirm", fmt.Sprintf(spec.confirm, m.name), help, uiPromptStyle)
}

func (m tuiModel) resultView() string {
	style := uiUsableStyle
	if m.failed {
		style = uiErrorStyle
	}
	return panelView("Result", m.message, "Press Enter to refresh the dashboard. q quits.", style)
}

func (m tuiModel) addLoginView() string {
	message := m.message + " Log in there, then return here."
	return panelView("Add account", message, "Press Enter after login to save the profile. Esc cancels and restores the previous session.", uiPromptStyle)
}

func panelView(title, message, help string, messageStyle lipgloss.Style) string {
	return "\n  " + uiTitleStyle.Render(title) + "\n\n  " + messageStyle.Render(message) + "\n\n  " + uiHelpStyle.Render(help) + "\n"
}

func styledStatus(r statusReport) string {
	lines := []string{uiPromptStyle.Render(r.line)}
	for _, w := range r.warnings {
		lines = append(lines, uiWarningStyle.Render("Warning: "+w))
	}
	return strings.Join(lines, "\n  ")
}
