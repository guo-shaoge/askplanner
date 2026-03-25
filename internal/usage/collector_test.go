package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectorSnapshotAndPages(t *testing.T) {
	root := t.TempDir()
	sessionStore := filepath.Join(root, "sessions.json")
	logPath := filepath.Join(root, "askplanner.log")
	workspaceRoot := filepath.Join(root, "workspaces")
	questionPath := filepath.Join(root, "usage_questions.jsonl")

	sessionData := `{
  "cli:default": {
    "conversation_key": "cli:default",
    "user_key": "cli_default",
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

	store := &QuestionStore{path: questionPath, sessionStore: sessionStore}
	if err := store.BackfillFromSessions(); err != nil {
		t.Fatalf("backfill questions: %v", err)
	}
	if err := store.Append(QuestionEvent{
		EventID:          "live-1",
		AskedAt:          time.Date(2026, 3, 24, 9, 59, 0, 0, time.UTC),
		Source:           sourceLark,
		UserKey:          "u1",
		ConversationKey:  "lark:chat:abc:user:u1",
		Question:         "explain index merge",
		Status:           statusSuccess,
		DurationMs:       1900,
		Model:            "gpt-5.3-codex",
		WorkspaceEnvHash: "env1",
	}); err != nil {
		t.Fatalf("append question event: %v", err)
	}

	now := time.Date(2026, 3, 24, 18, 0, 0, 0, time.Local)
	collector := &Collector{
		sessionStorePath: sessionStore,
		logPath:          logPath,
		workspaceRoot:    workspaceRoot,
		sessionTTL:       24 * time.Hour,
		logTailBytes:     1 << 20,
		questionStore:    store,
		now:              func() time.Time { return now },
	}

	snapshot, err := collector.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if snapshot.Summary.TotalConversations != 2 {
		t.Fatalf("total conversations = %d, want 2", snapshot.Summary.TotalConversations)
	}
	if snapshot.Summary.TotalUsers != 2 {
		t.Fatalf("total users = %d, want 2", snapshot.Summary.TotalUsers)
	}
	if snapshot.Summary.TotalQuestions != 3 {
		t.Fatalf("total questions = %d, want 3", snapshot.Summary.TotalQuestions)
	}
	if len(snapshot.TopUsers) == 0 || snapshot.TopUsers[0].UserKey != "u1" {
		t.Fatalf("top users = %+v, want u1 first", snapshot.TopUsers)
	}
	if snapshot.RequestStats.Requests1Hour != 2 {
		t.Fatalf("requests 1h = %d, want 2", snapshot.RequestStats.Requests1Hour)
	}
	if snapshot.RequestStats.Errors1Hour != 1 {
		t.Fatalf("errors 1h = %d, want 1", snapshot.RequestStats.Errors1Hour)
	}
	if len(snapshot.QuestionStatusBreakdown) == 0 || snapshot.QuestionStatusBreakdown[0].Name != statusSuccess {
		t.Fatalf("question status breakdown = %+v, want success", snapshot.QuestionStatusBreakdown)
	}

	page, err := collector.QuestionsPage(QuestionQuery{Page: 1, PageSize: 2, UserKey: "u1"})
	if err != nil {
		t.Fatalf("questions page: %v", err)
	}
	if page.TotalItems != 2 {
		t.Fatalf("questions total items = %d, want 2", page.TotalItems)
	}
	if len(page.Items) != 2 {
		t.Fatalf("questions items = %d, want 2", len(page.Items))
	}
	if page.Items[0].Question != "explain index merge" {
		t.Fatalf("first question = %q, want newest", page.Items[0].Question)
	}

	users, err := collector.UsersPage(UserQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("users page: %v", err)
	}
	if users.TotalItems != 2 {
		t.Fatalf("users total items = %d, want 2", users.TotalItems)
	}
	if users.Items[0].UserKey != "u1" || users.Items[0].QuestionCount != 2 {
		t.Fatalf("first user = %+v, want u1 count 2", users.Items[0])
	}
}
