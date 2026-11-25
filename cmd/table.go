package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	"github.com/charmbracelet/lipgloss"
)

// profileTable is the profile list shared by the use picker and the TUI
// dashboard. They differ only in the lead column: a row number in the picker,
// the profile name in the dashboard.
type profileTable struct {
	profiles  []profile.Meta
	current   string
	cursor    int
	now       time.Time
	leadTitle string
	lead      func(i int, p profile.Meta) string
}

type tableWidths struct {
	lead, account, plan, health, expires, last int
}

func (t profileTable) render(b *strings.Builder) {
	w := t.widths()
	b.WriteString("    ")
	b.WriteString(uiHeaderStyle.Render(fmt.Sprintf("%s  %s  %s  %s  %s  %s",
		padRight(t.leadTitle, w.lead),
		padRight("ACCOUNT", w.account),
		padRight("PLAN", w.plan),
		padRight("HEALTH", w.health),
		padRight("EXPIRES", w.expires),
		padRight("LAST", w.last),
	)))
	b.WriteString("\n")

	for i, p := range t.profiles {
		cursor := " "
		lead := padRight(t.lead(i, p), w.lead)
		account := padRight(accountLabel(p), w.account)
		plan := padRight(planLabel(p), w.plan)
		last := padRight(relativeLastUsed(p.LastUsed), w.last)
		if i == t.cursor {
			cursor = uiCursorStyle.Render(">")
			lead = uiSelectedStyle.Render(lead)
			account = uiSelectedStyle.Render(account)
			plan = uiSelectedStyle.Render(plan)
			last = uiSelectedStyle.Render(last)
		}
		badge := ""
		if p.Name == t.current {
			badge = "  " + uiBadgeStyle.Render("[ACTIVE]")
		}
		fmt.Fprintf(b, "  %s %s  %s  %s  %s  %s  %s%s\n",
			cursor,
			lead,
			account,
			plan,
			padRight(styledHealthLabel(p.ObservedHealth), w.health),
			padRight(styledExpiryLabel(p, t.now), w.expires),
			last,
			badge,
		)
	}
}

func (t profileTable) widths() tableWidths {
	w := tableWidths{
		lead:    lipgloss.Width(t.leadTitle),
		account: lipgloss.Width("ACCOUNT"),
		plan:    8,
		health:  lipgloss.Width("HEALTH"),
		expires: lipgloss.Width("EXPIRES"),
		last:    6,
	}
	for i, p := range t.profiles {
		w.lead = max(w.lead, lipgloss.Width(t.lead(i, p)))
		w.account = max(w.account, lipgloss.Width(accountLabel(p)))
		w.plan = max(w.plan, lipgloss.Width(planLabel(p)))
		w.health = max(w.health, lipgloss.Width(healthLabel(p.ObservedHealth)))
		w.expires = max(w.expires, lipgloss.Width(asciiExpiryLabel(p, t.now)))
		w.last = max(w.last, lipgloss.Width(relativeLastUsed(p.LastUsed)))
	}
	return w
}

func styledHealthLabel(health profile.Health) string {
	label := healthLabel(health)
	switch health {
	case profile.HealthUsable:
		return uiUsableStyle.Render(label)
	case profile.HealthExpired, profile.HealthMissing:
		return uiErrorStyle.Render(label)
	case profile.HealthUnknown:
		return uiWarningStyle.Render(label)
	default:
		return label
	}
}

// asciiExpiryLabel swaps the list's "⚠" for "!": terminals disagree on the
// width of "⚠", which would misalign the padded columns.
func asciiExpiryLabel(p profile.Meta, now time.Time) string {
	return strings.ReplaceAll(metaExpiryLabel(p, now), " ⚠", " !")
}

func styledExpiryLabel(p profile.Meta, now time.Time) string {
	label := asciiExpiryLabel(p, now)
	switch renewalFor(p, now) {
	case profile.RenewalExpired:
		return uiErrorStyle.Render(label)
	case profile.RenewalSoon:
		return uiWarningStyle.Render(label)
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
