package workspace

import (
	"strings"
	"testing"
)

func TestFormatStatusRedactsAskplannerPrefix(t *testing.T) {
	ws := &Workspace{
		RootDir:         "/home/gjt/work/staging_askplanner/.askplanner/workspaces/users/ou_ec47baa513ddbbc97d48d6f6f0fd22a1/root",
		UserFilesDir:    "/home/gjt/work/staging_askplanner/.askplanner/workspaces/users/ou_ec47baa513ddbbc97d48d6f6f0fd22a1/root/user-files",
		ClinicFilesDir:  "/home/gjt/work/staging_askplanner/.askplanner/workspaces/users/ou_ec47baa513ddbbc97d48d6f6f0fd22a1/root/clinic-files",
		EnvironmentHash: "204de1ba4afd189bcf7f98f2c156c4177391a2aade7321ae8ed9e44db0342281",
		Repos: []RepoState{{
			Name:         "tidb",
			RelativePath: "contrib/tidb",
			RequestedRef: "release-8.5",
			ResolvedSHA:  "499c8777ea5c1234567890",
		}},
	}

	status := FormatStatus(ws)

	if strings.Contains(status, "/home/gjt/work/staging_askplanner/.askplanner/") {
		t.Fatalf("status leaked absolute askplanner path: %s", status)
	}
	wantSnippets := []string{
		"- Root: workspaces/users/ou_ec47baa513ddbbc97d48d6f6f0fd22a1/root",
		"- User Files: workspaces/users/ou_ec47baa513ddbbc97d48d6f6f0fd22a1/root/user-files",
		"- Clinic Files: workspaces/users/ou_ec47baa513ddbbc97d48d6f6f0fd22a1/root/clinic-files",
		"- Environment Hash: 204de1ba4afd189bcf7f98f2c156c4177391a2aade7321ae8ed9e44db0342281",
		"  - contrib/tidb ref=release-8.5 sha=499c8777ea5c",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(status, snippet) {
			t.Fatalf("status missing %q:\n%s", snippet, status)
		}
	}
}

func TestFormatStatusKeepsPathsWithoutAskplannerPrefix(t *testing.T) {
	ws := &Workspace{
		RootDir:         "/tmp/workspaces/users/test/root",
		UserFilesDir:    "/tmp/workspaces/users/test/root/user-files",
		ClinicFilesDir:  "/tmp/workspaces/users/test/root/clinic-files",
		EnvironmentHash: "env-hash",
	}

	status := FormatStatus(ws)

	wantSnippets := []string{
		"- Root: /tmp/workspaces/users/test/root",
		"- User Files: /tmp/workspaces/users/test/root/user-files",
		"- Clinic Files: /tmp/workspaces/users/test/root/clinic-files",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(status, snippet) {
			t.Fatalf("status missing %q:\n%s", snippet, status)
		}
	}
}
