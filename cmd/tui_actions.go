package cmd

type actionSpec struct {
	label       string
	confirm     string
	progress    string
	success     string
	destructive bool
	run         func(b tuiBackend, name string) error
}

// Every template takes the profile name as its only %q argument. An empty
// template is never shown: Add hands off to the login prompt instead of a
// result, and FinishAdd/CancelAdd run without a confirmation.
var tuiActions = map[tuiAction]actionSpec{
	tuiActionActivate: {
		label:    "Activate",
		confirm:  "Activate %q? Claude Desktop will stop while the account is switched.",
		progress: "Activating %q...",
		success:  "Profile %q activated.",
		run:      tuiBackend.Activate,
	},
	tuiActionSave: {
		label:    "Save",
		confirm:  "Save the current session as %q? An existing profile will be replaced.",
		progress: "Saving %q...",
		success:  "Profile %q saved.",
		run:      tuiBackend.Save,
	},
	tuiActionAdd: {
		label:    "Add",
		confirm:  "Add %q? Current session state will be checkpointed before Claude opens signed out.",
		progress: "Preparing %q for login...",
		run:      tuiBackend.BeginAdd,
	},
	tuiActionFinishAdd: {
		label:    "Add",
		progress: "Saving %q...",
		success:  "Profile %q added.",
		run:      tuiBackend.FinishAdd,
	},
	tuiActionCancelAdd: {
		label:    "Cancel add",
		progress: "Cancelling %q and restoring the previous session...",
		success:  "Adding %q was cancelled. The previous session was restored.",
		run:      func(b tuiBackend, _ string) error { return b.CancelAdd() },
	},
	tuiActionDelete: {
		label:       "Delete",
		confirm:     "Permanently delete profile %q?",
		progress:    "Deleting %q...",
		success:     "Profile %q deleted.",
		destructive: true,
		run:         tuiBackend.Delete,
	},
}
