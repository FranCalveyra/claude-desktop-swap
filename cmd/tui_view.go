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
	return panelView("Claude Desktop Swap", fmt.Sprintf("%s profile name: %s", actionLabel(m.pending), m.name), "Enter continues. Esc cancels.", uiPromptStyle)
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
	help := "Press y or Enter to continue. Press n or Esc to cancel. q quits."
	if m.pending == tuiActionDelete {
		help = "Press y to delete. Press n or Esc to cancel. q quits."
	}
	return panelView("Confirm", prompt, help, uiPromptStyle)
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

func actionLabel(action tuiAction) string {
	switch action {
	case tuiActionActivate:
		return "Activate"
	case tuiActionSave:
		return "Save"
	case tuiActionAdd, tuiActionFinishAdd:
		return "Add"
	case tuiActionCancelAdd:
		return "Cancel add"
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
	case tuiActionCancelAdd:
		return "Cancelling and restoring the previous session..."
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
	case tuiActionCancelAdd:
		return fmt.Sprintf("Adding %q was cancelled. The previous session was restored.", name)
	case tuiActionDelete:
		return fmt.Sprintf("Profile %q deleted.", name)
	default:
		return "Operation completed."
	}
}
