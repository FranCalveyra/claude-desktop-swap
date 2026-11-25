package cmd

import "github.com/charmbracelet/lipgloss"

var (
	uiTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	uiHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	uiHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	uiBadgeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	uiCursorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	uiSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	uiPromptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	uiProgressStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	uiUsableStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	uiWarningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	uiErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)
