package cmd

import "testing"

func TestValidateSaveStateRefusesRunningClaudeEvenWithForce(t *testing.T) {
	for _, force := range []bool{false, true} {
		if err := validateSaveState(true, force); err == nil {
			t.Fatalf("running save with force=%v should fail", force)
		}
	}
}

func TestValidateSaveStateAllowsQuiescentCheckpoint(t *testing.T) {
	if err := validateSaveState(false, false); err != nil {
		t.Fatalf("quiescent save: %v", err)
	}
}
