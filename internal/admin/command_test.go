package admin

import "testing"

func TestParseCommand(t *testing.T) {
	cmd, matched, err := ParseCommand("/admin reset-user ou_test")
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if !matched {
		t.Fatalf("expected admin command to match")
	}
	if cmd.Action != "reset-user" || cmd.UserKey != "ou_test" {
		t.Fatalf("unexpected command: %+v", cmd)
	}
}
