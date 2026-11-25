package cmd

import "testing"

func TestRootUsesTUIOnlyForBareInvocation(t *testing.T) {
	if root.RunE == nil {
		t.Fatal("root has no TUI entry point")
	}
	if err := root.Args(root, []string{"unexpected"}); err == nil {
		t.Fatal("root accepted positional arguments")
	}
	for _, command := range []string{"save", "add", "use", "list", "delete", "status"} {
		found, _, err := root.Find([]string{command})
		if err != nil || found.Name() != command {
			t.Fatalf("subcommand %q unavailable: found=%v err=%v", command, found, err)
		}
	}
}
