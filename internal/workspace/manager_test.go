package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lab/askplanner/internal/config"
)

func TestParseCommand(t *testing.T) {
	cmd, matched, err := ParseCommand("/ws switch tidb release-8.5 -- analyze this query")
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if !matched {
		t.Fatalf("expected workspace command to match")
	}
	if cmd.Action != "switch" || cmd.Repo != "tidb" || cmd.Ref != "release-8.5" || cmd.Question != "analyze this query" {
		t.Fatalf("unexpected command: %+v", cmd)
	}
}

func TestEnsureSwitchAndAgentRulesRefresh(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	createTestRepo(t, root, "contrib/tidb", "main", []string{"release-8.5"})
	createTestRepo(t, root, "contrib/tidb-docs", "main", []string{"release-8.5"})
	agent := createTestRepo(t, root, "contrib/agent-rules", "main", nil)

	manager := newTestManager(t, root)

	ws, err := manager.Ensure(ctx, "ou_test-user")
	if err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	if len(ws.Repos) != 3 {
		t.Fatalf("repo count = %d, want 3", len(ws.Repos))
	}
	if _, err := os.Lstat(filepath.Join(ws.RootDir, "user-files")); err != nil {
		t.Fatalf("expected user-files symlink: %v", err)
	}

	switched, changed, err := manager.SwitchRepo(ctx, "ou_test-user", "tidb", "release-8.5")
	if err != nil {
		t.Fatalf("switch repo: %v", err)
	}
	if !changed {
		t.Fatalf("expected switch to report environment change")
	}
	if switched.EnvironmentHash == ws.EnvironmentHash {
		t.Fatalf("expected environment hash to change after repo switch")
	}
	if got := findRepo(switched, "tidb").RequestedRef; got != "release-8.5" {
		t.Fatalf("tidb requested ref = %q, want release-8.5", got)
	}
	if got := findRepo(switched, "tidb-docs").RequestedRef; got != "release-8.5" {
		t.Fatalf("tidb-docs requested ref = %q, want release-8.5", got)
	}

	oldAgentSHA := findRepo(switched, "agent-rules").ResolvedSHA
	agent.commit(t, "main", "second\n")
	if err := manager.syncAgentRulesMirror(ctx); err != nil {
		t.Fatalf("sync agent-rules mirror: %v", err)
	}
	refreshed, err := manager.Ensure(ctx, "ou_test-user")
	if err != nil {
		t.Fatalf("ensure workspace after agent sync: %v", err)
	}
	newAgentSHA := findRepo(refreshed, "agent-rules").ResolvedSHA
	if newAgentSHA == oldAgentSHA {
		t.Fatalf("expected agent-rules SHA to change after mirror refresh")
	}
}

func TestSweepRemovesExpiredWorkspace(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	createTestRepo(t, root, "contrib/tidb", "main", nil)
	createTestRepo(t, root, "contrib/tidb-docs", "main", nil)
	createTestRepo(t, root, "contrib/agent-rules", "main", nil)

	manager := newTestManager(t, root)
	ws, err := manager.Ensure(ctx, "ou_gc-user")
	if err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}

	metaPath := filepath.Join(manager.usersDir, "ou_gc-user", "data", metadataFileName)
	meta, err := loadMetadataFile(metaPath)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	meta.LastActiveAt = time.Now().Add(-2 * time.Hour).UTC()
	if err := saveMetadataFile(metaPath, meta); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	manager.idleTTL = time.Hour

	if err := manager.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if _, err := os.Stat(ws.RootDir); !os.IsNotExist(err) {
		t.Fatalf("workspace root still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(manager.uploadRoot, "ou_gc-user")); !os.IsNotExist(err) {
		t.Fatalf("upload dir still exists: %v", err)
	}
}

