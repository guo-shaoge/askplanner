package selfcmd

import "testing"

func TestIsWhoAmI(t *testing.T) {
	if !IsWhoAmI("/whoami") {
		t.Fatalf("expected /whoami to match")
	}
	if !IsWhoAmI(" /WHOAMI ") {
		t.Fatalf("expected case-insensitive /whoami to match")
	}
	if IsWhoAmI("/whoami now") {
		t.Fatalf("expected extra tokens to be rejected")
	}
}
