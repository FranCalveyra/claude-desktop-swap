package cmd

import (
	"fmt"
	"strings"

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
		return panelView("Claude Desktop Swap", m.message, "Please wait. Ctrl+C quits.", pickerProgressStyle)
	}
}

func (m tuiModel) dashboardView() string {
	var b strings.Builder
	profileWidth, accountWidth, planWidth, healthWidth, expiresWidth, lastWidth := m.dashboardWidths()

	b.WriteString("\n  ")
	b.WriteString(pickerTitleStyle.Render("Claude Desktop Swap"))
	b.WriteString("\n  ")
	b.WriteString(styledStatus(m.status))
	b.WriteString("\n\n  ")
	b.WriteString(pickerHeaderStyle.Render("Profiles"))
	b.WriteString("\n  ")
	b.WriteString(pickerHeaderStyle.Render(fmt.Sprintf(
		"     %s  %s  %s  %s  %s  %s",
		padRight("PROFILE", profileWidth),
		padRight("ACCOUNT", accountWidth),
		padRight("PLAN", planWidth),
		padRight("HEALTH", healthWidth),
		padRight("EXPIRES", expiresWidth),
		padRight("LAST", lastWidth),
	)))
	b.WriteString("\n")

	if len(m.profiles) == 0 {
		b.WriteString("\n  ")
		b.WriteString(pickerPromptStyle.Render("No profiles saved. Press s to save the current session or a to add an account."))
		b.WriteString("\n")
	}
	for i, p := range m.profiles {
		cursor := " "
		if i == m.cursor {
			cursor = pickerCursorStyle.Render(">")
		}
		name := padRight(p.Name, profileWidth)
		account := padRight(accountLabel(p), accountWidth)
		plan := padRight(planLabel(p), planWidth)
		last := padRight(relativeLastUsed(p.LastUsed), lastWidth)
		if i == m.cursor {
			name = pickerSelectedStyle.Render(name)
			account = pickerSelectedStyle.Render(account)
			plan = pickerSelectedStyle.Render(plan)
			last = pickerSelectedStyle.Render(last)
		}
		badge := ""
		if p.Name == m.current {
			badge = "  " + pickerBadgeStyle.Render("[ACTIVE]")
		}
		fmt.Fprintf(&b, "  %s  %s  %s  %s  %s  %s  %s%s\n",
			cursor,
			name,
			account,
			plan,
			padRight(styledHealthLabel(p.ObservedHealth), healthWidth),
			padRight(styledExpiryLabel(p, m.now), expiresWidth),
			last,
			badge,
		)
	}

	b.WriteString("\n  ")
	b.WriteString(pickerHelpStyle.Render("Keys: Up/Down or j/k, Enter/u activate, s save, a add, d delete, r refresh, q quit"))
	b.WriteString("\n")
	return b.String()
}

func (m tuiModel) dashboardWidths() (profileWidth, accountWidth, planWidth, healthWidth, expiresWidth, lastWidth int) {
	profileWidth = lipgloss.Width("PROFILE")
	accountWidth = lipgloss.Width("ACCOUNT")
	planWidth = lipgloss.Width("PLAN")
	healthWidth = lipgloss.Width("HEALTH")
	expiresWidth = lipgloss.Width("EXPIRES")
	lastWidth = lipgloss.Width("LAST")
	for _, p := range m.profiles {
		profileWidth = max(profileWidth, lipgloss.Width(p.Name))
		accountWidth = max(accountWidth, lipgloss.Width(accountLabel(p)))
		planWidth = max(planWidth, lipgloss.Width(planLabel(p)))
		healthWidth = max(healthWidth, lipgloss.Width(healthLabel(p.ObservedHealth)))
		expiresWidth = max(expiresWidth, lipgloss.Width(asciiExpiryLabel(p, m.now)))
		lastWidth = max(lastWidth, lipgloss.Width(relativeLastUsed(p.LastUsed)))
	}
	return profileWidth, accountWidth, planWidth, healthWidth, expiresWidth, lastWidth
}

func (m tuiModel) inputView() string {
	return panelView("Claude Desktop Swap", fmt.Sprintf("%s profile name: %s", actionLabel(m.pending), m.name), "Enter continues. Esc cancels.", pickerPromptStyle)
}

func (m tuiModel) confirmView() string {
	var prompt string
	switch m.pending {
	case tuiActionActivate:
		prompt = fmt.Sprintf("Activate %q? Claude Desktop will stop while the account is switched.", m.name)
	case tuiActionSave:
		prompt = fmt.Sprintf("Save the current session as %q? An existing profile will be replaced.", m.name)
	case tuiActionAdd:
		prompt = fmt.Sprintf("Add %q? Current session state will be checkpointed before Claude opens signed out.", m.name)
	case tuiActionDelete:
		prompt = fmt.Sprintf("Permanently delete profile %q?", m.name)
	}
	return panelView("Confirm", prompt, "Press y or Enter to continue. Press n or Esc to cancel. q quits.", pickerPromptStyle)
}

func (m tuiModel) resultView() string {
	style := pickerUsableStyle
	if m.failed {
		style = pickerErrorStyle
	}
	return panelView("Result", m.message, "Press Enter to refresh the dashboard. q quits.", style)
}

func (m tuiModel) addLoginView() string {
	message := m.message + " Log in there, then return here."
	return panelView("Add account", message, "Press Enter after login to save the profile. q quits.", pickerPromptStyle)
}

func panelView(title, message, help string, messageStyle lipgloss.Style) string {
	return "\n  " + pickerTitleStyle.Render(title) + "\n\n  " + messageStyle.Render(message) + "\n\n  " + pickerHelpStyle.Render(help) + "\n"
}

func styledStatus(status string) string {
	lines := strings.Split(status, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "Warning:") {
			lines[i] = pickerWarningStyle.Render(line)
		} else {
			lines[i] = pickerPromptStyle.Render(line)
		}
	}
	return strings.Join(lines, "\n  ")
}

func actionLabel(action tuiAction) string {
	switch action {
	case tuiActionActivate:
		return "Activate"
	case tuiActionSave:
		return "Save"
	case tuiActionAdd, tuiActionFinishAdd:
		return "Add"
	case tuiActionDelete:
		return "Delete"
	default:
		return "Operation"
	}
}

func progressMessage(action tuiAction, name string) string {
	switch action {
	case tuiActionActivate:
		return fmt.Sprintf("Activating %q...", name)
	case tuiActionSave:
		return fmt.Sprintf("Saving %q...", name)
	case tuiActionAdd:
		return fmt.Sprintf("Preparing %q for login...", name)
	case tuiActionFinishAdd:
		return fmt.Sprintf("Saving %q...", name)
	case tuiActionDelete:
		return fmt.Sprintf("Deleting %q...", name)
	default:
		return "Working..."
	}
}

func successMessage(action tuiAction, name string) string {
	switch action {
	case tuiActionActivate:
		return fmt.Sprintf("Profile %q activated.", name)
	case tuiActionSave:
		return fmt.Sprintf("Profile %q saved.", name)
	case tuiActionFinishAdd:
		return fmt.Sprintf("Profile %q added.", name)
	case tuiActionDelete:
		return fmt.Sprintf("Profile %q deleted.", name)
	default:
		return "Operation completed."
	}
}