func TestResetUserRemovesWorkspaceAndStateDirs(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	createTestRepo(t, root, "contrib/tidb", "main", nil)
	createTestRepo(t, root, "contrib/tidb-docs", "main", nil)
	createTestRepo(t, root, "contrib/agent-rules", "main", nil)

	manager := newTestManager(t, root)
	ws, err := manager.Ensure(ctx, "ou_reset-user")
	if err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}

	resetRoot, err := manager.ResetUser(ctx, "ou_reset-user")
	if err != nil {
		t.Fatalf("reset user: %v", err)
	}
	if resetRoot != ws.RootDir {
		t.Fatalf("reset root = %q, want %q", resetRoot, ws.RootDir)
	}
	for _, path := range []string{
		ws.RootDir,
		filepath.Join(manager.usersDir, "ou_reset-user"),
		filepath.Join(manager.uploadRoot, "ou_reset-user"),
		filepath.Join(manager.clinicRoot, "ou_reset-user"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("path still exists after reset: %s err=%v", path, err)
		}
	}
}

func findRepo(ws *Workspace, name string) RepoState {
	for _, repo := range ws.Repos {
		if repo.Name == name {
			return repo
		}
	}
	return RepoState{}
}

func newTestManager(t *testing.T, root string) *Manager {
	t.Helper()
	manager, err := NewManager(&config.Config{
		ProjectRoot:                       root,
		WorkspaceRoot:                     filepath.Join(root, "workspaces"),
		FeishuFileDir:                     filepath.Join(root, "uploads"),
		ClinicStoreDir:                    filepath.Join(root, "clinic"),
		WorkspaceIdleTTLHours:             1,
		WorkspaceGCIntervalMin:            1,
		AgentRulesSyncIntervalMin:         1,
		WorkspaceRepoTidbURL:              "unused",
		WorkspaceRepoTidbDefaultRef:       "main",
		WorkspaceRepoTidbDocsURL:          "unused",
		WorkspaceRepoTidbDocsDefaultRef:   "main",
		WorkspaceRepoAgentRulesURL:        "unused",
		WorkspaceRepoAgentRulesDefaultRef: "main",
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	return manager
}

type testRepo struct {
	path string
}

func createTestRepo(t *testing.T, root, relativePath, defaultBranch string, extraBranches []string) testRepo {
	t.Helper()
	ctx := context.Background()
	repoPath := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(repoPath), 0o755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", relativePath, err)
	}
	if _, err := runGit(ctx, "", "init", "-b", defaultBranch, repoPath); err != nil {
		t.Fatalf("git init %s: %v", relativePath, err)
	}
	if _, err := runGit(ctx, repoPath, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config email: %v", err)
	}
	if _, err := runGit(ctx, repoPath, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config name: %v", err)
	}
	writeRepoFile(t, repoPath, "README.md", relativePath+"\n")
	if _, err := runGit(ctx, repoPath, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := runGit(ctx, repoPath, "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	for _, branch := range extraBranches {
		if _, err := runGit(ctx, repoPath, "checkout", "-B", branch); err != nil {
			t.Fatalf("git checkout %s: %v", branch, err)
		}
		writeRepoFile(t, repoPath, "BRANCH.txt", branch+"\n")
		if _, err := runGit(ctx, repoPath, "add", "."); err != nil {
			t.Fatalf("git add branch %s: %v", branch, err)
		}
		if _, err := runGit(ctx, repoPath, "commit", "-m", "branch "+branch); err != nil {
			t.Fatalf("git commit branch %s: %v", branch, err)
		}
	}
	if _, err := runGit(ctx, repoPath, "checkout", defaultBranch); err != nil {
		t.Fatalf("git checkout default branch: %v", err)
	}
	return testRepo{path: repoPath}
}

func (r testRepo) commit(t *testing.T, branch, content string) {
	t.Helper()
	ctx := context.Background()
	if _, err := runGit(ctx, r.path, "checkout", branch); err != nil {
		t.Fatalf("git checkout %s: %v", branch, err)
	}
	writeRepoFile(t, r.path, "README.md", strings.TrimSpace(content)+"\n")
	if _, err := runGit(ctx, r.path, "add", "README.md"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := runGit(ctx, r.path, "commit", "-m", "update "+branch); err != nil {
		t.Fatalf("git commit: %v", err)
	}
}

func writeRepoFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
