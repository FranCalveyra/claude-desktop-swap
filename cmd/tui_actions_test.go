package cmd

import (
	"strings"
	"testing"
)

func TestEveryTUIActionIsFullySpecified(t *testing.T) {
	for action := tuiActionNone + 1; action <= tuiActionCancelAdd; action++ {
		spec, ok := tuiActions[action]
		if !ok || spec.run == nil || spec.label == "" || spec.progress == "" {
			t.Fatalf("action %d is missing from tuiActions or incomplete: %+v", action, spec)
		}
		for _, template := range []string{spec.confirm, spec.progress, spec.success} {
			if template != "" && strings.Count(template, "%q") != 1 {
				t.Fatalf("action %d template %q must take exactly one %%q", action, template)
			}
		}
	}
	for _, action := range []tuiAction{tuiActionActivate, tuiActionSave, tuiActionAdd, tuiActionDelete} {
		if tuiActions[action].confirm == "" {
			t.Fatalf("confirmable action %d has no confirmation prompt", action)
		}
	}
}
