package modelcmd

import "testing"

func TestParseCommandStatusDefault(t *testing.T) {
	cmd, matched, err := ParseCommand("/model")
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if !matched {
		t.Fatalf("expected model command to match")
	}
	if cmd.Action != "status" {
		t.Fatalf("action = %q, want status", cmd.Action)
	}
}

func TestParseCommandSetWithQuestion(t *testing.T) {
	cmd, matched, err := ParseCommand("/model set gpt-5.4 -- analyze this query")
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if !matched {
		t.Fatalf("expected model command to match")
	}
	if cmd.Action != "set" || cmd.Model != "gpt-5.4" || cmd.Question != "analyze this query" {
		t.Fatalf("unexpected command: %+v", cmd)
	}
}

func TestParseCommandShorthandSet(t *testing.T) {
	cmd, matched, err := ParseCommand("/model gpt-5.4")
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if !matched {
		t.Fatalf("expected model command to match")
	}
	if cmd.Action != "set" || cmd.Model != "gpt-5.4" {
		t.Fatalf("unexpected command: %+v", cmd)
	}
}

func TestParseCommandEffort(t *testing.T) {
	cmd, matched, err := ParseCommand("/model effort high -- analyze this query")
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if !matched {
		t.Fatalf("expected model command to match")
	}
	if cmd.Action != "effort" || cmd.Effort != "high" || cmd.Question != "analyze this query" {
		t.Fatalf("unexpected command: %+v", cmd)
	}
}

func TestParseCommandResetEffort(t *testing.T) {
	cmd, matched, err := ParseCommand("/model effort reset")
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if !matched {
		t.Fatalf("expected model command to match")
	}
	if cmd.Action != "reset-effort" {
		t.Fatalf("unexpected command: %+v", cmd)
	}
}
