package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectorSnapshot(t *testing.T) {
	root := t.TempDir()
	sessionStore := filepath.Join(root, "sessions.json")
	logPath := filepath.Join(root, "askplanner.log")
	workspaceRoot := filepath.Join(root, "workspaces")

	sessionData := `{
  "cli:default": {
    "conversation_key": "cli:default",
    "session_id": "session-cli",
    "work_dir": "/tmp/cli",
    "created_at": "2026-03-24T09:00:00Z",
    "last_active_at": "2026-03-24T09:55:00Z",
    "turn_count": 3,
    "model_override": "gpt-5.3-codex",
    "turns": [{"user":"show status","assistant":"ok","at":"2026-03-24T09:55:00Z"}]
  },
  "lark:chat:abc:user:u1": {
    "conversation_key": "lark:chat:abc:user:u1",
    "user_key": "u1",
    "session_id": "session-lark",
    "work_dir": "/tmp/lark",
    "created_at": "2026-03-24T08:30:00Z",
    "last_active_at": "2026-03-24T09:58:00Z",
    "turn_count": 7,
    "last_error": "resume failed once",
    "turns": [{"user":"optimize this sql","assistant":"ok","at":"2026-03-24T09:58:00Z"}]
  }
}`
	if err := os.WriteFile(sessionStore, []byte(sessionData), 0o644); err != nil {
		t.Fatalf("write session store: %v", err)
	}

	metaPath := filepath.Join(workspaceRoot, "users", "u1", "data")
	if err := os.MkdirAll(metaPath, 0o755); err != nil {
		t.Fatalf("mkdir workspace metadata: %v", err)
	}
	workspaceData := `{
  "user_key": "u1",
  "last_active_at": "2026-03-24T09:57:00Z",
  "repos": {
    "tidb": {
      "name": "tidb",
      "relative_path": "contrib/tidb",
      "requested_ref": "release-8.5"
    }
  }
}`
	if err := os.WriteFile(filepath.Join(metaPath, "workspace.json"), []byte(workspaceData), 0o644); err != nil {
		t.Fatalf("write workspace metadata: %v", err)
	}

	logData := "" +
		"2026/03/24 17:58:10 main.go:172: [askplanner] request done conversation=cli:default elapsed=850ms\n" +
		"2026/03/24 17:58:20 handler.go:47: [larkbot] handle event done message_id=m1 conversation=lark:chat:abc:user:u1 elapsed=1.7s\n" +
		"2026/03/24 17:58:30 handler.go:133: [larkbot] handle event error: upstream timeout (message_id=m2)\n"
	if err := os.WriteFile(logPath, []byte(logData), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	now := time.Date(2026, 3, 24, 18, 0, 0, 0, time.Local)
	collector := &Collector{
		sessionStorePath: sessionStore,
		logPath:          logPath,
		workspaceRoot:    workspaceRoot,
		sessionTTL:       24 * time.Hour,
		logTailBytes:     1 << 20,
		now:              func() time.Time { return now },
	}

	snapshot, err := collector.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if snapshot.Summary.TotalConversations != 2 {
		t.Fatalf("total conversations = %d, want 2", snapshot.Summary.TotalConversations)
	}
	if snapshot.Summary.Active15Min != 2 {
		t.Fatalf("active 15m = %d, want 2", snapshot.Summary.Active15Min)
	}
	if snapshot.Summary.ErrorSessions != 1 {
		t.Fatalf("error sessions = %d, want 1", snapshot.Summary.ErrorSessions)
	}
	if snapshot.Summary.WorkspaceUsers != 1 {
		t.Fatalf("workspace users = %d, want 1", snapshot.Summary.WorkspaceUsers)
	}
	if snapshot.RequestStats.Requests1Hour != 2 {
		t.Fatalf("requests 1h = %d, want 2", snapshot.RequestStats.Requests1Hour)
	}
	if snapshot.RequestStats.Errors1Hour != 1 {
		t.Fatalf("errors 1h = %d, want 1", snapshot.RequestStats.Errors1Hour)
	}
	if len(snapshot.RepoBreakdown) != 1 || snapshot.RepoBreakdown[0].Name != "tidb" {
		t.Fatalf("repo breakdown = %+v, want tidb", snapshot.RepoBreakdown)
	}
	if len(snapshot.RecentRequests) != 2 {
		t.Fatalf("recent requests = %d, want 2", len(snapshot.RecentRequests))
	}
	if len(snapshot.RecentErrors) != 1 {
		t.Fatalf("recent errors = %d, want 1", len(snapshot.RecentErrors))
	}
}
